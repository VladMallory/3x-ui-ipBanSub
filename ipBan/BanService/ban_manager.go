package ipban

import (
	"encoding/json"
	"fmt"
	"ipBanSystem/ipBan/logger/initLogs"
	"os"
	"sync"
	"time"
)

// BanInfo содержит информацию о бане пользователя
type BanInfo struct {
	Email       string    `json:"email"`
	BannedAt    time.Time `json:"banned_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Reason      string    `json:"reason"`
	IPAddresses []string  `json:"ip_addresses"`
}

// BanManager управляет банами пользователей
// mutex добавлен для предотвращения гонок при одновременном доступе к карте Bans из разных горутин
type BanManager struct {
	BansFile string
	Bans     map[string]*BanInfo
	mutex    sync.RWMutex // Мьютекс для синхронизации доступа к карте Bans
}

// NewBanManager создает новый менеджер банов
func NewBanManager(bansFile string) *BanManager {
	bm := &BanManager{
		BansFile: bansFile,
		Bans:     make(map[string]*BanInfo),
	}
	bm.loadBans()
	return bm
}

// loadBans загружает баны из файла
func (bm *BanManager) loadBans() {
	data, err := os.ReadFile(bm.BansFile)
	if err != nil {
		// Файл не существует или пуст - это нормально
		return
	}

	if err := json.Unmarshal(data, &bm.Bans); err != nil {
		fmt.Printf("BAN_MANAGER: Ошибка загрузки банов: %v\n", err)
		bm.Bans = make(map[string]*BanInfo)
	}
}

// saveBans сохраняет баны в файл
func (bm *BanManager) saveBans() error {
	data, err := json.MarshalIndent(bm.Bans, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации банов: %v", err)
	}

	return os.WriteFile(bm.BansFile, data, 0o644)
}

// IsBanned проверяет, забанен ли пользователь
// Использует RWMutex для защиты от гонок при одновременном доступе к карте Bans
func (bm *BanManager) IsBanned(email string) bool {
	bm.mutex.RLock() // Блокировка на чтение
	ban, exists := bm.Bans[email]
	bm.mutex.RUnlock()
	
	if !exists {
		return false
	}

	// Проверяем, не истек ли бан
	if time.Now().After(ban.ExpiresAt) {
		// Бан истек, удаляем его
		// Логируем в bot.log: автоматическое разбанирование при проверке
		initLogs.LogIPBanAction("АВТО_РАЗБАНЕН_ПРИ_ПРОВЕРКЕ", email, len(ban.IPAddresses), ban.IPAddresses)
		initLogs.LogIPBanInfo("Пользователь %s автоматически разбанен при проверке (бан истек: %s)",
			email, ban.ExpiresAt.Format("2006-01-02 15:04:05"))

		// При удалении элемента из карты нужна блокировка на запись
		bm.mutex.Lock()
		delete(bm.Bans, email)
		bm.saveBans()
		bm.mutex.Unlock()
		return false
	}

	return true
}

// BanUser банит пользователя
// Использует mutex.Lock() для обеспечения атомарности операции добавления нового бана
func (bm *BanManager) BanUser(email string, reason string, ipAddresses []string) error {
	banDuration := time.Duration(IP_BAN_DURATION) * time.Minute
	if IP_BAN_DURATION <= 0 {
		banDuration = 0 // Бесконечный бан
	}

	now := time.Now()
	ban := &BanInfo{
		Email:       email,
		BannedAt:    now,
		ExpiresAt:   now.Add(banDuration),
		Reason:      reason,
		IPAddresses: ipAddresses,
	}

	// Блокировка на запись при модификации карты Bans
	bm.mutex.Lock()
	bm.Bans[email] = ban
	err := bm.saveBans()
	bm.mutex.Unlock()

	// Логируем в bot.log: банирование пользователя
	initLogs.LogIPBanAction("ЗАБАНЕН", email, len(ipAddresses), ipAddresses)
	initLogs.LogIPBanInfo("Причина бана: %s", reason)
	if banDuration > 0 {
		initLogs.LogIPBanInfo("Бан до: %s", ban.ExpiresAt.Format("2006-01-02 15:04:05"))
	} else {
		initLogs.LogIPBanInfo("Бан бессрочный")
	}

	// Логируем информацию о забаненном пользователе, если это включено в конфиге
	if LOG_BANNED_USERS {
		initLogs.LogBannedUser(ban.Email, ban.IPAddresses, ban.Reason, ban.ExpiresAt)
	}

	return err
}

// UnbanUser разбанивает пользователя
// Использует синхронизацию для безопасного удаления элемента из общей карты
func (bm *BanManager) UnbanUser(email string) error {
	// Получаем информацию о бане перед удалением
	// Используем RLock при чтении из карты
	bm.mutex.RLock()
	banInfo, exists := bm.Bans[email]
	bm.mutex.RUnlock()
	
	if exists {
		// Логируем в bot.log: разбанирование пользователя с деталями
		initLogs.LogIPBanAction("РАЗБАНЕН", email, len(banInfo.IPAddresses), banInfo.IPAddresses)
		initLogs.LogIPBanInfo("Пользователь %s разбанен (был забанен: %s, причина: %s)",
			email,
			banInfo.BannedAt.Format("2006-01-02 15:04:05"),
			banInfo.Reason)
	} else {
		initLogs.LogIPBanWarning("Попытка разбанить пользователя %s, который не был забанен", email)
		// Логируем в bot.log: попытка разбанить несуществующего пользователя
	}

	// Блокировка на запись при удалении из карты Bans
	bm.mutex.Lock()
	delete(bm.Bans, email)
	err := bm.saveBans()
	bm.mutex.Unlock()
	return err
}

// GetBanInfo возвращает информацию о бане пользователя
func (bm *BanManager) GetBanInfo(email string) *BanInfo {
	bm.mutex.RLock()
	ban, exists := bm.Bans[email]
	bm.mutex.RUnlock()
	
	if !exists {
		return nil
	}

	// Проверяем, не истек ли бан
	if time.Now().After(ban.ExpiresAt) {
		// Логируем в bot.log: автоматическое разбанирование при получении информации
		initLogs.LogIPBanAction("АВТО_РАЗБАНЕН_ПРИ_ЗАПРОСЕ", email, len(ban.IPAddresses), ban.IPAddresses)
		initLogs.LogIPBanInfo("Пользователь %s автоматически разбанен при запросе информации (бан истек: %s)",
			email, ban.ExpiresAt.Format("2006-01-02 15:04:05"))

		bm.mutex.Lock()
		delete(bm.Bans, email)
		bm.saveBans()
		bm.mutex.Unlock()
		return nil
	}

	return ban
}

// CleanupExpiredBans удаляет истекшие баны
func (bm *BanManager) CleanupExpiredBans() {
	now := time.Now()
	expiredCount := 0

	bm.mutex.Lock()
	for email, ban := range bm.Bans {
		if now.After(ban.ExpiresAt) {
			// Логируем в bot.log: автоматическое разбанирование по истечении срока
			initLogs.LogIPBanAction("АВТО_РАЗБАНЕН", email, len(ban.IPAddresses), ban.IPAddresses)
			initLogs.LogIPBanInfo("Пользователь %s автоматически разбанен (бан истек: %s, был забанен: %s)",
				email,
				ban.ExpiresAt.Format("2006-01-02 15:04:05"),
				ban.BannedAt.Format("2006-01-02 15:04:05"))

			delete(bm.Bans, email)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		bm.saveBans()
		fmt.Printf("BAN_MANAGER: Удалено %d истекших банов\n", expiredCount)
		// Логируем в bot.log: общая статистика очистки
		initLogs.LogIPBanInfo("Очистка истекших банов: удалено %d пользователей", expiredCount)
	}
	bm.mutex.Unlock()
}

// CleanupOldBans удаляет баны, которые истекли дольше заданного времени назад
func (bm *BanManager) CleanupOldBans(retentionMinutes int) {
	if retentionMinutes <= 0 {
		return // Если время хранения = 0, данные хранятся бесконечно
	}

	now := time.Now()
	cutoffTime := now.Add(-time.Duration(retentionMinutes) * time.Minute)
	oldBansCount := 0

	fmt.Printf("BAN_MANAGER: Очистка старых банов: удаляются баны, истекшие дольше %d минут назад\n", retentionMinutes)

	bm.mutex.Lock()
	for email, ban := range bm.Bans {
		// Удаляем баны, которые истекли дольше retentionMinutes назад
		if ban.ExpiresAt.Before(cutoffTime) {
			// Логируем в bot.log: удаление старого бана
			initLogs.LogIPBanInfo("Удаление старого бана для %s (истёк: %s, был забанен: %s)",
				email,
				ban.ExpiresAt.Format("2006-01-02 15:04:05"),
				ban.BannedAt.Format("2006-01-02 15:04:05"))

			delete(bm.Bans, email)
			oldBansCount++
			fmt.Printf("BAN_MANAGER: Удален старый бан для %s (истёк: %s)\n",
				email, ban.ExpiresAt.Format("15:04:05 02.01.2006"))
		}
	}

	if oldBansCount > 0 {
		bm.saveBans()
		fmt.Printf("BAN_MANAGER: Удалено %d старых банов из файла\n", oldBansCount)
		// Логируем в bot.log: общая статистика очистки старых банов
		initLogs.LogIPBanInfo("Очистка старых банов: удалено %d пользователей (старше %d минут)", oldBansCount, retentionMinutes)
	}
	bm.mutex.Unlock()
}

// GetActiveBans возвращает список активных банов
func (bm *BanManager) GetActiveBans() map[string]*BanInfo {
	bm.CleanupExpiredBans() // Очищаем истекшие баны
	
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	
	// Создаем копию карты, чтобы избежать race condition при возврате
	activeBans := make(map[string]*BanInfo)
	for k, v := range bm.Bans {
		activeBans[k] = v
	}
	return activeBans
}

// GetBanStats возвращает статистику банов
func (bm *BanManager) GetBanStats() map[string]interface{} {
	bm.CleanupExpiredBans()

	bm.mutex.RLock()
	totalBans := len(bm.Bans)
	expiredSoon := 0
	now := time.Now()

	for _, ban := range bm.Bans {
		if ban.ExpiresAt.Sub(now) < time.Hour {
			expiredSoon++
		}
	}
	bm.mutex.RUnlock()

	return map[string]interface{}{
		"total_bans":     totalBans,
		"expired_soon":   expiredSoon,
		"ban_duration":   IP_BAN_DURATION,
		"unlimited_bans": IP_BAN_DURATION <= 0,
	}
}

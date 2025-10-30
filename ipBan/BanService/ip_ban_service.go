package ipban

import (
	"fmt"
	"ipBanSystem/ipBan/logger/analyzerLogs"
	"ipBanSystem/ipBan/logger/initLogs"
	"ipBanSystem/ipBan/panel"
	"ipBanSystem/ipBan/panel/client"
	"strings"
	"time"
)

// IPBanService основной сервис для управления IP банами
type IPBanService struct {
	Analyzer      *analyzerLogs.LogAnalyzer
	ConfigManager *panel.ConfigManager
	BanManager    *BanManager
	IPTables      *IPTablesManager
	MaxIPs        int
	CheckInterval time.Duration
	GracePeriod   time.Duration
	Running       bool
	StopChan      chan bool
}

// NewIPBanService создает новый сервис IP бана
func NewIPBanService(analyzer *analyzerLogs.LogAnalyzer, configManager *panel.ConfigManager, banManager *BanManager, iptables *IPTablesManager, maxIPs int, checkInterval, gracePeriod time.Duration) *IPBanService {
	return &IPBanService{
		Analyzer:      analyzer,
		ConfigManager: configManager,
		BanManager:    banManager,
		IPTables:      iptables,
		MaxIPs:        maxIPs,
		CheckInterval: checkInterval,
		GracePeriod:   gracePeriod,
		Running:       false,
		StopChan:      make(chan bool, 1),
	}
}

// Start запускает сервис мониторинга
func (s *IPBanService) Start() error {
	if s.Running {
		return fmt.Errorf("сервис уже запущен")
	}

	s.Running = true
	fmt.Printf("🚀 Запуск IP Ban сервиса...\n")
	fmt.Printf("📊 Максимум IP на конфиг: %d\n", s.MaxIPs)
	fmt.Printf("⏰ Интервал проверки: %v\n", s.CheckInterval)
	fmt.Printf("⏳ Период ожидания: %v\n", s.GracePeriod)
	fmt.Println(strings.Repeat("=", 50))

	go s.monitorLoop()
	return nil
}

// Stop останавливает сервис мониторинга
func (s *IPBanService) Stop() {
	if !s.Running {
		return
	}

	fmt.Println("🛑 Остановка IP Ban сервиса...")
	s.Running = false
	s.StopChan <- true
}

// monitorLoop основной цикл мониторинга
func (s *IPBanService) monitorLoop() {
	ticker := time.NewTicker(s.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.performCheck()
		case <-s.StopChan:
			fmt.Println("✅ IP Ban сервис остановлен")
			return
		}
	}
}

// performCheck выполняет проверку и управление конфигами
func (s *IPBanService) performCheck() {
	initLogs.LogIPBanInfo("Начало проверки...")

	// Получаем все конфиги из панели
	allConfigs, err := client.All(s.ConfigManager)
	if err != nil {
		initLogs.LogIPBanError("Ошибка получения конфигов из панели: %v", err)
		return
	}

	if len(allConfigs) == 0 {
		initLogs.LogIPBanInfo("Нет конфигов для анализа")
		return
	}

	// Анализируем лог файл для получения статистики IP
	logStats, err := s.Analyzer.AnalyzeLog()
	if err != nil {
		initLogs.LogIPBanError("Ошибка анализа лога: %v", err)
		return
	}

	// Создаем карту статистики IP по email
	ipStatsMap := make(map[string]*analyzerLogs.EmailIPStats)
	for _, stats := range logStats {
		ipStatsMap[stats.Email] = stats
	}

	// Очищаем истекшие баны
	s.BanManager.CleanupExpiredBans()

	// Очищаем старые баны (которые истекли дольше IP_COUNTER_RETENTION назад)
	s.BanManager.CleanupOldBans(IP_COUNTER_RETENTION)

	// Обрабатываем каждый конфиг из панели
	suspiciousCount := 0
	normalCount := 0
	enabledCount := 0
	bannedCount := 0

	for _, config := range allConfigs {
		// Проверяем, не забанен ли пользователь
		if s.BanManager.IsBanned(config.Email) {
			banInfo := s.BanManager.GetBanInfo(config.Email)
			initLogs.LogIPBanInfo("Забаненный конфиг: %s (бан до: %s)", config.Email, banInfo.ExpiresAt.Format("15:04:05 02.01.2006"))
			bannedCount++

			// ВАЖНО: Если забаненный конфиг включен в панели — применяем АГРЕССИВНЫЙ сброс
			if config.Enable {
				initLogs.LogIPBanInfo("Забаненный конфиг %s включен — выполняем агрессивный сброс", config.Email)
				if _, err := client.AggressiveBanReset(s.ConfigManager, config.Email); err != nil {
					initLogs.LogIPBanError("Ошибка AggressiveBanReset для %s: %v", config.Email, err)
				} else {
					initLogs.LogIPBanInfo("Забаненный конфиг %s агрессивно сброшен (enable=false, depleted/exhausted=true, UUID обновлён)", config.Email)
				}
			} else {
				initLogs.LogIPBanInfo("Забаненный конфиг %s уже отключен в панели", config.Email)
			}
			continue
		}

		// Получаем статистику IP для этого конфига
		ipStats, hasActivity := ipStatsMap[config.Email]

		if hasActivity {
			// Конфиг имеет активность в логах
			if ipStats.TotalIPs > s.MaxIPs {
				// Подозрительный конфиг - баним
				suspiciousCount++
				s.handleSuspiciousConfig(ipStats)
			} else {
				// Нормальный конфиг - включаем
				normalCount++
				s.handleNormalConfig(ipStats)
			}
		} else {
			// Конфиг не имеет активности в логах
			if !config.Enable {
				// Отключенный конфиг без активности - включаем
				initLogs.LogIPBanInfo("Конфиг без активности: %s (отключен, включаем)", config.Email)
				if err := client.Enable(s.ConfigManager, config.Email); err != nil {
					initLogs.LogIPBanError("Ошибка включения конфига %s: %v", config.Email, err)
				} else {
					initLogs.LogIPBanInfo("Конфиг %s успешно включен", config.Email)
					enabledCount++
				}
			} else {
				// Включенный конфиг без активности - оставляем как есть, логировать не нужно
			}
		}
	}

	initLogs.LogIPBanInfo("Подозрительных конфигов: %d", suspiciousCount)
	initLogs.LogIPBanInfo("Нормальных конфигов: %d", normalCount)
	initLogs.LogIPBanInfo("Включено отключенных: %d", enabledCount)
	initLogs.LogIPBanInfo("Забаненных конфигов: %d", bannedCount)
	initLogs.LogIPBanInfo("Проверка завершена")
}

// handleSuspiciousConfig обрабатывает подозрительный конфиг
func (s *IPBanService) handleSuspiciousConfig(stats *analyzerLogs.EmailIPStats) {
	initLogs.LogIPBanInfo("Подозрительный конфиг: %s (IP адресов: %d, максимум: %d)",
		stats.Email, stats.TotalIPs, s.MaxIPs)

	// Собираем список IP адресов для уведомления
	var ipAddresses []string
	for ip, activity := range stats.IPs {
		initLogs.LogIPBanInfo("   📍 %s (соединений: %d, последний раз: %s)",
			ip, activity.Count, activity.LastSeen.Format("15:04:05"))
		ipAddresses = append(ipAddresses, ip)
	}

	// Проверяем, не забанен ли уже пользователь
	if s.BanManager.IsBanned(stats.Email) {
		banInfo := s.BanManager.GetBanInfo(stats.Email)
		initLogs.LogIPBanInfo("   ℹ️  Пользователь %s уже забанен до %s, пропускаем повторный бан",
			stats.Email, banInfo.ExpiresAt.Format("15:04:05 02.01.2006"))
		return
	}

	// Баним пользователя
	reason := fmt.Sprintf("Превышение лимита IP адресов: %d (максимум: %d)", stats.TotalIPs, s.MaxIPs)
	initLogs.LogIPBanInfo("Начало банирования пользователя %s (IP адресов: %d, лимит: %d)", stats.Email, stats.TotalIPs, s.MaxIPs)

	if err := s.BanManager.BanUser(stats.Email, reason, ipAddresses); err != nil {
		initLogs.LogIPBanError("Ошибка бана пользователя %s: %v", stats.Email, err)
		return
	}

	initLogs.LogIPBanInfo("   🚫 Пользователь %s забанен на %d минут", stats.Email, IP_BAN_DURATION)

	// Агрессивный сброс: отключение, выставление depleted/exhausted, смена email(-reset) и UUID, двойной апдейт + ресет Remark
	initLogs.LogIPBanInfo("   🔒 Агрессивный сброс для %s...", stats.Email)
	if _, err := client.AggressiveBanReset(s.ConfigManager, stats.Email); err != nil {
		initLogs.LogIPBanError("❌ Ошибка AggressiveBanReset для %s: %v", stats.Email, err)
	} else {
		initLogs.LogIPBanInfo("   ✅ Агрессивный сброс применён для %s", stats.Email)
	}
}

// handleNormalConfig обрабатывает нормальный конфиг
func (s *IPBanService) handleNormalConfig(stats *analyzerLogs.EmailIPStats) {
	// Логируем информацию о нормальном конфиге
	initLogs.LogIPBanInfo("%s (IP адресов: %d, максимум: %d)", stats.Email, stats.TotalIPs, s.MaxIPs)

	// Проверяем, не забанен ли пользователь
	if s.BanManager.IsBanned(stats.Email) {
		// Если пользователь забанен, но активность нормализовалась, разблокируем IP
		initLogs.LogIPBanInfo("   🔓 Разблокировка IP адресов для %s...", stats.Email)

		unblockedCount := 0
		for ip := range stats.IPs {
			if err := s.IPTables.UnblockIP(ip); err != nil {
				initLogs.LogIPBanError("Ошибка разблокировки IP %s: %v", ip, err)
			} else {
				unblockedCount++
			}
		}

		if unblockedCount > 0 {
			initLogs.LogIPBanInfo("   ✅ Разблокировано %d IP адресов через iptables", unblockedCount)
		}
	} else {
		// ВАЖНО: Проверяем статус конфига в панели - если он отключен, включаем его
		currentStatus, err := client.Status(s.ConfigManager, stats.Email)
		if err != nil {
			initLogs.LogIPBanError("Ошибка получения статуса нормального конфига %s: %v", stats.Email, err)
		} else if !currentStatus {
			// Конфиг отключен в панели, но активность нормальная - включаем его
			initLogs.LogIPBanInfo("   🔓 Нормальный конфиг %s отключен в панели - включаем!", stats.Email)
			if err := client.Enable(s.ConfigManager, stats.Email); err != nil {
				initLogs.LogIPBanError("Ошибка включения нормального конфига %s: %v", stats.Email, err)
			} else {
				initLogs.LogIPBanInfo("   ✅ Нормальный конфиг %s успешно включен в панели", stats.Email)
			}
		}
		// Если конфиг уже включен и работает нормально, дополнительное логирование не требуется,
		// так как основная информация уже была залогирована в начале функции.
	}
}

// GetStatus возвращает текущий статус сервиса
func (s *IPBanService) GetStatus() map[string]interface{} {
	stats, err := s.Analyzer.AnalyzeLog()
	if err != nil {
		return map[string]interface{}{
			"running": s.Running,
			"error":   err.Error(),
		}
	}

	suspiciousCount := len(s.Analyzer.GetSuspiciousEmails(s.MaxIPs))
	normalCount := len(s.Analyzer.GetNormalEmails(s.MaxIPs))

	return map[string]interface{}{
		"running":            s.Running,
		"total_emails":       len(stats),
		"suspicious_count":   suspiciousCount,
		"normal_count":       normalCount,
		"max_ips_per_config": s.MaxIPs,
		"check_interval":     s.CheckInterval.String(),
		"grace_period":       s.GracePeriod.String(),
	}
}

// PrintCurrentStats выводит текущую статистику
func (s *IPBanService) PrintCurrentStats() {
	fmt.Println("\n📊 Текущая статистика:")

	stats, err := s.Analyzer.AnalyzeLog()
	if err != nil {
		fmt.Printf("❌ Ошибка получения статистики: %v\n", err)
		return
	}

	if len(stats) == 0 {
		fmt.Println("📝 Нет данных для отображения")
		return
	}

	suspiciousEmails := s.Analyzer.GetSuspiciousEmails(s.MaxIPs)
	normalEmails := s.Analyzer.GetNormalEmails(s.MaxIPs)

	fmt.Printf("📈 Всего email: %d\n", len(stats))
	fmt.Printf("🚨 Подозрительных: %d\n", len(suspiciousEmails))
	fmt.Printf("✅ Нормальных: %d\n", len(normalEmails))

	fmt.Println("\n📋 Детальная статистика:")
	for email, emailStats := range stats {
		status := "✅ Нормальный"
		if emailStats.TotalIPs > s.MaxIPs {
			status = "🚨 Подозрительный"
		}

		fmt.Printf("  %s %s: %d IP\n", status, email, emailStats.TotalIPs)
	}
}

// IPTablesManager управляет блокировкой IP через iptables
// getIPAddressesFromStats извлекает IP адреса из статистики для логирования
// func getIPAddressesFromStats(stats *analyzerLogs.EmailIPStats) []string {
// 	var ips []string
// 	for ip := range stats.IPs {
// 		ips = append(ips, ip)
// 	}
// 	return ips
// }

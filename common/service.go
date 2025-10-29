package common

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// IPBanService основной сервис для мониторинга и управления IP банами
type IPBanService struct {
	Analyzer      *LogAnalyzer
	ConfigManager *ConfigManager
	BanManager    *BanManager
	IPTables      *IPTablesManager // Менеджер для работы с iptables
	MaxIPs        int
	CheckInterval time.Duration
	GracePeriod   time.Duration
	Running       bool
	StopChan      chan bool
}

// NewIPBanService создает новый сервис IP бана
func NewIPBanService(analyzer *LogAnalyzer, configManager *ConfigManager, banManager *BanManager, iptables *IPTablesManager, maxIPs int, checkInterval, gracePeriod time.Duration) *IPBanService {
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
	LogIPBanInfo("Начало проверки...")

	// Получаем все конфиги из панели
	allConfigs, err := s.ConfigManager.GetConfigs()
	if err != nil {
		LogIPBanError("Ошибка получения конфигов из панели: %v", err)
		return
	}

	if len(allConfigs) == 0 {
		LogIPBanInfo("Нет конфигов для анализа")
		return
	}

	// Анализируем лог файл для получения статистики IP
	logStats, err := s.Analyzer.AnalyzeLog()
	if err != nil {
		LogIPBanError("Ошибка анализа лога: %v", err)
		return
	}

	// Создаем карту статистики IP по email
	ipStatsMap := make(map[string]*EmailIPStats)
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
			LogIPBanInfo("Забаненный конфиг: %s (бан до: %s)", config.Email, banInfo.ExpiresAt.Format("15:04:05 02.01.2006"))
			bannedCount++

			// ВАЖНО: Проверяем, включен ли забаненный конфиг в панели, и отключаем его
			if config.Enable {
				LogIPBanInfo("Забаненный конфиг %s включен в панели - отключаем!", config.Email)
				if err := s.ConfigManager.DisableConfig(config.Email); err != nil {
					LogIPBanError("Ошибка отключения забаненного конфига %s: %v", config.Email, err)
				} else {
					LogIPBanInfo("Забаненный конфиг %s успешно отключен в панели", config.Email)
				}
			} else {
				LogIPBanInfo("Забаненный конфиг %s уже отключен в панели", config.Email)
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
				LogIPBanInfo("Конфиг без активности: %s (отключен, включаем)", config.Email)
				if err := s.ConfigManager.EnableConfig(config.Email); err != nil {
					LogIPBanError("Ошибка включения конфига %s: %v", config.Email, err)
				} else {
					LogIPBanInfo("Конфиг %s успешно включен", config.Email)
					enabledCount++
				}
			} else {
				// Включенный конфиг без активности - оставляем как есть, логировать не нужно
			}
		}
	}

	LogIPBanInfo("Подозрительных конфигов: %d", suspiciousCount)
	LogIPBanInfo("Нормальных конфигов: %d", normalCount)
	LogIPBanInfo("Включено отключенных: %d", enabledCount)
	LogIPBanInfo("Забаненных конфигов: %d", bannedCount)
	LogIPBanInfo("Проверка завершена")
}

// handleSuspiciousConfig обрабатывает подозрительный конфиг
func (s *IPBanService) handleSuspiciousConfig(stats *EmailIPStats) {
	LogIPBanInfo("Подозрительный конфиг: %s (IP адресов: %d, максимум: %d)",
		stats.Email, stats.TotalIPs, s.MaxIPs)

	// Собираем список IP адресов для уведомления
	var ipAddresses []string
	for ip, activity := range stats.IPs {
		LogIPBanInfo("   📍 %s (соединений: %d, последний раз: %s)",
			ip, activity.Count, activity.LastSeen.Format("15:04:05"))
		ipAddresses = append(ipAddresses, ip)
	}

	// Проверяем, не забанен ли уже пользователь
	if s.BanManager.IsBanned(stats.Email) {
		banInfo := s.BanManager.GetBanInfo(stats.Email)
		LogIPBanInfo("   ℹ️  Пользователь %s уже забанен до %s, пропускаем повторный бан",
			stats.Email, banInfo.ExpiresAt.Format("15:04:05 02.01.2006"))
		return
	}

	// Баним пользователя
	reason := fmt.Sprintf("Превышение лимита IP адресов: %d (максимум: %d)", stats.TotalIPs, s.MaxIPs)
	LogIPBanInfo("Начало банирования пользователя %s (IP адресов: %d, лимит: %d)", stats.Email, stats.TotalIPs, s.MaxIPs)

	if err := s.BanManager.BanUser(stats.Email, reason, ipAddresses); err != nil {
		LogIPBanError("Ошибка бана пользователя %s: %v", stats.Email, err)
		return
	}

	LogIPBanInfo("   🚫 Пользователь %s забанен на %d минут", stats.Email, IP_BAN_DURATION)

	// Мгновенно отключаем конфиг и ротируем UUID, чтобы обрубить активные сессии без рестарта Xray
	LogIPBanInfo("   🔒 Отключение и ротация UUID для %s...", stats.Email)
	if _, err := s.ConfigManager.DisableAndRotateConfig(stats.Email); err != nil {
		LogIPBanError("❌ Ошибка DisableAndRotateConfig для %s: %v", stats.Email, err)
	} else {
		LogIPBanInfo("   ✅ Конфиг %s отключён, UUID обновлён", stats.Email)
	}
}

// handleNormalConfig обрабатывает нормальный конфиг
func (s *IPBanService) handleNormalConfig(stats *EmailIPStats) {
	// Логируем информацию о нормальном конфиге
	LogIPBanInfo("%s (IP адресов: %d, максимум: %d)", stats.Email, stats.TotalIPs, s.MaxIPs)

	// Проверяем, не забанен ли пользователь
	if s.BanManager.IsBanned(stats.Email) {
		// Если пользователь забанен, но активность нормализовалась, разблокируем IP
		LogIPBanInfo("   🔓 Разблокировка IP адресов для %s...", stats.Email)

		unblockedCount := 0
		for ip := range stats.IPs {
			if err := s.IPTables.UnblockIP(ip); err != nil {
				LogIPBanError("Ошибка разблокировки IP %s: %v", ip, err)
			} else {
				unblockedCount++
			}
		}

		if unblockedCount > 0 {
			LogIPBanInfo("   ✅ Разблокировано %d IP адресов через iptables", unblockedCount)
		}
	} else {
		// ВАЖНО: Проверяем статус конфига в панели - если он отключен, включаем его
		currentStatus, err := s.ConfigManager.GetConfigStatus(stats.Email)
		if err != nil {
			LogIPBanError("Ошибка получения статуса нормального конфига %s: %v", stats.Email, err)
		} else if !currentStatus {
			// Конфиг отключен в панели, но активность нормальная - включаем его
			LogIPBanInfo("   🔓 Нормальный конфиг %s отключен в панели - включаем!", stats.Email)
			if err := s.ConfigManager.EnableConfig(stats.Email); err != nil {
				LogIPBanError("Ошибка включения нормального конфига %s: %v", stats.Email, err)
			} else {
				LogIPBanInfo("   ✅ Нормальный конфиг %s успешно включен в панели", stats.Email)
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
type IPTablesManager struct {
	BlockedIPs map[string]bool // Карта заблокированных IP
}

// NewIPTablesManager создает новый менеджер iptables
func NewIPTablesManager() *IPTablesManager {
	return &IPTablesManager{
		BlockedIPs: make(map[string]bool),
	}
}

// BlockIP блокирует IP адрес через iptables
func (i *IPTablesManager) BlockIP(ipAddress string) error {
	// Проверяем, не заблокирован ли уже IP
	if i.BlockedIPs[ipAddress] {
		fmt.Printf("ℹ️  IP %s уже заблокирован\n", ipAddress)
		// Логируем в bot.log: IP уже заблокирован
		LogIPBanInfo("IP %s уже заблокирован через iptables", ipAddress)
		return nil
	}

	// Логируем в bot.log: начало блокировки IP
	LogIPBanInfo("Блокировка IP %s через iptables", ipAddress)

	// Блокируем IP через iptables
	cmd := fmt.Sprintf("iptables -I INPUT -s %s -j DROP", ipAddress)
	if err := i.executeCommand(cmd); err != nil {
		// Логируем в bot.log: ошибка блокировки IP
		LogIPBanError("Ошибка блокировки IP %s через iptables: %v", ipAddress, err)
		return fmt.Errorf("ошибка блокировки IP %s: %v", ipAddress, err)
	}

	// Добавляем IP в список заблокированных
	i.BlockedIPs[ipAddress] = true
	fmt.Printf("✅ IP %s успешно заблокирован через iptables\n", ipAddress)

	// Логируем в bot.log: успешная блокировка IP
	LogIPBanAction("IP_ЗАБЛОКИРОВАН", ipAddress, 0, []string{})

	return nil
}

// UnblockIP разблокирует IP адрес через iptables
func (i *IPTablesManager) UnblockIP(ipAddress string) error {
	// Проверяем, заблокирован ли IP
	if !i.BlockedIPs[ipAddress] {
		fmt.Printf("ℹ️  IP %s не был заблокирован\n", ipAddress)
		// Логируем в bot.log: IP не был заблокирован
		LogIPBanInfo("IP %s не был заблокирован через iptables", ipAddress)
		return nil
	}

	// Логируем в bot.log: начало разблокировки IP
	LogIPBanInfo("Разблокировка IP %s через iptables", ipAddress)

	// Разблокируем IP через iptables
	cmd := fmt.Sprintf("iptables -D INPUT -s %s -j DROP", ipAddress)
	if err := i.executeCommand(cmd); err != nil {
		// Логируем в bot.log: ошибка разблокировки IP
		LogIPBanError("Ошибка разблокировки IP %s через iptables: %v", ipAddress, err)
		return fmt.Errorf("ошибка разблокировки IP %s: %v", ipAddress, err)
	}

	// Удаляем IP из списка заблокированных
	delete(i.BlockedIPs, ipAddress)
	fmt.Printf("✅ IP %s успешно разблокирован через iptables\n", ipAddress)

	// Логируем в bot.log: успешная разблокировка IP
	LogIPBanAction("IP_РАЗБЛОКИРОВАН", ipAddress, 0, []string{})

	return nil
}

// executeCommand выполняет команду в системе
func (i *IPTablesManager) executeCommand(cmd string) error {
	// Используем os/exec для выполнения команды
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return fmt.Errorf("неверная команда: %s", cmd)
	}

	// Выполняем команду
	execCmd := exec.Command(parts[0], parts[1:]...)
	output, err := execCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ошибка выполнения команды '%s': %v, output: %s", cmd, err, string(output))
	}

	return nil
}

// getIPAddressesFromStats извлекает IP адреса из статистики для логирования
func getIPAddressesFromStats(stats *EmailIPStats) []string {
	var ips []string
	for ip := range stats.IPs {
		ips = append(ips, ip)
	}
	return ips
}

// GetBlockedIPs возвращает список заблокированных IP
func (i *IPTablesManager) GetBlockedIPs() []string {
	var ips []string
	for ip := range i.BlockedIPs {
		ips = append(ips, ip)
	}
	return ips
}

// IsIPBlocked проверяет, заблокирован ли IP
func (i *IPTablesManager) IsIPBlocked(ipAddress string) bool {
	return i.BlockedIPs[ipAddress]
}

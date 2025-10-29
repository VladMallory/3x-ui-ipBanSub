package app

import (
	ipban "ipBanSystem/ipBan/BanService"
	"ipBanSystem/ipBan/logger/accumulatorLogs"
	"ipBanSystem/ipBan/logger/analyzerLogs"
	"ipBanSystem/ipBan/logger/initLogs"
	"ipBanSystem/ipBan/panel"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Run запускает основной IP Ban сервис
func Run() {
	// Инициализируем логгеры
	if err := initLogs.InitIPBanLogger(ipban.IP_BAN_LOG_PATH); err != nil {
		log.Fatalf("Ошибка инициализации логгера: %v", err)
	}
	if err := initLogs.InitBannedUsersLogger(ipban.BANNED_USERS_LOG_PATH); err != nil {
		log.Fatalf("Ошибка инициализации логгера забаненных пользователей: %v", err)
	}

	initLogs.LogIPBanInfo("Запуск IP Ban сервиса...")

	// Создаем компоненты
	accumulator := accumulatorLogs.NewLogAccumulator(ipban.ACCESS_LOG_PATH, ipban.IP_ACCUMULATED_PATH)
	if err := accumulator.Start(); err != nil {
		initLogs.LogIPBanError("Ошибка запуска накопителя логов: %v", err)
		return
	}
	accumulator.StartCleanupService()
	initLogs.LogIPBanInfo("Накопитель логов запущен")

	analyzer := analyzerLogs.NewLogAnalyzer(ipban.IP_ACCUMULATED_PATH, ipban.IP_COUNTER_RETENTION, ipban.IP_ACCUMULATED_PATH)

	configManager := panel.NewConfigManager(
		panel.PANEL_URL,
		panel.PANEL_USER,
		panel.PANEL_PASS,
		panel.INBOUND_ID,
	)

	banManager := ipban.NewBanManager("/var/log/ip_bans.json")
	iptablesManager := ipban.NewIPTablesManager()

	// Создаем и запускаем сервис
	service := ipban.NewIPBanService(
		analyzer,
		configManager,
		banManager,
		iptablesManager,
		ipban.MAX_IPS_PER_CONFIG,
		time.Duration(ipban.IP_CHECK_INTERVAL)*time.Minute,
		time.Duration(ipban.IP_BAN_GRACE_PERIOD)*time.Minute,
	)

	if err := service.Start(); err != nil {
		initLogs.LogIPBanError("Ошибка запуска IP Ban сервиса: %v", err)
		return
	}

	// Ожидаем сигнала для завершения работы
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Останавливаем сервис
	service.Stop()
	initLogs.LogIPBanInfo("IP Ban сервис остановлен.")
}

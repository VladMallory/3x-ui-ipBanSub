package app

import (
	"ipBanSystem/common"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Run запускает основной IP Ban сервис
func Run() {
	// Инициализируем логгеры
	if err := common.InitIPBanLogger(); err != nil {
		log.Fatalf("Ошибка инициализации логгера: %v", err)
	}
	if err := common.InitBannedUsersLogger(); err != nil {
		log.Fatalf("Ошибка инициализации логгера забаненных пользователей: %v", err)
	}

	common.LogIPBanInfo("Запуск IP Ban сервиса...")

	// Создаем компоненты
	accumulator := common.NewLogAccumulator(common.ACCESS_LOG_PATH, common.IP_ACCUMULATED_PATH)
	if err := accumulator.Start(); err != nil {
		common.LogIPBanError("Ошибка запуска накопителя логов: %v", err)
		return
	}
	accumulator.StartCleanupService()
	common.LogIPBanInfo("Накопитель логов запущен")

	analyzer := common.NewLogAnalyzer(common.IP_ACCUMULATED_PATH)

	configManager := common.NewConfigManager(
		common.PANEL_URL,
		common.PANEL_USER,
		common.PANEL_PASS,
		common.INBOUND_ID,
	)

	banManager := common.NewBanManager("/var/log/ip_bans.json")
	iptablesManager := common.NewIPTablesManager()

	// Создаем и запускаем сервис
	service := common.NewIPBanService(
		analyzer,
		configManager,
		banManager,
		iptablesManager,
		common.MAX_IPS_PER_CONFIG,
		time.Duration(common.IP_CHECK_INTERVAL)*time.Minute,
		time.Duration(common.IP_BAN_GRACE_PERIOD)*time.Minute,
	)

	if err := service.Start(); err != nil {
		common.LogIPBanError("Ошибка запуска IP Ban сервиса: %v", err)
		return
	}

	// Ожидаем сигнала для завершения работы
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Останавливаем сервис
	service.Stop()
	common.LogIPBanInfo("IP Ban сервис остановлен.")
}

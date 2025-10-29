package flags

import (
	"flag"
	"ipBanSystem/installer"
)

type FlagsConfig struct {
	InstallFlag   bool
	UninstallFlag bool
	ReinstallFlag bool
}

// Flags создает флаги для запуска программы
func Flags() *FlagsConfig {
	// указываем флаги которые будут
	installFlag := flag.Bool("install", false, "Установить и запустить сервис")
	uninstallFlag := flag.Bool("uninstall", false, "Остановить и удалить сервис")
	reinstallFlag := flag.Bool("reinstall", false, "Переустановить сервис")
	flag.Parse()

	// возвращаем их через структуру, чтобы в main.go
	// дальше могли им пользоваться
	return &FlagsConfig{
		InstallFlag:   *installFlag,
		UninstallFlag: *uninstallFlag,
		ReinstallFlag: *reinstallFlag,
	}
}

// HandleServiceFlags выполняет команду по флагам установки сервиса.
// Возвращает true, если команда обработана и программу надо завершить.
func HandleServiceFlags(cfg *FlagsConfig) bool {
	if cfg.InstallFlag {
		installer.InstallService()
		return true
	}
	if cfg.UninstallFlag {
		installer.UninstallService()
		return true
	}
	if cfg.ReinstallFlag {
		installer.UninstallService()
		installer.InstallService()
		return true
	}
	return false
}

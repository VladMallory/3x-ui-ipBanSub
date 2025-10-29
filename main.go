package main

import (
	"ipBanSystem/app"
	"ipBanSystem/flags"
)

func main() {
	cfg := flags.Flags()

	// единый диспетчер служебных флагов (install/uninstall/reinstall)
	if flags.HandleServiceFlags(cfg) {
		return
	}

	// Запуск основного приложения
	app.Run()
}

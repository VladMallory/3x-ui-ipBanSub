package installer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	serviceName        = "ipBanService"
	serviceDescription = "IP Ban System Service"
	serviceFilePath    = "/etc/systemd/system/ipBanService.service"
	binaryPath         = "/usr/local/bin/ipBanService"
)

// InstallService устанавливает и запускает systemd-сервис ipBanService
func InstallService() {
	fmt.Println("Установка сервиса ipBanService...")

	// Шаг 1: Сборка бинарного файла
	fmt.Println("Шаг 1: Сборка бинарного файла...")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if err := runCommand(cmd); err != nil {
		log.Fatalf("Ошибка сборки бинарного файла: %v", err)
	}
	fmt.Println("Бинарный файл успешно собран в", binaryPath)

	// Шаг 2: Создание файла сервиса systemd
	fmt.Println("Шаг 2: Создание файла сервиса systemd...")
	// Исправление: обработка ошибки, которая раньше игнорировалась (err был присвоен _)
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Ошибка получения текущей директории: %v", err)
	}
	serviceFileContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
Type=simple
User=root
ExecStart=%s
WorkingDirectory=%s
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`,
		serviceDescription, binaryPath, workingDir)

	if err := os.WriteFile(serviceFilePath, []byte(serviceFileContent), 0644); err != nil {
		log.Fatalf("Ошибка создания файла сервиса: %v", err)
	}
	fmt.Println("Файл сервиса успешно создан в", serviceFilePath)

	// Шаг 3: Перезагрузка, включение и запуск сервиса
	fmt.Println("Шаг 3: Перезагрузка, включение и запуск сервиса...")
	if err := runCommand(exec.Command("systemctl", "daemon-reload")); err != nil {
		log.Fatalf("Ошибка перезагрузки демона systemd: %v", err)
	}
	if err := runCommand(exec.Command("systemctl", "enable", serviceName)); err != nil {
		log.Fatalf("Ошибка включения сервиса: %v", err)
	}
	if err := runCommand(exec.Command("systemctl", "start", serviceName)); err != nil {
		log.Fatalf("Ошибка запуска сервиса: %v", err)
	}

	fmt.Println("\n✅ Сервис ipBanService успешно установлен и запущен!")
	fmt.Println("Для проверки статуса используйте: systemctl status", serviceName)
}

// UninstallService останавливает и удаляет systemd-сервис ipBanService
func UninstallService() {
	fmt.Println("Удаление сервиса ipBanService...")

	// Шаг 1: Остановка и отключение сервиса
	fmt.Println("Шаг 1: Остановка и отключение сервиса...")
	runCommand(exec.Command("systemctl", "stop", serviceName))    // Игнорируем ошибку, если сервис не запущен
	runCommand(exec.Command("systemctl", "disable", serviceName)) // Игнорируем ошибку, если сервис не включен

	// Шаг 2: Удаление файла сервиса
	fmt.Println("Шаг 2: Удаление файла сервиса...")
	if err := os.Remove(serviceFilePath); err != nil {
		log.Printf("Ошибка удаления файла сервиса (возможно, он уже удален): %v", err)
	} else {
		fmt.Println("Файл сервиса удален.")
	}

	// Шаг 3: Перезагрузка демона systemd
	fmt.Println("Шаг 3: Перезагрузка демона systemd...")
	if err := runCommand(exec.Command("systemctl", "daemon-reload")); err != nil {
		log.Printf("Ошибка перезагрузки демона systemd: %v", err)
	}

	// Шаг 4: Удаление бинарного файла
	fmt.Println("Шаг 4: Удаление бинарного файла...")
	if err := os.Remove(binaryPath); err != nil {
		log.Printf("Ошибка удаления бинарного файла (возможно, он уже удален): %v", err)
	} else {
		fmt.Println("Бинарный файл удален.")
	}

	fmt.Println("\n✅ Сервис ipBanService успешно удален.")
}

// runCommand запускает команду и возвращает ошибку с выводом при неуспехе
func runCommand(cmd *exec.Cmd) error {
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("команда '%s' завершилась с ошибкой: %v\nВывод: %s", cmd.String(), err, out.String())
	}
	return nil
}

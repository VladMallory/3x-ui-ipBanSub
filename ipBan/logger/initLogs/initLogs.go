package initLogs

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	IPBanLogger       *log.Logger
	BannedUsersLogger *log.Logger
)

// InitIPBanLogger инициализирует логгер для IP ban в отдельный файл
func InitIPBanLogger(logPath string) error {
	// Создаем директорию для логов, если она не существует
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	// Открываем файл для записи логов IP ban
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}

	// Создаем логгер, который пишет и в файл, и в stdout
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	IPBanLogger = log.New(multiWriter, "", 0)

	return nil
}

// InitBannedUsersLogger инициализирует логгер для забаненных пользователей
func InitBannedUsersLogger(logPath string) error {
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}

	BannedUsersLogger = log.New(logFile, "", log.LstdFlags)
	return nil
}

// LogIPBanInfo логирует информационное сообщение о IP ban
func LogIPBanInfo(format string, v ...interface{}) {
	if IPBanLogger != nil {
		IPBanLogger.Printf("[INFO] "+format, v...)
	} else {
		fmt.Printf("[INFO]: "+format, v...)
	}
}

// LogIPBanWarning логирует предупреждение о IP ban
func LogIPBanWarning(format string, v ...interface{}) {
	if IPBanLogger != nil {
		IPBanLogger.Printf("[WARNING] "+format, v...)
	} else {
		log.Printf("IP_BAN [WARNING]: "+format, v...)
	}
}

// LogIPBanError логирует ошибку IP ban
func LogIPBanError(format string, v ...interface{}) {
	if IPBanLogger != nil {
		IPBanLogger.Printf("[ERROR] "+format, v...)
	} else {
		log.Printf("IP_BAN [ERROR]: "+format, v...)
	}
}

// LogIPBanAction логирует действие IP ban (включение/отключение конфига)
func LogIPBanAction(action, email string, ipCount int, ips []string) {
	if IPBanLogger != nil {
		IPBanLogger.Printf("[ACTION] %s конфиг %s (IP адресов: %d, IP: %v)", action, email, ipCount, ips)
	} else {
		log.Printf("IP_BAN [ACTION]: %s конфиг %s (IP адресов: %d, IP: %v)", action, email, ipCount, ips)
	}
}

// LogIPBanStats логирует статистику IP ban
func LogIPBanStats(totalEmails, suspiciousCount, normalCount int) {
	if IPBanLogger != nil {
		IPBanLogger.Printf("[STATS] Всего email: %d, Подозрительных: %d, Нормальных: %d", totalEmails, suspiciousCount, normalCount)
	} else {
		log.Printf("IP_BAN [STATS]: Всего email: %d, Подозрительных: %d, Нормальных: %d", totalEmails, suspiciousCount, normalCount)
	}
}

// LogBannedUser логирует информацию о забаненном пользователе
func LogBannedUser(email string, ipAddresses []string, reason string, expiresAt time.Time) {
	if BannedUsersLogger != nil {
		BannedUsersLogger.Printf("Пользователь: %s, IP адреса: %v, Причина: %s, Забанен до: %s",
			email, ipAddresses, reason, expiresAt.Format("2006-01-02 15:04:05"))
	} else {
		log.Printf("IP_BAN [BANNED]: Пользователь: %s, IP адреса: %v, Причина: %s, Забанен до: %s",
			email, ipAddresses, reason, expiresAt.Format("2006-01-02 15:04:05"))
	}
}

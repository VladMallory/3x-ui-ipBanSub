package panel

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Настройки панели X-UI
var (
	// PANEL_URL панели управления X-UI, включая уникальный путь.
	// Пример: "https://your.domain.com:54321/path/"
	PANEL_URL string

	// PANEL_USER имя пользователя для авторизации в API панели X-UI.
	PANEL_USER string

	// PANEL_PASS пароль для авторизации в API панели X-UI.
	PANEL_PASS string

	// INBOUND_ID входящего соединения (Inbound) в панели X-UI, для которого будут отслеживаться пользователи.
	INBOUND_ID int
)

// Инициализация настроек панели
func init() {
	// Загружаем переменные из .env файла
	err := godotenv.Load()
	if err != nil {
		log.Printf("Предупреждение: не удалось загрузить .env файл: %v", err)
	}

	// Читаем переменные из окружения
	PANEL_URL = os.Getenv("PANEL_URL")
	PANEL_USER = os.Getenv("PANEL_USER")
	PANEL_PASS = os.Getenv("PANEL_PASS")

	// Логируем успешную загрузку из .env (только если переменные заданы)
	if PANEL_URL != "" && PANEL_USER != "" && PANEL_PASS != "" {
		log.Println("Конфигурация панели успешно загружена из .env файла")
	}

	// Проверяем что все необходимые переменные заданы
	if PANEL_URL == "" {
		log.Println("ОШИБКА: PANEL_URL не задан в .env файле")
	}
	if PANEL_USER == "" {
		log.Println("ОШИБКА: PANEL_USER не задан в .env файле")
	}
	if PANEL_PASS == "" {
		log.Println("ОШИБКА: PANEL_PASS не задан в .env файле")
	}

	// ID входящего соединения (можно также вынести в .env при необходимости)
	inboundIDStr := os.Getenv("INBOUND_ID")
	if inboundIDStr != "" {
		if id, err := strconv.Atoi(inboundIDStr); err == nil {
			INBOUND_ID = id
		}
	}
}

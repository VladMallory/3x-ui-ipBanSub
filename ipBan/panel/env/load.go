// Пакет env: загружает конфигурацию панели из переменных окружения.
// Единственное действие файла — предоставить функцию MustLoad(),
// возвращающую конфиг для инициализации ConfigManager.
package env

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config содержит параметры подключения к панели x-ui
type Config struct {
	// PanelURL — базовый URL панели x-ui, оканчивается слешем, например: "http://localhost:54321/"
	PanelURL string
	// PanelUser — имя пользователя для авторизации в панели
	PanelUser string
	// PanelPass — пароль пользователя для авторизации в панели
	PanelPass string
	// InboundID — идентификатор inbound, с которым работает сервис
	InboundID int
}

// MustLoad загружает .env (если есть) и читает необходимые переменные окружения.
// При отсутствии/некорректности значений завершает процесс через log.Fatalf.
func MustLoad() Config {
	// Загружаем переменные из файла .env; при ошибке — завершаем
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Ошибка загрузки .env файла: %v", err)
	}

	// Формируем конфигурацию из обязательных переменных окружения
	return Config{
		PanelURL:  mustGet("PANEL_URL"),
		PanelUser: mustGet("PANEL_USER"),
		PanelPass: mustGet("PANEL_PASS"),
		InboundID: mustGetInt("INBOUND_ID"),
	}
}

// mustGet возвращает значение обязательной переменной окружения или завершает приложение
func mustGet(key string) string {
	// Получаем значение переменной из окружения
	v, ok := os.LookupEnv(key)
	// Проверяем, что переменная существует и не пустая
	if !ok || strings.TrimSpace(v) == "" {
		log.Fatalf("Переменная окружения %s обязательна и должна быть задана", key)
	}
	return v
}

// mustGetInt читает обязательную переменную и преобразует её в целое число
func mustGetInt(key string) int {
	vStr := mustGet(key)
	// Пробуем преобразовать строку в число
	v, err := strconv.Atoi(vStr)
	// Если преобразование не удалось — завершаем выполнение
	if err != nil {
		log.Fatalf("Переменная окружения %s должна быть целым числом, текущее значение: %q", key, vStr)
	}
	return v
}

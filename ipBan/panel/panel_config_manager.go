// Пакет panel: ConfigManager управляет HTTP-взаимодействием с панелью (куки, базовый URL, HTTP-клиент).
// Единственное действие файла — определить тип ConfigManager и фабричную функцию NewConfigManager.
package panel

import (
	"net/http"
	"time"
)

// Пакет panel: ConfigManager управляет HTTP-взаимодействием с панелью (куки, базовый URL, запросы).
type ConfigManager struct {
	// PanelURL — базовый URL панели x-ui, используется для построения запросов
	PanelURL string
	// PanelUser — имя пользователя панели для авторизации
	PanelUser string
	// PanelPass — пароль пользователя панели для авторизации
	PanelPass string
	// InboundID — идентификатор inbound, для которого выполняются операции
	InboundID int
	// Client — общий HTTP‑клиент с таймаутом, через который выполняются запросы
	Client *http.Client
	// SessionCookie — сериализованная кука сессии (например, "3x-ui=..."), добавляется к запросам
	SessionCookie string
}

// NewConfigManager создает новый менеджер конфигураций
// NewConfigManager создает и возвращает новый менеджер конфигураций панели.
// Он инкапсулирует базовые параметры и HTTP-клиент с таймаутом.
func NewConfigManager(panelURL, panelUser, panelPass string, inboundID int) *ConfigManager {
	// Инициализируем HTTP‑клиент с разумным таймаутом запросов
	return &ConfigManager{
		PanelURL:  panelURL,
		PanelUser: panelUser,
		PanelPass: panelPass,
		InboundID: inboundID,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Пакет client: типы Client/Settings и загрузка настроек клиентов из inbound.Settings.
package client

import (
	"encoding/json"
	"fmt"

	"ipBanSystem/ipBan/panel"
	"ipBanSystem/ipBan/panel/inbound"
)

// Client структура клиента для x-ui
type Client struct {
	// ID — уникальный идентификатор клиента в рамках inbound
	ID int `json:"id"`
	// InboundID — идентификатор inbound, к которому относится клиент
	InboundID int `json:"inboundId"`
	// Enable — флаг включённости клиента (разрешен доступ)
	Enable bool `json:"enable"`
	// Email — уникальный идентификатор пользователя (может использоваться как логин)
	Email string `json:"email"`
	// UUID — ключ доступа клиента, используется в протоколе (например, vless)
	UUID string `json:"uuid"`
	// Flow — дополнительные параметры потока (зависит от протокола), обычно пусто
	Flow string `json:"flow"`
	// Limitip — ограничение по IP для клиента (0 — без ограничений)
	Limitip int `json:"limitip"`
	// TotalGB — общий лимит трафика в гигабайтах
	TotalGB int64 `json:"totalGB"`
	// ExpiryTime — время истечения доступа (unix timestamp, миллисекунды)
	ExpiryTime int64 `json:"expiryTime"`
	// Reset — сброс счётчиков трафика (значение зависит от панели)
	Reset int `json:"reset"`
}

// Settings структура настроек клиентов
type Settings struct {
	// Clients — массив клиентов, входящих в настройки inbound
	Clients []Client `json:"clients"`
}

// GetSettings получает настройки клиентов
func GetSettings(cm *panel.ConfigManager) (*Settings, error) {
	// Запрашиваем объект inbound для доступа к строке настроек
	inb, err := inbound.GetInbound(cm)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения inbound: %v", err)
	}

	// Декодируем JSON‑строку настроек inbound в структуру Settings
	var settings Settings
	if err := json.Unmarshal([]byte(inb.Settings), &settings); err != nil {
		return nil, fmt.Errorf("ошибка парсинга настроек: %v", err)
	}

	// Возвращаем распарсенные настройки
	return &settings, nil
}

// Пакет client: вспомогательная функция для автокоррекции VLESS настроек
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"ipBanSystem/ipBan/panel"
	"ipBanSystem/ipBan/panel/api"
	"ipBanSystem/ipBan/panel/inbound"
)

// EnsureVLESSDecryptionNone гарантирует, что в inbound.Settings установлен decryption:"none" для VLESS
// Если значение уже корректно, функция ничего не меняет.
func EnsureVLESSDecryptionNone(cm *panel.ConfigManager) error {
	// Загружаем текущий inbound
	inb, err := inbound.GetInbound(cm)
	if err != nil {
		return fmt.Errorf("ошибка получения inbound: %v", err)
	}

	// Проверяем, что протокол VLESS — иначе не требуется правка
	if inb.Protocol != "vless" {
		return nil
	}

	// Парсим настройки inbound как произвольный JSON
	var raw map[string]interface{}
	if err = json.Unmarshal([]byte(inb.Settings), &raw); err != nil {
		return fmt.Errorf("ошибка парсинга настроек inbound: %v", err)
	}

	// Если decryption уже "none", изменений не требуется
	if dec, ok := raw["decryption"].(string); ok && dec == "none" {
		return nil
	}

	// Устанавливаем требуемое значение
	raw["decryption"] = "none"

	// Сериализуем обратно в строку
	settingsJSON, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("ошибка сериализации настроек: %v", err)
	}
	inb.Settings = string(settingsJSON)

	// Обновляем inbound в панели
	url := fmt.Sprintf("%spanel/api/inbounds/update/%d", cm.PanelURL, cm.InboundID)
	inbJSON, err := json.Marshal(inb)
	if err != nil {
		return fmt.Errorf("ошибка сериализации inbound: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(inbJSON))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Cookie", cm.SessionCookie)

	resp, err := cm.Client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("некорректный статус ответа: %d, body=%s", resp.StatusCode, string(body))
	}

	var response api.APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if !response.Success {
		return fmt.Errorf("ошибка обновления inbound настроек: %s", response.Msg)
	}

	return nil
}

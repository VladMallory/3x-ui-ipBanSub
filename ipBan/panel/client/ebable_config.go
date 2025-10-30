// Пакет client: корректное включение конфига по образцу ProxyMaster.
// Логика работает через точечный JSON‑патч поля enable конкретного клиента,
// без пересборки массива clients — это сохраняет все неизвестные поля (например, subId).
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

// EnableConfig включает клиента по email или по строковому id, аккуратно патча JSON настроек.
func EnableConfig(cm *panel.ConfigManager, emailOrID string) error {
	// Пробуем найти клиента по email (без учёта регистра)
	if c, err := ByEmail(cm, emailOrID); err == nil {
		return patchEnableByID(cm, c.ID, true)
	}
	// Если по email не нашли — считаем, что передан id
	return patchEnableByID(cm, emailOrID, true)
}

// patchEnableByID меняет только поле enable у клиента по id, не затрагивая остальные ключи.
func patchEnableByID(cm *panel.ConfigManager, clientID string, enable bool) error {
	// Получаем inbound
	inb, err := inbound.GetInbound(cm)
	if err != nil {
		return fmt.Errorf("ошибка получения inbound: %v", err)
	}

	// Парсим настройки inbound как произвольный JSON
	var raw map[string]interface{}
	if err = json.Unmarshal([]byte(inb.Settings), &raw); err != nil {
		return fmt.Errorf("ошибка парсинга исходных настроек inbound: %v", err)
	}

	// Получаем массив клиентов как []interface{}
	clientsAny, ok := raw["clients"].([]interface{})
	if !ok {
		return fmt.Errorf("поле clients отсутствует или имеет неверный тип")
	}

	// Находим клиента по id и патчим только enable
	clientFound := false
	for i := range clientsAny {
		m, ok := clientsAny[i].(map[string]interface{})
		if !ok {
			continue
		}
		idVal, _ := m["id"].(string)
		if idVal == clientID {
			m["enable"] = enable
			clientsAny[i] = m
			clientFound = true
			break
		}
	}
	if !clientFound {
		return fmt.Errorf("клиент с ID %s не найден", clientID)
	}

	// Возвращаем обновленный массив клиентов и гарантируем decryption:"none"
	raw["clients"] = clientsAny
	if dec, ok := raw["decryption"].(string); !ok || dec != "none" {
		raw["decryption"] = "none"
	}

	// Сериализуем обратно в строку settings
	settingsJSON, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("ошибка сериализации настроек: %v", err)
	}
	inb.Settings = string(settingsJSON)

	// Готовим запрос на обновление inbound
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
		return fmt.Errorf("ошибка обновления клиента: %s", response.Msg)
	}

	// Жёсткий ресет Remark, чтобы панель/Xray надёжно применили изменения
	if err := HardResetInbound(cm); err != nil {
		return fmt.Errorf("обновление прошло, но жёсткий ресет не удался: %v", err)
	}

	return nil
}

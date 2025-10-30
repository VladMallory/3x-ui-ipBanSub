// Пакет client: сброс статуса "исчерпано" (depleted/exhausted=false) при разбане клиента.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ipBanSystem/ipBan/panel"
	"ipBanSystem/ipBan/panel/api"
	"ipBanSystem/ipBan/panel/inbound"
)

// ResetDepletedStatus сбрасывает depleted/exhausted у клиента (email) и сохраняет изменения в панели.
// Логика: точечный патч найденного клиента в inbound.Settings, установка depleted=false, exhausted=false.
// Никаких лишних изменений массива clients, сохраняем неизвестные поля (subId, flow и т.п.).
func ResetDepletedStatus(cm *panel.ConfigManager, email string) error {
	// Загружаем текущий inbound
	inb, err := inbound.GetInbound(cm)
	if err != nil {
		return fmt.Errorf("ошибка получения inbound: %v", err)
	}

	// Парсим настройки inbound как произвольный JSON
	var raw map[string]interface{}
	if err = json.Unmarshal([]byte(inb.Settings), &raw); err != nil {
		return fmt.Errorf("ошибка парсинга настроек inbound: %v", err)
	}

	// Получаем массив клиентов как []interface{}
	clientsAny, ok := raw["clients"].([]interface{})
	if !ok {
		return fmt.Errorf("поле clients отсутствует или имеет неверный тип")
	}

	// Патчим только нужного клиента по email: depleted=false, exhausted=false
	clientFound := false
	falseVal := false
	for i := range clientsAny {
		m, ok := clientsAny[i].(map[string]interface{})
		if !ok {
			continue
		}
		em, _ := m["email"].(string)
		if strings.EqualFold(em, email) {
			m["depleted"] = &falseVal
			m["exhausted"] = &falseVal
			clientsAny[i] = m
			clientFound = true
			break
		}
	}

	if !clientFound {
		return fmt.Errorf("клиент с email %s не найден", email)
	}

	// Возвращаем обновлённый массив клиентов и гарантируем decryption:"none"
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

	// Отправляем апдейт inbound
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

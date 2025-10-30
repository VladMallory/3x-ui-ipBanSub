// RotateUUID отключает клиента и меняет его UUID через JSON‑патч, не теряя неизвестные поля
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"ipBanSystem/ipBan/panel"
	"ipBanSystem/ipBan/panel/api"
	"ipBanSystem/ipBan/panel/inbound"
)

// RotateUUID отключает клиента и генерирует новый UUID по email, аккуратно патчит JSON
func RotateUUID(cm *panel.ConfigManager, email string) (string, error) {
	// Получаем inbound
	inb, err := inbound.GetInbound(cm)
	if err != nil {
		return "", fmt.Errorf("ошибка получения inbound: %v", err)
	}

	// Парсим исходные настройки как произвольный JSON
	var raw map[string]interface{}
	if err = json.Unmarshal([]byte(inb.Settings), &raw); err != nil {
		return "", fmt.Errorf("ошибка парсинга исходных настроек inbound: %v", err)
	}

	// Достаём массив клиентов как []interface{}
	clientsAny, ok := raw["clients"].([]interface{})
	if !ok {
		return "", fmt.Errorf("поле clients отсутствует или имеет неверный тип")
	}

	// Ищем клиента по email (без учёта регистра) и патчим нужные поля
	newUUID := ""
	found := false
	for i := range clientsAny {
		m, ok := clientsAny[i].(map[string]interface{})
		if !ok {
			continue
		}
		em, _ := m["email"].(string)
		if strings.EqualFold(em, email) {
			// Отключаем клиента
			m["enable"] = false
			// Генерируем новый UUID
			newUUID = uuid.New().String()
			m["id"] = newUUID
			clientsAny[i] = m
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("клиент с email %s не найден", email)
	}

	// Обновляем массив клиентов обратно в raw и гарантируем decryption:"none"
	raw["clients"] = clientsAny
	if dec, ok := raw["decryption"].(string); !ok || dec != "none" {
		raw["decryption"] = "none"
	}

	// Сериализуем обратно в строку settings
	settingsJSON, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации настроек: %v", err)
	}
	inb.Settings = string(settingsJSON)

	// Обновляем inbound в панели
	url := fmt.Sprintf("%spanel/api/inbounds/update/%d", cm.PanelURL, cm.InboundID)
	inbJSON, err := json.Marshal(inb)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации inbound: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(inbJSON))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Cookie", cm.SessionCookie)

	resp, err := cm.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("некорректный статус ответа: %d, body=%s", resp.StatusCode, string(body))
	}

	var response api.APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if !response.Success {
		return "", fmt.Errorf("ошибка обновления клиента: %s", response.Msg)
	}

	// Жёсткий ресет, чтобы Xray/панель применили изменения
	if err := HardResetInbound(cm); err != nil {
		return "", fmt.Errorf("UUID обновлён, но жёсткий ресет не удался: %v", err)
	}

	return newUUID, nil
}

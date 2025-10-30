package client

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/google/uuid"

    "ipBanSystem/ipBan/panel"
    "ipBanSystem/ipBan/panel/api"
    "ipBanSystem/ipBan/panel/inbound"
)

// AggressiveBanReset выполняет максимально жёсткую процедуру для бана:
// - отключает клиента
// - выставляет depleted/exhausted=true ("исчерпано")
// - меняет email на email+"-reset" для первого апдейта
// - меняет UUID
// - применяет апдейт
// - возвращает email назад, сбрасывает depleted/exhausted=false
// - применяет второй апдейт
// Возвращает новый UUID.
func AggressiveBanReset(cm *panel.ConfigManager, email string) (string, error) {
    // Получаем inbound
    inb, err := inbound.GetInbound(cm)
    if err != nil {
        return "", fmt.Errorf("ошибка получения inbound: %v", err)
    }

    // Парсим исходные настройки как произвольный JSON
    var raw map[string]interface{}
    if err := json.Unmarshal([]byte(inb.Settings), &raw); err != nil {
        return "", fmt.Errorf("ошибка парсинга исходных настроек inbound: %v", err)
    }

    // Достаём массив клиентов как []interface{}
    clientsAny, ok := raw["clients"].([]interface{})
    if !ok {
        return "", fmt.Errorf("поле clients отсутствует или имеет неверный тип")
    }

    // Фаза A: патчим только целевого клиента по email: enable=false, depleted/exhausted=true, email+"-reset", id=newUUID
    newUUID := ""
    resetEmail := ""
    trueVal := true
    found := false
    for i := range clientsAny {
        m, ok := clientsAny[i].(map[string]interface{})
        if !ok {
            continue
        }
        em, _ := m["email"].(string)
        if strings.EqualFold(em, email) {
            resetEmail = em + "-reset"
            m["enable"] = false
            m["depleted"] = &trueVal
            m["exhausted"] = &trueVal
            m["email"] = resetEmail
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

    // Обновляем клиентов и принудительно ставим decryption=none
    raw["clients"] = clientsAny
    if dec, ok := raw["decryption"].(string); !ok || dec != "none" {
        raw["decryption"] = "none"
    }
    settingsJSON_A, err := json.Marshal(raw)
    if err != nil {
        return "", fmt.Errorf("ошибка сериализации настроек (A): %v", err)
    }
    inb.Settings = string(settingsJSON_A)

    // Отправляем апдейт A
    if err := postInboundUpdate(cm, inb); err != nil {
        return "", fmt.Errorf("ошибка обновления inbound (A): %v")
    }

    // Небольшая пауза
    time.Sleep(1000 * time.Millisecond)

    // Фаза B: возвращаем email, оставляя depleted/exhausted=true и enable=false
    for i := range clientsAny {
        m, ok := clientsAny[i].(map[string]interface{})
        if !ok {
            continue
        }
        em, _ := m["email"].(string)
        if em == resetEmail {
            // Вернём исходный email
            m["email"] = strings.TrimSuffix(resetEmail, "-reset")
            m["enable"] = false
            m["depleted"] = &trueVal
            m["exhausted"] = &trueVal
            clientsAny[i] = m
            break
        }
    }

    raw["clients"] = clientsAny
    if dec, ok := raw["decryption"].(string); !ok || dec != "none" {
        raw["decryption"] = "none"
    }
    settingsJSON_B, err := json.Marshal(raw)
    if err != nil {
        return "", fmt.Errorf("ошибка сериализации настроек (B): %v", err)
    }
    inb.Settings = string(settingsJSON_B)

    // Отправляем апдейт B
    if err := postInboundUpdate(cm, inb); err != nil {
        return "", fmt.Errorf("ошибка обновления inbound (B): %v")
    }

    // Жёсткий ресет Remark
    if err := HardResetInbound(cm); err != nil {
        return "", fmt.Errorf("жёсткий ресет Remark не удался: %v")
    }

    return newUUID, nil
}

// postInboundUpdate отправляет объект inbound в панель
func postInboundUpdate(cm *panel.ConfigManager, inb *inbound.Inbound) error {
	url := fmt.Sprintf("%spanel/api/inbounds/update/%d", cm.PanelURL, cm.InboundID)
	inbJSON, err := json.Marshal(inb)
	if err != nil {
		return fmt.Errorf("ошибка сериализации inbound: %v")
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(inbJSON))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %v")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Cookie", cm.SessionCookie)
	resp, err := cm.Client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %v")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения ответа: %v")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("некорректный статус ответа: %d, body=%s", resp.StatusCode, string(body))
	}
	var response api.APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("ошибка парсинга JSON: %v")
	}
	if !response.Success {
		return fmt.Errorf("ошибка обновления inbound: %s", response.Msg)
	}
	return nil
}

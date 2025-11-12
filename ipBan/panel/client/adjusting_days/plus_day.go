package adjustingdays

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ipBanSystem/ipBan/panel"
	"ipBanSystem/ipBan/panel/api"
	"ipBanSystem/ipBan/panel/client"
	"ipBanSystem/ipBan/panel/inbound"
)

// AddOneDay увеличивает expiryTime целевого клиента (по email) на +1 день.
// База продления: max(текущий expiryTime, текущее время).
func AddOneDay(cm *panel.ConfigManager, email string) error {
	inb, err := inbound.GetInbound(cm)
	if err != nil {
		return fmt.Errorf("ошибка получения inbound: %v", err)
	}

	var raw map[string]interface{}
	if err = json.Unmarshal([]byte(inb.Settings), &raw); err != nil {
		return fmt.Errorf("ошибка парсинга настроек inbound: %v", err)
	}

	clientsAny, ok := raw["clients"].([]interface{})
	if !ok {
		return fmt.Errorf("поле clients отсутствует или имеет неверный тип")
	}

	found := false
	nowMs := time.Now().UnixMilli()
	const dayMs = int64(24 * time.Hour / time.Millisecond)

	for i := range clientsAny {
		m, ok := clientsAny[i].(map[string]interface{})
		if !ok {
			continue
		}
		em, _ := m["email"].(string)
		if strings.EqualFold(em, email) {
			var currMs int64
			switch v := m["expiryTime"].(type) {
			case float64:
				currMs = int64(v)
			case int64:
				currMs = v
			case json.Number:
				if vi, e := v.Int64(); e == nil {
					currMs = vi
				}
			case string:
				// best-effort: parse numeric string
				if vi, e := parseInt64(v); e == nil {
					currMs = vi
				}
			default:
				currMs = 0
			}

			base := currMs
			if nowMs > base {
				base = nowMs
			}
			m["expiryTime"] = base + dayMs
			clientsAny[i] = m
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("клиент с email %s не найден", email)
	}

	raw["clients"] = clientsAny
	if dec, ok := raw["decryption"].(string); !ok || dec != "none" {
		raw["decryption"] = "none"
	}

	settingsJSON, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("ошибка сериализации настроек: %v", err)
	}
	inb.Settings = string(settingsJSON)

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

	if err := client.HardResetInbound(cm); err != nil {
		return fmt.Errorf("обновление прошло, но жёсткий ресет не удался: %v", err)
	}

	return nil
}

func parseInt64(s string) (int64, error) {
	var n json.Number = json.Number(s)
	return n.Int64()
}

// Пакет client: жёсткий ресет inbound через смену Remark туда‑сюда
package client

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
	"ipBanSystem/ipBan/panel/inbound"
)

// HardResetInbound выполняет двойное обновление Remark, чтобы панель и Xray заметили изменение:
// 1) remark -> remark+"-reset"
// 2) remark+"-reset" -> remark
func HardResetInbound(cm *panel.ConfigManager) error {
	inb, err := inbound.GetInbound(cm)
	if err != nil {
		return fmt.Errorf("ошибка получения inbound: %v")
	}

	// Нормализуем базовое имя без суффикса -reset
	base := strings.TrimSuffix(inb.Remark, "-reset")
	reset := base + "-reset"

	// Первый апдейт: remark -> remark-reset
	if err := updateInboundRemark(cm, inb, reset); err != nil {
		return fmt.Errorf("ошибка первого обновления Remark: %v")
	}

	// Небольшая пауза, чтобы панель/Xray применили изменение
	time.Sleep(500 * time.Millisecond)

	// Второй апдейт: remark-reset -> remark
	if err := updateInboundRemark(cm, inb, base); err != nil {
		return fmt.Errorf("ошибка второго обновления Remark: %v")
	}

	return nil
}

// updateInboundRemark обновляет поле Remark и отправляет объект inbound в панель
func updateInboundRemark(cm *panel.ConfigManager, inb *inbound.Inbound, newRemark string) error {
	inb.Remark = newRemark

	// Сериализуем и отправляем
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

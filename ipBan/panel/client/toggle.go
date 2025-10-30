// Пакет client: включает/отключает клиента и сохраняет изменения в панели.
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

// Enable включает клиента по email или ID
func Enable(cm *panel.ConfigManager, emailOrID interface{}) error {
    // Ветвим логику по типу входного параметра: int или string
    switch v := emailOrID.(type) {
    case int:
        // Историческая совместимость: int не поддерживается для строкового id
        return fmt.Errorf("ID теперь строковый, передайте string")
    case string:
        // Ищем клиента по email и включаем его по найденному ID
        // Если это email — найдём клиента; если это строковый id — обновим напрямую
        if c, err := ByEmail(cm, v); err == nil {
            return updateConfig(cm, c.ID, true)
        }
        // Не нашли по email — считаем, что передан id
        return updateConfig(cm, v, true)
    default:
        return fmt.Errorf("неподдерживаемый тип параметра: %T", emailOrID)
    }
}

// Disable отключает клиента по email или ID
func Disable(cm *panel.ConfigManager, emailOrID interface{}) error {
    // Ветвим логику по типу входного параметра: int или string
    switch v := emailOrID.(type) {
    case int:
        // Историческая совместимость: int не поддерживается для строкового id
        return fmt.Errorf("ID теперь строковый, передайте string")
    case string:
        if c, err := ByEmail(cm, v); err == nil {
            return updateConfig(cm, c.ID, false)
        }
        return updateConfig(cm, v, false)
    default:
        return fmt.Errorf("неподдерживаемый тип параметра: %T", emailOrID)
    }
}

// updateConfig изменяет флаг Enable клиента по ID и отправляет обновление inbound
func updateConfig(cm *panel.ConfigManager, clientID string, enable bool) error {
    // Загружаем текущие настройки клиентов
    settings, err := GetSettings(cm)
    if err != nil {
        return fmt.Errorf("ошибка получения настроек клиентов: %v", err)
    }

    // Ищем клиента по ID и обновляем флаг Enable
    clientFound := false
    for i := range settings.Clients {
        if settings.Clients[i].ID == clientID {
            settings.Clients[i].Enable = enable
            clientFound = true
            break
        }
    }

    if !clientFound {
        return fmt.Errorf("клиент с ID %s не найден", clientID)
    }

    // Получаем inbound для применения обновлённых настроек
    inb, err := inbound.GetInbound(cm)
    if err != nil {
        return fmt.Errorf("ошибка получения inbound: %v", err)
    }

    // Аккуратно обновляем JSON настроек inbound: сохраняем прочие поля и клиенты
    var raw map[string]interface{}
    if err := json.Unmarshal([]byte(inb.Settings), &raw); err != nil {
        return fmt.Errorf("ошибка парсинга исходных настроек inbound: %v", err)
    }
    raw["clients"] = settings.Clients
    if dec, ok := raw["decryption"].(string); !ok || dec != "none" {
        raw["decryption"] = "none"
    }
    settingsJSON, err := json.Marshal(raw)
    if err != nil {
        return fmt.Errorf("ошибка сериализации настроек: %v", err)
    }
    inb.Settings = string(settingsJSON)

    // Готовим запрос на обновление inbound в панели
    url := fmt.Sprintf("%spanel/api/inbounds/update/%d", cm.PanelURL, cm.InboundID)
    inbJSON, err := json.Marshal(inb)
    if err != nil {
        return fmt.Errorf("ошибка сериализации inbound: %v", err)
    }

    // Создаём POST‑запрос для сохранения изменений
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(inbJSON))
    if err != nil {
        return fmt.Errorf("ошибка создания запроса: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Add("Cookie", cm.SessionCookie)

    // Выполняем запрос и читаем ответ
    resp, err := cm.Client.Do(req)
    if err != nil {
        return fmt.Errorf("ошибка выполнения запроса: %v", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("ошибка чтения ответа: %v", err)
    }

    // Проверяем HTTP‑статус
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("некорректный статус ответа: %d, body=%s", resp.StatusCode, string(body))
    }

    // Парсим ответ панели и проверяем успешность операции
    var response api.APIResponse
    if err := json.Unmarshal(body, &response); err != nil {
        return fmt.Errorf("ошибка парсинга JSON: %v", err)
    }

    if !response.Success {
        return fmt.Errorf("ошибка обновления клиента: %s", response.Msg)
    }

    // Выполняем жёсткий ресет, чтобы панель/Xray наверняка применили изменения
    if err := HardResetInbound(cm); err != nil {
        return fmt.Errorf("обновление прошло, но жёсткий ресет не удался: %v", err)
    }

    return nil
}

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
        // Прямое включение по ID
        return updateConfig(cm, v, true)
    case string:
        // Ищем клиента по email и включаем его по найденному ID
        c, err := ByEmail(cm, v)
        if err != nil {
            return fmt.Errorf("ошибка получения клиента: %v", err)
        }
        return updateConfig(cm, c.ID, true)
    default:
        return fmt.Errorf("неподдерживаемый тип параметра: %T", emailOrID)
    }
}

// Disable отключает клиента по email или ID
func Disable(cm *panel.ConfigManager, emailOrID interface{}) error {
    // Ветвим логику по типу входного параметра: int или string
    switch v := emailOrID.(type) {
    case int:
        // Прямое отключение по ID
        return updateConfig(cm, v, false)
    case string:
        // Ищем клиента по email и отключаем его по найденному ID
        c, err := ByEmail(cm, v)
        if err != nil {
            return fmt.Errorf("ошибка получения клиента: %v", err)
        }
        return updateConfig(cm, c.ID, false)
    default:
        return fmt.Errorf("неподдерживаемый тип параметра: %T", emailOrID)
    }
}

// updateConfig изменяет флаг Enable клиента по ID и отправляет обновление inbound
func updateConfig(cm *panel.ConfigManager, clientID int, enable bool) error {
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
        return fmt.Errorf("клиент с ID %d не найден", clientID)
    }

    // Получаем inbound для применения обновлённых настроек
    inb, err := inbound.GetInbound(cm)
    if err != nil {
        return fmt.Errorf("ошибка получения inbound: %v", err)
    }

    // Сериализуем обновлённые настройки и присваиваем их inbound
    settingsJSON, err := json.Marshal(settings)
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

    return nil
}

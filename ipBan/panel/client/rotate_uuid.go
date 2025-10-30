// RotateUUID отключает клиента и меняет его UUID, затем сохраняет изменения
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

// RotateUUID отключает клиента и генерирует новый UUID по email
func RotateUUID(cm *panel.ConfigManager, email string) (string, error) {
    // Получаем текущие настройки клиентов
    settings, err := GetSettings(cm)
    if err != nil {
        return "", fmt.Errorf("ошибка получения настроек клиентов: %v", err)
    }

    var clientID string
    found := false
    // Находим клиента по email, отключаем и назначаем новый UUID
    for i := range settings.Clients {
        if strings.EqualFold(settings.Clients[i].Email, email) {
            // Отключаем клиента перед сменой учётных данных
            settings.Clients[i].Enable = false
            // Генерируем новый UUID и присваиваем в поле id (vless/vmess)
            newUUID := uuid.New().String()
            settings.Clients[i].ID = newUUID
            clientID = settings.Clients[i].ID
            found = true
            break
        }
    }

    // Если клиент не найден — возвращаем ошибку
    if !found {
        return "", fmt.Errorf("клиент с email %s не найден", email)
    }

    // Получаем inbound для дальнейшего обновления его настроек
    inb, err := inbound.GetInbound(cm)
    if err != nil {
        return "", fmt.Errorf("ошибка получения inbound: %v", err)
    }

    // Аккуратно обновляем JSON настроек inbound: сохраняем прочие поля и клиенты
    var raw map[string]interface{}
    if err := json.Unmarshal([]byte(inb.Settings), &raw); err != nil {
        return "", fmt.Errorf("ошибка парсинга исходных настроек inbound: %v", err)
    }
    raw["clients"] = settings.Clients
    if dec, ok := raw["decryption"].(string); !ok || dec != "none" {
        raw["decryption"] = "none"
    }

    settingsJSON, err := json.Marshal(raw)
    if err != nil {
        return "", fmt.Errorf("ошибка сериализации настроек: %v", err)
    }
    inb.Settings = string(settingsJSON)

    // Формируем URL эндпоинта обновления inbound
    url := fmt.Sprintf("%spanel/api/inbounds/update/%d", cm.PanelURL, cm.InboundID)
    // Сериализуем объект inbound для отправки
    inbJSON, err := json.Marshal(inb)
    if err != nil {
        return "", fmt.Errorf("ошибка сериализации inbound: %v", err)
    }

    // Создаём HTTP POST‑запрос с JSON‑телом
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(inbJSON))
    if err != nil {
        return "", fmt.Errorf("ошибка создания запроса: %v", err)
    }

    // Устанавливаем тип контента и добавляем сессионный cookie
    req.Header.Set("Content-Type", "application/json")
    req.Header.Add("Cookie", cm.SessionCookie)

    // Выполняем запрос
    resp, err := cm.Client.Do(req)
    if err != nil {
        return "", fmt.Errorf("ошибка выполнения запроса: %v", err)
    }
    defer resp.Body.Close()

    // Читаем тело ответа
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("ошибка чтения ответа: %v", err)
    }

    // Проверяем HTTP‑статус
    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("некорректный статус ответа: %d, body=%s", resp.StatusCode, string(body))
    }

    // Парсим ответ панели
    var response api.APIResponse
    if err := json.Unmarshal(body, &response); err != nil {
        return "", fmt.Errorf("ошибка парсинга JSON: %v", err)
    }

    // Проверяем флаг успешности операции
    if !response.Success {
        return "", fmt.Errorf("ошибка обновления клиента (ID %s): %s", clientID, response.Msg)
    }

    // Выполняем жёсткий ресет, чтобы панель/Xray заметили смену UUID
    if err := HardResetInbound(cm); err != nil {
        return "", fmt.Errorf("UUID обновлён, но жёсткий ресет не удался: %v", err)
    }

    // Находим клиента и возвращаем новый UUID
    for _, c := range settings.Clients {
        if c.ID == clientID {
            return c.ID, nil
        }
    }
    return "", fmt.Errorf("клиент с ID %s не найден после обновления", clientID)
}

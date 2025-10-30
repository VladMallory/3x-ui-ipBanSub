// Пакет client: добавляет нового клиента и сохраняет изменения в панели.
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

	"github.com/google/uuid"
)

// Add создаёт нового клиента и записывает в настройки inbound
func Add(cm *panel.ConfigManager, email string, totalGB int64, expiryTime int64) (*Client, error) {
    // Загружаем текущие настройки клиентов
    settings, err := GetSettings(cm)
    if err != nil {
        return nil, fmt.Errorf("ошибка получения настроек клиентов: %v", err)
    }

    // Проверяем, что клиент с таким email ещё не существует
    for _, c := range settings.Clients {
        if strings.EqualFold(c.Email, email) {
            return nil, fmt.Errorf("клиент с email %s уже существует", email)
        }
    }

    // Формируем структуру нового клиента
    newClient := Client{
        ID:         len(settings.Clients) + 1,
        InboundID:  cm.InboundID,
        Enable:     true,
        Email:      email,
        UUID:       uuid.New().String(),
        Flow:       "",
        Limitip:    0,
        TotalGB:    totalGB,
        ExpiryTime: expiryTime,
        Reset:      0,
    }

    // Добавляем клиента в список
    settings.Clients = append(settings.Clients, newClient)

    // Получаем текущий inbound, чтобы обновить его настройки
    inb, err := inbound.GetInbound(cm)
    if err != nil {
        return nil, fmt.Errorf("ошибка получения inbound: %v", err)
    }

    // Сериализуем обновлённые настройки клиентов и сохраняем в inbound
    settingsJSON, err := json.Marshal(settings)
    if err != nil {
        return nil, fmt.Errorf("ошибка сериализации настроек: %v", err)
    }
    inb.Settings = string(settingsJSON)

    // Подготавливаем запрос на обновление inbound в панели
    url := fmt.Sprintf("%spanel/api/inbounds/update/%d", cm.PanelURL, cm.InboundID)
    inbJSON, err := json.Marshal(inb)
    if err != nil {
        return nil, fmt.Errorf("ошибка сериализации inbound: %v", err)
    }

    // Создаём POST‑запрос на обновление настроек
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(inbJSON))
    if err != nil {
        return nil, fmt.Errorf("ошибка создания запроса: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Add("Cookie", cm.SessionCookie)

    // Выполняем запрос и читаем ответ
    resp, err := cm.Client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
    }

    // Проверяем HTTP‑статус
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("некорректный статус ответа: %d, body=%s", resp.StatusCode, string(body))
    }

    // Парсим ответ панели и проверяем успешность операции
    var response api.APIResponse
    if err := json.Unmarshal(body, &response); err != nil {
        return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
    }

    if !response.Success {
        return nil, fmt.Errorf("ошибка добавления клиента: %s", response.Msg)
    }

    // Возвращаем созданного клиента
    return &newClient, nil
}

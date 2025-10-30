// Пакет client: функции работы с клиентами (поиск, статус, перечисление).
package client

import (
    "fmt"
    "strings"

    "ipBanSystem/ipBan/panel"
)

// ByEmail возвращает клиента по email (без учета регистра)
func ByEmail(cm *panel.ConfigManager, email string) (*Client, error) {
    // Получаем все настройки клиентов
    settings, err := GetSettings(cm)
    if err != nil {
        return nil, fmt.Errorf("ошибка получения настроек клиентов: %v", err)
    }

    // Проходим по списку и ищем совпадение по email без учета регистра
    for _, c := range settings.Clients {
        if strings.EqualFold(c.Email, email) {
            return &c, nil
        }
    }
    return nil, fmt.Errorf("клиент с email %s не найден", email)
}

// ByID возвращает клиента по ID
func ByID(cm *panel.ConfigManager, clientID string) (*Client, error) {
    // Получаем все настройки клиентов
    settings, err := GetSettings(cm)
    if err != nil {
        return nil, fmt.Errorf("ошибка получения настроек клиентов: %v", err)
    }

    // Ищем в списке клиента с указанным ID
    for _, c := range settings.Clients {
        if c.ID == clientID {
            return &c, nil
        }
    }
    return nil, fmt.Errorf("клиент с ID %s не найден", clientID)
}

// All возвращает всех клиентов
func All(cm *panel.ConfigManager) ([]Client, error) {
    // Загружаем настройки и отдаём массив клиентов
    settings, err := GetSettings(cm)
    if err != nil {
        return nil, fmt.Errorf("ошибка получения настроек клиентов: %v", err)
    }
    return settings.Clients, nil
}

// Status возвращает флаг Enable по email
func Status(cm *panel.ConfigManager, email string) (bool, error) {
    // Находим клиента и возвращаем его поле Enable
    c, err := ByEmail(cm, email)
    if err != nil {
        return false, fmt.Errorf("ошибка получения клиента: %v", err)
    }
    return c.Enable, nil
}

// Пакет auth: отвечает за аутентификацию в панели x-ui.
// Login выполняет авторизацию и сохраняет сессионную куку в ConfigManager.
package auth

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"

    "ipBanSystem/ipBan/panel"
)

// LoginRequest структура для запроса авторизации
type LoginRequest struct {
    // Username — имя пользователя панели x-ui
    Username string `json:"username"`
    // Password — пароль пользователя панели x-ui
    Password string `json:"password"`
}

// LoginResponse структура для ответа авторизации
type LoginResponse struct {
    // Success — флаг успешности авторизации
    Success bool   `json:"success"`
    // Msg — сообщение панели (ошибка/успех)
    Msg     string `json:"msg"`
    // Obj — произвольный объект, возвращаемый панелью (может быть пустым)
    Obj     any    `json:"obj"`
}

// Login авторизует ConfigManager в панели x-ui
func Login(cm *panel.ConfigManager) error {
    // Формируем тело запроса авторизации
    loginData := LoginRequest{
        Username: cm.PanelUser,
        Password: cm.PanelPass,
    }

    // Сериализуем данные в JSON
    jsonData, err := json.Marshal(loginData)
    if err != nil {
        return fmt.Errorf("ошибка сериализации данных авторизации: %v", err)
    }

    // Создаём POST‑запрос к эндпоинту авторизации
    req, err := http.NewRequest("POST", cm.PanelURL+"login", strings.NewReader(string(jsonData)))
    if err != nil {
        return fmt.Errorf("ошибка создания запроса: %v", err)
    }
    req.Header.Set("Content-Type", "application/json")

    // Выполняем запрос через общий HTTP‑клиент
    resp, err := cm.Client.Do(req)
    if err != nil {
        return fmt.Errorf("ошибка выполнения запроса: %v", err)
    }
    defer resp.Body.Close()

    // Читаем тело ответа
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("ошибка чтения ответа: %v", err)
    }

    // Проверяем HTTP‑статус
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("некорректный статус ответа: %d, body=%s", resp.StatusCode, string(body))
    }

    // Парсим ответ панели
    var response LoginResponse
    if err := json.Unmarshal(body, &response); err != nil {
        return fmt.Errorf("ошибка парсинга JSON: %v", err)
    }

    // Проверяем флаг успеха
    if !response.Success {
        return fmt.Errorf("ошибка авторизации: %s", response.Msg)
    }

    // Извлекаем сессионную куку
    for _, cookie := range resp.Cookies() {
        if cookie.Name == "3x-ui" {
            // Сохраняем сериализованную куку в менеджер для дальнейших запросов
            cm.SessionCookie = cookie.String()
            return nil
        }
    }

    // Если не нашли необходимую куку — считаем это ошибкой
    return fmt.Errorf("сессионная кука не найдена в ответе")
}

// Пакет inbound: изолирует работу с объектом Inbound панели x-ui.
// GetInbound получает объект inbound из панели x-ui по ID менеджера.
package inbound

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"ipBanSystem/ipBan/panel"
)

// Inbound структура для входящего соединения
type Inbound struct {
	// ID — уникальный идентификатор inbound в панели
	ID int `json:"id"`
	// Up — объём исходящего трафика (байты)
	Up int64 `json:"up"`
	// Down — объём входящего трафика (байты)
	Down int64 `json:"down"`
	// Total — суммарный лимит трафика (байты)
	Total int64 `json:"total"`
	// Remark — произвольная метка/описание для данного inbound
	Remark string `json:"remark"`
	// Enable — флаг включённости inbound
	Enable bool `json:"enable"`
	// ExpiryTime — время истечения (unix timestamp, миллисекунды)
	ExpiryTime int64 `json:"expiryTime"`
	// Listen — адрес для прослушивания (опционально)
	Listen string `json:"listen"`
	// Port — порт, на котором работает inbound
	Port int `json:"port"`
	// Protocol — используемый протокол (например, "vless")
	Protocol string `json:"protocol"`
	// Settings — строка JSON с настройками клиентов (включая массив clients)
	Settings string `json:"settings"`
	// StreamSettings — строка JSON настроек транспортного уровня
	StreamSettings string `json:"streamSettings"`
	// Tag — тег для маршрутизации/идентификации
	Tag string `json:"tag"`
	// Sniffing — строка JSON с параметрами сниффинга
	Sniffing string `json:"sniffing"`
	// ClientStats — статистика по клиентам; тип зависит от настроек панели
	ClientStats interface{} `json:"clientStats"`
}

// GetInboundInfo структура для ответа с информацией о inbound
type GetInboundInfo struct {
	// Success — флаг успешности ответа панели
	Success bool `json:"success"`
	// Msg — сообщение об ошибке/успехе, предоставляемое панелью
	Msg string `json:"msg"`
	// Obj — полезная нагрузка с объектом inbound
	Obj Inbound `json:"obj"`
}

// GetInbound получает объект inbound
func GetInbound(cm *panel.ConfigManager) (*Inbound, error) {
	// Формируем URL запроса к ресурсу inbound по ID
	url := fmt.Sprintf("%spanel/api/inbounds/get/%d", cm.PanelURL, cm.InboundID)

	// Создаём GET‑запрос
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	// Добавляем сессионную куку для авторизованного доступа
	req.Header.Add("Cookie", cm.SessionCookie)

	// Выполняем запрос через общий HTTP‑клиент
	resp, err := cm.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	// Проверяем HTTP‑статус
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("некорректный статус ответа: %d, body=%s", resp.StatusCode, string(body))
	}

	// Парсим JSON‑ответ панели
	var response GetInboundInfo
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	// Проверяем флаг успеха — панель вернула корректный объект
	if !response.Success {
		return nil, fmt.Errorf("ошибка получения inbound: %s", response.Msg)
	}

	// Возвращаем объект inbound
	return &response.Obj, nil
}

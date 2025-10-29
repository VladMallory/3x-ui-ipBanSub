package panel

// Client структура для 3x-ui API
type Client struct {
	ID         string      `json:"id"`
	Flow       string      `json:"flow"`
	Email      string      `json:"email"`
	LimitIP    int         `json:"limitIp"`
	TotalGB    int         `json:"totalGB"`
	ExpiryTime int64       `json:"expiryTime"`
	Enable     bool        `json:"enable"`
	TgID       interface{} `json:"tgId"` // Может быть числом или строкой
	SubID      string      `json:"subId"`
	Reset      int         `json:"reset"`

	// Дополнительные поля, которые есть в реальном API
	CreatedAt int64 `json:"created_at,omitempty"`
	UpdatedAt int64 `json:"updated_at,omitempty"`

	// Попытка управлять состоянием "исчерпано"
	Depleted  *bool `json:"depleted,omitempty"`  // указатель, чтобы различать false и отсутствие поля
	Exhausted *bool `json:"exhausted,omitempty"` // на случай, если используется другое название
}

// Settings структура для поля settings
type Settings struct {
	Clients    []Client `json:"clients"`
	Decryption string   `json:"decryption"`
}

// AddClientRequest структура для добавления клиента
type AddClientRequest struct {
	ID       int    `json:"id"`
	Settings string `json:"settings"`
}

// UpdateClientRequest структура для обновления клиента
type UpdateClientRequest struct {
	ID       int    `json:"id"`
	Settings string `json:"settings"`
}
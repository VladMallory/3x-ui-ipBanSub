package panel

// LoginRequest структура для запроса авторизации
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse структура для ответа авторизации
type LoginResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

// APIResponse общая структура для API ответов
type APIResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

package panel

// TrafficStats структура для статистики трафика клиента
type TrafficStats struct {
	ID         int    `json:"id"`
	InboundID  int    `json:"inboundId"`
	Enable     bool   `json:"enable"`
	Email      string `json:"email"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	ExpiryTime int64  `json:"expiryTime"`
	Total      int64  `json:"total"`
	Reset      int    `json:"reset"`
}

// UsersStatistics структура для статистики пользователей
type UsersStatistics struct {
	TotalUsers          int     `json:"total_users"`
	PayingUsers         int     `json:"paying_users"`
	TrialAvailableUsers int     `json:"trial_available_users"`
	TrialUsedUsers      int     `json:"trial_used_users"`
	InactiveUsers       int     `json:"inactive_users"`
	ActiveConfigs       int     `json:"active_configs"`
	TotalRevenue        float64 `json:"total_revenue"`
	NewThisWeek         int     `json:"new_this_week"`
	NewThisMonth        int     `json:"new_this_month"`
	ConversionRate      float64 `json:"conversion_rate"`
}

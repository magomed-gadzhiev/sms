package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ClientConfig представляет конфигурацию клиента
type ClientConfig struct {
	ID                 uuid.UUID       `json:"id" db:"id"`
	ClientID           uuid.UUID       `json:"client_id" db:"client_id"`
	RateLimitPerSecond int             `json:"rate_limit_per_second" db:"rate_limit_per_second"`
	RateLimitPerMinute int             `json:"rate_limit_per_minute" db:"rate_limit_per_minute"`
	RateLimitPerHour   int             `json:"rate_limit_per_hour" db:"rate_limit_per_hour"`
	RateLimitPerDay    int             `json:"rate_limit_per_day" db:"rate_limit_per_day"`
	AllowedSources     []string        `json:"allowed_sources" db:"allowed_sources"`
	BlockedDestinations []string       `json:"blocked_destinations" db:"blocked_destinations"`
	Settings           json.RawMessage `json:"settings" db:"settings"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
}

// RateLimits представляет rate limits
type RateLimits struct {
	PerSecond int `json:"per_second"`
	PerMinute int `json:"per_minute"`
	PerHour   int `json:"per_hour"`
	PerDay    int `json:"per_day"`
}

// ToRateLimits преобразует ClientConfig в RateLimits
func (c *ClientConfig) ToRateLimits() *RateLimits {
	return &RateLimits{
		PerSecond: c.RateLimitPerSecond,
		PerMinute: c.RateLimitPerMinute,
		PerHour:   c.RateLimitPerHour,
		PerDay:    c.RateLimitPerDay,
	}
}

// GetSettings возвращает настройки как map
func (c *ClientConfig) GetSettings() map[string]string {
	if c.Settings == nil || len(c.Settings) == 0 {
		return make(map[string]string)
	}

	var settings map[string]string
	if err := json.Unmarshal(c.Settings, &settings); err != nil {
		return make(map[string]string)
	}

	return settings
}

// SetSettings устанавливает настройки из map
func (c *ClientConfig) SetSettings(settings map[string]string) error {
	if settings == nil {
		c.Settings = json.RawMessage("{}")
		return nil
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	c.Settings = json.RawMessage(data)
	return nil
}

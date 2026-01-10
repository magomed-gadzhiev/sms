package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Client представляет клиента системы
type Client struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	Name          string          `json:"name" db:"name"`
	Email         string          `json:"email" db:"email"`
	ContactPerson string          `json:"contact_person" db:"contact_person"`
	Phone         string          `json:"phone" db:"phone"`
	Active        bool            `json:"active" db:"active"`
	Metadata      json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`

	// Связи
	Config *ClientConfig `json:"config,omitempty" db:"-"`
}

// IsActive проверяет, активен ли клиент
func (c *Client) IsActive() bool {
	return c.Active
}

// GetMetadata возвращает метаданные как map
func (c *Client) GetMetadata() map[string]string {
	if c.Metadata == nil || len(c.Metadata) == 0 {
		return make(map[string]string)
	}

	var metadata map[string]string
	if err := json.Unmarshal(c.Metadata, &metadata); err != nil {
		return make(map[string]string)
	}

	return metadata
}

// SetMetadata устанавливает метаданные из map
func (c *Client) SetMetadata(metadata map[string]string) error {
	if metadata == nil {
		c.Metadata = json.RawMessage("{}")
		return nil
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	c.Metadata = json.RawMessage(data)
	return nil
}

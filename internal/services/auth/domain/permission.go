package domain

import (
	"time"

	"github.com/google/uuid"
)

// Permission представляет право доступа
type Permission struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Resource    string    `json:"resource" db:"resource"`
	Action      string    `json:"action" db:"action"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// String возвращает строковое представление права в формате "resource:action"
func (p *Permission) String() string {
	return p.Resource + ":" + p.Action
}

// Equals проверяет, равны ли два права
func (p *Permission) Equals(other *Permission) bool {
	return p.Resource == other.Resource && p.Action == other.Action
}

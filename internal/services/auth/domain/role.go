package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role представляет роль пользователя
type Role struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	// Загружаемые связи
	Permissions []Permission `json:"permissions,omitempty" db:"-"`
	UserCount   int32        `json:"user_count,omitempty" db:"-"`
}

// RoleName представляет предопределенные имена ролей
type RoleName string

const (
	RoleAdmin    RoleName = "admin"
	RoleClient   RoleName = "client"
	RoleOperator RoleName = "operator"
)

// HasPermission проверяет, имеет ли роль указанное право
func (r *Role) HasPermission(resource, action string) bool {
	if r.Permissions == nil {
		return false
	}

	for _, perm := range r.Permissions {
		if perm.Resource == resource && perm.Action == action {
			return true
		}
	}

	return false
}

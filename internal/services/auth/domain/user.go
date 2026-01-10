package domain

import (
	"time"

	"github.com/google/uuid"
)

// User представляет пользователя системы
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	RoleID       uuid.UUID `json:"role_id" db:"role_id"`
	Active       bool      `json:"active" db:"active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`

	// Загружаемые связи
	Role       *Role        `json:"role,omitempty" db:"-"`
	Permissions []Permission `json:"permissions,omitempty" db:"-"`
}

// IsActive проверяет, активен ли пользователь
func (u *User) IsActive() bool {
	return u.Active
}

// HasPermission проверяет, имеет ли пользователь указанное право
func (u *User) HasPermission(resource, action string) bool {
	if u.Permissions == nil {
		return false
	}

	for _, perm := range u.Permissions {
		if perm.Resource == resource && perm.Action == action {
			return true
		}
	}

	return false
}

// HasAnyPermission проверяет, имеет ли пользователь хотя бы одно из указанных прав
func (u *User) HasAnyPermission(permissions ...Permission) bool {
	if u.Permissions == nil {
		return false
	}

	for _, requiredPerm := range permissions {
		for _, userPerm := range u.Permissions {
			if userPerm.Resource == requiredPerm.Resource &&
				userPerm.Action == requiredPerm.Action {
				return true
			}
		}
	}

	return false
}

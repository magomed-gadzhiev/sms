package domain

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// APIKey представляет API ключ пользователя
type APIKey struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	Name       string     `json:"name" db:"name"`
	KeyHash    string     `json:"-" db:"key_hash"`    // Хеш ключа (никогда не возвращаем сам ключ)
	KeyPrefix  string     `json:"prefix" db:"key_prefix"` // Префикс для отображения
	Active     bool       `json:"active" db:"active"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`

	// Загружаемые связи
	Scopes     []string `json:"scopes,omitempty" db:"-"`
	AllowedIPs []string `json:"allowed_ips,omitempty" db:"allowed_ips"`
}

// IsExpired проверяет, истек ли срок действия ключа
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid проверяет, валиден ли ключ (активен и не истек)
func (k *APIKey) IsValid() bool {
	return k.Active && !k.IsExpired()
}

// IsIPAllowed проверяет, разрешён ли IP адрес для данного ключа.
// Если AllowedIPs пуст — ограничений нет, доступ разрешён.
// Поддерживает как точные IP адреса, так и CIDR нотацию.
func (k *APIKey) IsIPAllowed(ip string) bool {
	if len(k.AllowedIPs) == 0 {
		return true
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, allowed := range k.AllowedIPs {
		// Пробуем как CIDR
		_, cidr, err := net.ParseCIDR(allowed)
		if err == nil {
			if cidr.Contains(parsedIP) {
				return true
			}
			continue
		}

		// Пробуем как точный IP
		if net.ParseIP(allowed) != nil && allowed == ip {
			return true
		}
	}

	return false
}

// HasScope проверяет, имеет ли ключ указанный scope
func (k *APIKey) HasScope(scope string) bool {
	if k.Scopes == nil || len(k.Scopes) == 0 {
		return true // Если scopes не указаны, ключ имеет все права пользователя
	}

	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}

	return false
}

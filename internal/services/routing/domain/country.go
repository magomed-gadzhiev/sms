package domain

import (
	"time"

	"github.com/google/uuid"
)

// Country представляет доменную модель страны
type Country struct {
	ID        uuid.UUID
	Name      string
	ISOCode   string // ISO 3166-1 alpha-2
	PhoneCode string // e.g. "+7"
	Currency  string // ISO 4217
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewCountry создает новую страну
func NewCountry(name, isoCode, phoneCode, currency string) *Country {
	now := time.Now()
	return &Country{
		ID:        uuid.New(),
		Name:      name,
		ISOCode:   isoCode,
		PhoneCode: phoneCode,
		Currency:  currency,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

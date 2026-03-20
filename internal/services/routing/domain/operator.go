package domain

import (
	"time"

	"github.com/google/uuid"
)

// Operator представляет доменную модель мобильного оператора
type Operator struct {
	ID                 uuid.UUID
	CountryID          uuid.UUID
	Name               string
	Code               string // unique code e.g. "mts-ru"
	SupportsPaidSender bool
	SupportsFreeSender bool
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewOperator создает нового оператора
func NewOperator(countryID uuid.UUID, name, code string, supportsPaid, supportsFree bool) *Operator {
	now := time.Now()
	return &Operator{
		ID:                 uuid.New(),
		CountryID:          countryID,
		Name:               name,
		Code:               code,
		SupportsPaidSender: supportsPaid,
		SupportsFreeSender: supportsFree,
		Active:             true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

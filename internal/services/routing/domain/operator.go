package domain

import (
	"time"

	"github.com/google/uuid"
)

// Operator представляет доменную модель мобильного оператора
type Operator struct {
	ID                  uuid.UUID
	CountryID           uuid.UUID
	Name                string
	Code                string // unique code e.g. "mts-ru"
	SupportsPaidSender  bool
	SupportsFreeSender  bool
	MonthlyTariffAmount *string // ежемесячный тариф в RUB; nil = free-only оператор
	Active              bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// HasPaidRegistration возвращает true если оператор поддерживает платную регистрацию
func (o *Operator) HasPaidRegistration() bool {
	return o.SupportsPaidSender && o.MonthlyTariffAmount != nil && *o.MonthlyTariffAmount != "" && *o.MonthlyTariffAmount != "0"
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

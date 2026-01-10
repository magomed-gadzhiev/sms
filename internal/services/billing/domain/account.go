package domain

import (
	"time"

	"github.com/google/uuid"
)

// Account представляет доменную модель счета клиента
type Account struct {
	ID        uuid.UUID
	ClientID  uuid.UUID
	Balance   string // Используем строку для точности финансовых расчетов
	Currency  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewAccount создает новый счет для клиента
func NewAccount(clientID uuid.UUID, currency string) *Account {
	now := time.Now()
	return &Account{
		ID:        uuid.New(),
		ClientID:  clientID,
		Balance:   "0",
		Currency:  currency,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate валидирует счет
func (a *Account) Validate() error {
	if a.ClientID == uuid.Nil {
		return ErrAccountClientIDRequired
	}
	if a.Currency == "" {
		return ErrAccountCurrencyRequired
	}
	return nil
}

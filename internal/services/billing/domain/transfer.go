package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	TransactionTypeTransferOut TransactionType = "transfer_out" // Перевод со счёта
	TransactionTypeTransferIn  TransactionType = "transfer_in"  // Перевод на счёт
)

// BalanceTransfer представляет перевод средств между клиентами
type BalanceTransfer struct {
	ID                uuid.UUID
	FromClientID      uuid.UUID
	ToClientID        uuid.UUID
	Amount            string
	Currency          string
	FromTransactionID uuid.UUID
	ToTransactionID   uuid.UUID
	CreatedAt         time.Time
}

// NewBalanceTransfer создает новый перевод средств
func NewBalanceTransfer(
	fromClientID, toClientID uuid.UUID,
	amount, currency string,
	fromTransactionID, toTransactionID uuid.UUID,
) *BalanceTransfer {
	return &BalanceTransfer{
		ID:                uuid.New(),
		FromClientID:      fromClientID,
		ToClientID:        toClientID,
		Amount:            amount,
		Currency:          currency,
		FromTransactionID: fromTransactionID,
		ToTransactionID:   toTransactionID,
		CreatedAt:         time.Now(),
	}
}

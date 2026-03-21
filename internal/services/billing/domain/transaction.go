package domain

import (
	"time"

	"github.com/google/uuid"
)

// TransactionType представляет тип транзакции
type TransactionType string

const (
	TransactionTypeCharge     TransactionType = "charge"     // Списание за сообщение
	TransactionTypeCredit     TransactionType = "credit"     // Пополнение баланса
	TransactionTypeRefund     TransactionType = "refund"     // Возврат средств
	TransactionTypeAdjustment TransactionType = "adjustment" // Корректировка баланса
)

// Transaction представляет доменную модель транзакции
type Transaction struct {
	ID           uuid.UUID
	ClientID     uuid.UUID
	Type         TransactionType
	Amount       string // Используем строку для точности
	Currency     string
	BalanceBefore string
	BalanceAfter  string
	Description   string
	MessageID     *uuid.UUID
	PaymentMethod *string
	Metadata      map[string]interface{}
	CreatedAt     time.Time
}

// NewTransaction создает новую транзакцию
func NewTransaction(
	clientID uuid.UUID,
	transactionType TransactionType,
	amount, balanceBefore, balanceAfter string,
	currency string,
) *Transaction {
	return &Transaction{
		ID:            uuid.New(),
		ClientID:      clientID,
		Type:          transactionType,
		Amount:        amount,
		Currency:      currency,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Metadata:      make(map[string]interface{}),
		CreatedAt:     time.Now(),
	}
}

// WithMessageID связывает транзакцию с сообщением
func (t *Transaction) WithMessageID(messageID uuid.UUID) *Transaction {
	t.MessageID = &messageID
	return t
}

// WithDescription добавляет описание транзакции
func (t *Transaction) WithDescription(description string) *Transaction {
	t.Description = description
	return t
}

// WithPaymentMethod добавляет способ оплаты
func (t *Transaction) WithPaymentMethod(method string) *Transaction {
	t.PaymentMethod = &method
	return t
}

// Validate валидирует транзакцию
func (t *Transaction) Validate() error {
	if t.ClientID == uuid.Nil {
		return ErrTransactionClientIDRequired
	}
	if !t.IsValidType() {
		return ErrTransactionInvalidType
	}
	if t.Amount == "" {
		return ErrTransactionAmountRequired
	}
	if t.Currency == "" {
		return ErrTransactionCurrencyRequired
	}
	return nil
}

// IsValidType проверяет валидность типа транзакции
func (t *Transaction) IsValidType() bool {
	return t.Type == TransactionTypeCharge ||
		t.Type == TransactionTypeCredit ||
		t.Type == TransactionTypeRefund ||
		t.Type == TransactionTypeAdjustment ||
		t.Type == TransactionTypeTransferOut ||
		t.Type == TransactionTypeTransferIn
}

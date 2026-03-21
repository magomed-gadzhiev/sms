package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTransaction(t *testing.T) {
	t.Run("creates transaction with correct fields", func(t *testing.T) {
		clientID := uuid.New()
		tx := NewTransaction(clientID, TransactionTypeCharge, "1.50", "100.00", "98.50", "RUB")

		assert.NotEqual(t, uuid.Nil, tx.ID)
		assert.Equal(t, clientID, tx.ClientID)
		assert.Equal(t, TransactionTypeCharge, tx.Type)
		assert.Equal(t, "1.50", tx.Amount)
		assert.Equal(t, "RUB", tx.Currency)
		assert.Equal(t, "100.00", tx.BalanceBefore)
		assert.Equal(t, "98.50", tx.BalanceAfter)
		require.NotNil(t, tx.Metadata)
		assert.Empty(t, tx.Metadata)
		assert.False(t, tx.CreatedAt.IsZero())
	})
}

func TestTransaction_WithMessageID(t *testing.T) {
	t.Run("sets message ID and returns self for chaining", func(t *testing.T) {
		tx := NewTransaction(uuid.New(), TransactionTypeCharge, "1.00", "10.00", "9.00", "RUB")
		msgID := uuid.New()

		result := tx.WithMessageID(msgID)

		require.NotNil(t, tx.MessageID)
		assert.Equal(t, msgID, *tx.MessageID)
		assert.Same(t, tx, result) // fluent interface returns same pointer
	})
}

func TestTransaction_WithDescription(t *testing.T) {
	t.Run("sets description and returns self for chaining", func(t *testing.T) {
		tx := NewTransaction(uuid.New(), TransactionTypeCredit, "100.00", "0.00", "100.00", "RUB")

		result := tx.WithDescription("Top-up via bank transfer")

		assert.Equal(t, "Top-up via bank transfer", tx.Description)
		assert.Same(t, tx, result)
	})
}

func TestTransaction_WithPaymentMethod(t *testing.T) {
	t.Run("sets payment method and returns self for chaining", func(t *testing.T) {
		tx := NewTransaction(uuid.New(), TransactionTypeCredit, "100.00", "0.00", "100.00", "RUB")

		result := tx.WithPaymentMethod("bank_transfer")

		require.NotNil(t, tx.PaymentMethod)
		assert.Equal(t, "bank_transfer", *tx.PaymentMethod)
		assert.Same(t, tx, result)
	})
}

func TestTransaction_Validate(t *testing.T) {
	t.Run("valid transaction returns nil", func(t *testing.T) {
		tx := NewTransaction(uuid.New(), TransactionTypeCharge, "1.00", "10.00", "9.00", "RUB")
		err := tx.Validate()
		assert.NoError(t, err)
	})

	t.Run("nil ClientID returns error", func(t *testing.T) {
		tx := NewTransaction(uuid.Nil, TransactionTypeCharge, "1.00", "10.00", "9.00", "RUB")
		err := tx.Validate()
		assert.ErrorIs(t, err, ErrTransactionClientIDRequired)
	})

	t.Run("invalid type returns error", func(t *testing.T) {
		tx := NewTransaction(uuid.New(), TransactionType("invalid"), "1.00", "10.00", "9.00", "RUB")
		err := tx.Validate()
		assert.ErrorIs(t, err, ErrTransactionInvalidType)
	})

	t.Run("empty amount returns error", func(t *testing.T) {
		tx := NewTransaction(uuid.New(), TransactionTypeCharge, "", "10.00", "9.00", "RUB")
		err := tx.Validate()
		assert.ErrorIs(t, err, ErrTransactionAmountRequired)
	})

	t.Run("empty currency returns error", func(t *testing.T) {
		tx := NewTransaction(uuid.New(), TransactionTypeCharge, "1.00", "10.00", "9.00", "")
		err := tx.Validate()
		assert.ErrorIs(t, err, ErrTransactionCurrencyRequired)
	})
}

func TestTransaction_IsValidType(t *testing.T) {
	t.Run("charge is valid", func(t *testing.T) {
		tx := &Transaction{Type: TransactionTypeCharge}
		assert.True(t, tx.IsValidType())
	})

	t.Run("credit is valid", func(t *testing.T) {
		tx := &Transaction{Type: TransactionTypeCredit}
		assert.True(t, tx.IsValidType())
	})

	t.Run("refund is valid", func(t *testing.T) {
		tx := &Transaction{Type: TransactionTypeRefund}
		assert.True(t, tx.IsValidType())
	})

	t.Run("adjustment is valid", func(t *testing.T) {
		tx := &Transaction{Type: TransactionTypeAdjustment}
		assert.True(t, tx.IsValidType())
	})

	t.Run("transfer_out is valid", func(t *testing.T) {
		tx := &Transaction{Type: TransactionTypeTransferOut}
		assert.True(t, tx.IsValidType())
	})

	t.Run("transfer_in is valid", func(t *testing.T) {
		tx := &Transaction{Type: TransactionTypeTransferIn}
		assert.True(t, tx.IsValidType())
	})

	t.Run("unknown type is invalid", func(t *testing.T) {
		tx := &Transaction{Type: TransactionType("unknown")}
		assert.False(t, tx.IsValidType())
	})

	t.Run("empty type is invalid", func(t *testing.T) {
		tx := &Transaction{Type: TransactionType("")}
		assert.False(t, tx.IsValidType())
	})
}

func TestTransaction_FluentChaining(t *testing.T) {
	t.Run("chain all With methods", func(t *testing.T) {
		msgID := uuid.New()
		tx := NewTransaction(uuid.New(), TransactionTypeCharge, "1.50", "100.00", "98.50", "RUB").
			WithMessageID(msgID).
			WithDescription("SMS charge").
			WithPaymentMethod("balance")

		assert.Equal(t, &msgID, tx.MessageID)
		assert.Equal(t, "SMS charge", tx.Description)
		require.NotNil(t, tx.PaymentMethod)
		assert.Equal(t, "balance", *tx.PaymentMethod)
	})
}

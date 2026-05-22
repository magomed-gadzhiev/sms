package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewAccount(t *testing.T) {
	t.Run("creates account with zero balance", func(t *testing.T) {
		clientID := uuid.New()
		account := NewAccount(clientID, "RUB")

		assert.NotEqual(t, uuid.Nil, account.ID)
		assert.Equal(t, clientID, account.ClientID)
		assert.Equal(t, "0", account.Balance)
		assert.Equal(t, "RUB", account.Currency)
		assert.False(t, account.CreatedAt.IsZero())
		assert.False(t, account.UpdatedAt.IsZero())
	})
}

func TestAccount_Validate(t *testing.T) {
	t.Run("valid account returns nil", func(t *testing.T) {
		account := NewAccount(uuid.New(), "RUB")
		err := account.Validate()
		assert.NoError(t, err)
	})

	t.Run("nil ClientID returns error", func(t *testing.T) {
		account := NewAccount(uuid.Nil, "RUB")
		err := account.Validate()
		assert.ErrorIs(t, err, ErrAccountClientIDRequired)
	})

	t.Run("empty currency returns error", func(t *testing.T) {
		account := NewAccount(uuid.New(), "")
		err := account.Validate()
		assert.ErrorIs(t, err, ErrAccountCurrencyRequired)
	})

	t.Run("nil ClientID is checked before empty currency", func(t *testing.T) {
		account := NewAccount(uuid.Nil, "")
		err := account.Validate()
		// ClientID check comes first in the method
		assert.ErrorIs(t, err, ErrAccountClientIDRequired)
	})
}

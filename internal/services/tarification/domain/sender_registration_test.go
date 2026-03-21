package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewSenderRegistration(t *testing.T) {
	t.Run("creates registration with pending status", func(t *testing.T) {
		clientID := uuid.New()
		operatorID := uuid.New()

		reg := NewSenderRegistration(clientID, operatorID, "MySender", SenderTypePaid)

		assert.NotEqual(t, uuid.Nil, reg.ID)
		assert.Equal(t, clientID, reg.ClientID)
		assert.Equal(t, operatorID, reg.OperatorID)
		assert.Equal(t, "MySender", reg.SenderName)
		assert.Equal(t, SenderTypePaid, reg.Type)
		assert.Equal(t, SenderStatusPending, reg.Status)
		assert.False(t, reg.CreatedAt.IsZero())
		assert.False(t, reg.UpdatedAt.IsZero())
	})
}

func TestSenderRegistration_Validate(t *testing.T) {
	t.Run("valid registration returns nil", func(t *testing.T) {
		reg := NewSenderRegistration(uuid.New(), uuid.New(), "MySender", SenderTypePaid)
		err := reg.Validate()
		assert.NoError(t, err)
	})

	t.Run("nil ClientID returns error", func(t *testing.T) {
		reg := NewSenderRegistration(uuid.Nil, uuid.New(), "MySender", SenderTypePaid)
		err := reg.Validate()
		assert.ErrorIs(t, err, ErrSenderRegistrationNotFound)
	})

	t.Run("nil OperatorID returns error", func(t *testing.T) {
		reg := NewSenderRegistration(uuid.New(), uuid.Nil, "MySender", SenderTypePaid)
		err := reg.Validate()
		assert.ErrorIs(t, err, ErrSenderRegistrationNotFound)
	})

	t.Run("empty SenderName returns error", func(t *testing.T) {
		reg := NewSenderRegistration(uuid.New(), uuid.New(), "", SenderTypePaid)
		err := reg.Validate()
		assert.ErrorIs(t, err, ErrSenderRegistrationNotFound)
	})

	t.Run("paid type is valid", func(t *testing.T) {
		reg := NewSenderRegistration(uuid.New(), uuid.New(), "S", SenderTypePaid)
		err := reg.Validate()
		assert.NoError(t, err)
	})

	t.Run("free type is valid", func(t *testing.T) {
		reg := NewSenderRegistration(uuid.New(), uuid.New(), "S", SenderTypeFree)
		err := reg.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid type returns error", func(t *testing.T) {
		reg := NewSenderRegistration(uuid.New(), uuid.New(), "S", SenderRegistrationType("trial"))
		err := reg.Validate()
		assert.ErrorIs(t, err, ErrSenderRegistrationInvalidType)
	})
}

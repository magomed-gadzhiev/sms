package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestOperatorBinding_StatusTransitions(t *testing.T) {
	assert.True(t, IsValidBindingTransition(BindingStatusPending, BindingStatusApproved))
	assert.True(t, IsValidBindingTransition(BindingStatusPending, BindingStatusRejected))
	assert.False(t, IsValidBindingTransition(BindingStatusApproved, BindingStatusPending))
	assert.False(t, IsValidBindingTransition(BindingStatusRejected, BindingStatusApproved))
	assert.False(t, IsValidBindingTransition("", BindingStatusApproved))
	assert.True(t, IsValidBindingTransition("", BindingStatusPending))
	// rejected can re-enter pending on resubmit
	assert.True(t, IsValidBindingTransition(BindingStatusRejected, BindingStatusPending))
}

func TestNewOperatorBinding_SetsDefaults(t *testing.T) {
	tmplID := uuid.New()
	snID := uuid.New()
	opID := uuid.New()
	b := NewOperatorBinding(tmplID, snID, opID)
	assert.Equal(t, tmplID, b.TemplateID)
	assert.Equal(t, snID, b.SenderNameID)
	assert.Equal(t, opID, b.OperatorID)
	assert.Equal(t, BindingStatusPending, b.Status)
	assert.NotEqual(t, uuid.Nil, b.ID)
	assert.False(t, b.CreatedAt.IsZero())
	assert.False(t, b.UpdatedAt.IsZero())
}

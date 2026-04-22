package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestOperatorBinding_StatusTransitions(t *testing.T) {
	assert.True(t, IsValidOperatorBindingTransition(OperatorBindingStatusPending, OperatorBindingStatusApproved))
	assert.True(t, IsValidOperatorBindingTransition(OperatorBindingStatusPending, OperatorBindingStatusRejected))
	assert.False(t, IsValidOperatorBindingTransition(OperatorBindingStatusApproved, OperatorBindingStatusPending))
	assert.False(t, IsValidOperatorBindingTransition(OperatorBindingStatusRejected, OperatorBindingStatusApproved))
	assert.False(t, IsValidOperatorBindingTransition("", OperatorBindingStatusApproved))
	assert.True(t, IsValidOperatorBindingTransition("", OperatorBindingStatusPending))
	// rejected can re-enter pending on resubmit
	assert.True(t, IsValidOperatorBindingTransition(OperatorBindingStatusRejected, OperatorBindingStatusPending))
}

func TestNewOperatorBinding_SetsDefaults(t *testing.T) {
	tmplID := uuid.New()
	snID := uuid.New()
	opID := uuid.New()
	b := NewOperatorTemplateBinding(tmplID, snID, opID)
	assert.Equal(t, tmplID, b.TemplateID)
	assert.Equal(t, snID, b.SenderNameID)
	assert.Equal(t, opID, b.OperatorID)
	assert.Equal(t, OperatorBindingStatusPending, b.Status)
	assert.NotEqual(t, uuid.Nil, b.ID)
	assert.False(t, b.CreatedAt.IsZero())
	assert.False(t, b.UpdatedAt.IsZero())
}

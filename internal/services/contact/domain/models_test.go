package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Error constants ---

func TestContactErrors_NonNil(t *testing.T) {
	errs := []error{
		ErrContactListNotFound,
		ErrContactNotFound,
		ErrImportNotFound,
		ErrDuplicatePhone,
		ErrInvalidPhone,
		ErrInvalidSegmentRules,
		ErrSegmentDepthExceeded,
		ErrImportAlreadyStarted,
	}
	for _, e := range errs {
		assert.NotNil(t, e)
	}
}

func TestContactErrors_AllDistinct(t *testing.T) {
	errs := []error{
		ErrContactListNotFound,
		ErrContactNotFound,
		ErrImportNotFound,
		ErrDuplicatePhone,
		ErrInvalidPhone,
		ErrInvalidSegmentRules,
		ErrSegmentDepthExceeded,
		ErrImportAlreadyStarted,
	}
	seen := make(map[error]bool)
	for _, e := range errs {
		assert.False(t, seen[e], "duplicate error sentinel: %v", e)
		seen[e] = true
	}
}

// --- Import status constants ---

func TestImportStatusConstants(t *testing.T) {
	assert.Equal(t, "pending", ImportStatusPending)
	assert.Equal(t, "processing", ImportStatusProcessing)
	assert.Equal(t, "completed", ImportStatusCompleted)
	assert.Equal(t, "failed", ImportStatusFailed)
}

func TestImportStatusConstants_AllDistinct(t *testing.T) {
	statuses := []string{
		ImportStatusPending,
		ImportStatusProcessing,
		ImportStatusCompleted,
		ImportStatusFailed,
	}
	seen := make(map[string]bool)
	for _, s := range statuses {
		assert.False(t, seen[s], "duplicate import status: %s", s)
		seen[s] = true
	}
}

// --- Attribute type constants ---

func TestAttrTypeConstants(t *testing.T) {
	assert.Equal(t, "string", AttrTypeString)
	assert.Equal(t, "number", AttrTypeNumber)
	assert.Equal(t, "date", AttrTypeDate)
	assert.Equal(t, "boolean", AttrTypeBoolean)
}

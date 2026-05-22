package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- SMPPStatToEventType ---

func TestSMPPStatToEventType_DELIVRD(t *testing.T) {
	assert.Equal(t, "delivered", SMPPStatToEventType["DELIVRD"])
}

func TestSMPPStatToEventType_UNDELIV(t *testing.T) {
	assert.Equal(t, "failed", SMPPStatToEventType["UNDELIV"])
}

func TestSMPPStatToEventType_EXPIRED(t *testing.T) {
	assert.Equal(t, "expired", SMPPStatToEventType["EXPIRED"])
}

func TestSMPPStatToEventType_DELETED(t *testing.T) {
	assert.Equal(t, "failed", SMPPStatToEventType["DELETED"])
}

func TestSMPPStatToEventType_REJECTD(t *testing.T) {
	assert.Equal(t, "rejected", SMPPStatToEventType["REJECTD"])
}

func TestSMPPStatToEventType_UNKNOWN(t *testing.T) {
	assert.Equal(t, "failed", SMPPStatToEventType["UNKNOWN"])
}

func TestSMPPStatToEventType_MissingKey(t *testing.T) {
	val, ok := SMPPStatToEventType["NOSUCHSTAT"]
	assert.False(t, ok, "expected missing key to be absent")
	assert.Empty(t, val)
}

func TestSMPPStatToEventType_AllEntriesCount(t *testing.T) {
	// Verify the exact set of known entries.
	expected := map[string]string{
		"DELIVRD": "delivered",
		"UNDELIV": "failed",
		"EXPIRED": "expired",
		"DELETED": "failed",
		"REJECTD": "rejected",
		"UNKNOWN": "failed",
	}
	assert.Equal(t, expected, SMPPStatToEventType)
}

// --- ValidEventTypes ---

func TestValidEventTypes_Delivered(t *testing.T) {
	assert.True(t, ValidEventTypes["delivered"])
}

func TestValidEventTypes_Failed(t *testing.T) {
	assert.True(t, ValidEventTypes["failed"])
}

func TestValidEventTypes_Expired(t *testing.T) {
	assert.True(t, ValidEventTypes["expired"])
}

func TestValidEventTypes_Rejected(t *testing.T) {
	assert.True(t, ValidEventTypes["rejected"])
}

func TestValidEventTypes_InvalidEntry(t *testing.T) {
	assert.False(t, ValidEventTypes["sent"])
	assert.False(t, ValidEventTypes["DELIVERED"])
	assert.False(t, ValidEventTypes[""])
}

// --- MaxSubscriptionsPerClient ---

func TestMaxSubscriptionsPerClient(t *testing.T) {
	assert.Equal(t, 10, MaxSubscriptionsPerClient)
}

// --- Sentinel errors are non-nil ---

func TestErrors_NonNil(t *testing.T) {
	errs := []error{
		ErrSubscriptionNotFound,
		ErrMaxSubscriptionsReached,
		ErrInvalidURL,
		ErrInvalidEventType,
	}
	for _, e := range errs {
		assert.NotNil(t, e)
	}
}

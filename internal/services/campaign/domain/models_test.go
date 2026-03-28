package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Campaign status constants ---

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, "draft", StatusDraft)
	assert.Equal(t, "scheduled", StatusScheduled)
	assert.Equal(t, "materializing", StatusMaterializing)
	assert.Equal(t, "running", StatusRunning)
	assert.Equal(t, "paused", StatusPaused)
	assert.Equal(t, "completed", StatusCompleted)
	assert.Equal(t, "cancelled", StatusCancelled)
}

func TestStatusConstants_AllDistinct(t *testing.T) {
	statuses := []string{
		StatusDraft,
		StatusScheduled,
		StatusMaterializing,
		StatusRunning,
		StatusPaused,
		StatusCompleted,
		StatusCancelled,
	}
	seen := make(map[string]bool)
	for _, s := range statuses {
		assert.False(t, seen[s], "duplicate status constant: %s", s)
		seen[s] = true
	}
}

// --- Recipient status constants ---

func TestRecipientStatusConstants(t *testing.T) {
	assert.Equal(t, "pending", RecipientPending)
	assert.Equal(t, "sent", RecipientSent)
	assert.Equal(t, "delivered", RecipientDelivered)
	assert.Equal(t, "failed", RecipientFailed)
	assert.Equal(t, "retry", RecipientRetry)
	assert.Equal(t, "cancelled", RecipientCancelled)
	assert.Equal(t, "capped", RecipientCapped)
	assert.Equal(t, "pending_rollout", RecipientPendingRollout)
	assert.Equal(t, "skipped_quiet_hours", RecipientSkippedQuietHours)
}

// --- Test phase constants ---

func TestTestPhaseConstants(t *testing.T) {
	assert.Equal(t, "none", TestPhaseNone)
	assert.Equal(t, "testing", TestPhaseTesting)
	assert.Equal(t, "waiting_winner", TestPhaseWaitingWinner)
	assert.Equal(t, "rollout", TestPhaseRollout)
	assert.Equal(t, "completed", TestPhaseCompleted)
}

// --- Strategy constants ---

func TestStrategyConstants(t *testing.T) {
	assert.Equal(t, "full_split", StrategyFullSplit)
	assert.Equal(t, "test_then_send", StrategyTestThenSend)
}

// --- Sentinel errors ---

func TestSentinelErrors_NonNil(t *testing.T) {
	errs := []error{
		ErrCampaignNotFound,
		ErrVariantNotFound,
		ErrInvalidCampaignStatus,
		ErrCampaignNotDraft,
		ErrCampaignNotRunning,
		ErrCampaignNotPaused,
		ErrVariantPercentageSum,
		ErrTooFewVariants,
		ErrTooManyVariants,
		ErrWinnerAlreadySelected,
		ErrNoFailedRecipients,
	}
	for _, e := range errs {
		assert.NotNil(t, e)
	}
}

func TestSentinelErrors_AllDistinct(t *testing.T) {
	errs := []error{
		ErrCampaignNotFound,
		ErrVariantNotFound,
		ErrInvalidCampaignStatus,
		ErrCampaignNotDraft,
		ErrCampaignNotRunning,
		ErrCampaignNotPaused,
		ErrVariantPercentageSum,
		ErrTooFewVariants,
		ErrTooManyVariants,
		ErrWinnerAlreadySelected,
		ErrNoFailedRecipients,
	}
	seen := make(map[error]bool)
	for _, e := range errs {
		assert.False(t, seen[e], "duplicate error sentinel: %v", e)
		seen[e] = true
	}
}

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapToRecipientStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"sent", "sent"},
		{"accepted", "sent"},
		{"delivered", "delivered"},
		{"failed", "failed"},
		{"rejected", "failed"},
		{"expired", "failed"},
		{"undeliverable", "failed"},
		{"unknown", ""},
		{"queued", ""},
		{"", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, mapToRecipientStatus(tt.input))
		})
	}
}

func TestStatusToCounterColumn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"sent", "sent_count"},
		{"delivered", "delivered_count"},
		{"failed", "failed_count"},
		{"pending", ""},
		{"queued", ""},
		{"", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, statusToCounterColumn(tt.input))
		})
	}
}

func TestIsCampaignComplete_AllDelivered(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"delivered": 85,
	}
	assert.True(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_AllFailed(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"failed": 10,
	}
	assert.True(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_MixedFinalStatuses(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"delivered": 70,
		"failed":    15,
	}
	assert.True(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_HasPending(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"pending":   5,
		"delivered": 80,
	}
	assert.False(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_HasSent(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"sent":      3,
		"delivered": 82,
	}
	assert.False(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_AllPending(t *testing.T) {
	t.Parallel()
	counts := map[string]int{
		"pending": 85,
	}
	assert.False(t, isCampaignComplete(counts))
}

func TestIsCampaignComplete_Empty(t *testing.T) {
	t.Parallel()
	assert.True(t, isCampaignComplete(map[string]int{}))
}

// TestCompletionCondition_PendingBlocksCompletion verifies that a single
// pending recipient prevents campaign from being marked complete —
// mirroring the SQL WHERE status IN ('pending','sent') check in processStatusMessage.
func TestCompletionCondition_PendingBlocksCompletion(t *testing.T) {
	t.Parallel()

	// Scenario: 84 delivered, 1 still pending → not complete
	counts := map[string]int{"delivered": 84, "pending": 1}
	assert.False(t, isCampaignComplete(counts),
		"campaign must not complete while a recipient is still pending")
}

// TestCompletionCondition_SentBlocksCompletion verifies that a recipient in
// 'sent' state (awaiting DLR) also blocks completion.
func TestCompletionCondition_SentBlocksCompletion(t *testing.T) {
	t.Parallel()

	counts := map[string]int{"delivered": 84, "sent": 1}
	assert.False(t, isCampaignComplete(counts),
		"campaign must not complete while a recipient is awaiting DLR (sent)")
}

// TestCompletionCondition_LastRecipientDelivered verifies that transitioning
// the last pending recipient to delivered triggers completion.
func TestCompletionCondition_LastRecipientDelivered(t *testing.T) {
	t.Parallel()

	before := map[string]int{"delivered": 84, "pending": 1}
	assert.False(t, isCampaignComplete(before))

	after := map[string]int{"delivered": 85}
	assert.True(t, isCampaignComplete(after),
		"campaign must complete once all recipients reach terminal status")
}

// TestCompletionCondition_LastRecipientFailed verifies that a final 'failed'
// recipient also completes the campaign (failure is terminal).
func TestCompletionCondition_LastRecipientFailed(t *testing.T) {
	t.Parallel()

	counts := map[string]int{"delivered": 80, "failed": 5}
	assert.True(t, isCampaignComplete(counts),
		"failed is a terminal status — campaign should complete")
}

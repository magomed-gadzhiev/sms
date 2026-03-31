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

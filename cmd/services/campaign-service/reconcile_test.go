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

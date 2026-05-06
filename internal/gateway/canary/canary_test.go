package canary_test

import (
	"testing"

	"github.com/smpp-server/smpp-server/internal/gateway/canary"
)

func TestIsAllowed(t *testing.T) {
	cases := []struct {
		name     string
		envValue string
		clientID string
		want     bool
	}{
		{"empty env → all allowed", "", "any-id", true},
		{"single match", "abc-123", "abc-123", true},
		{"multi match middle", "abc-123,def-456,ghi-789", "def-456", true},
		{"multi match last", "abc-123,def-456", "def-456", true},
		{"multi reject", "abc-123,def-456", "xyz-999", false},
		{"whitespace tolerance", " abc-123 , def-456 ", "abc-123", true},
		{"single non-match", "abc-123", "xyz-999", false},
		{"empty client_id with non-empty env", "abc-123", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(canary.EnvVar, tc.envValue)
			got := canary.IsAllowed(tc.clientID)
			if got != tc.want {
				t.Errorf("IsAllowed(%q) с env=%q = %v, want %v", tc.clientID, tc.envValue, got, tc.want)
			}
		})
	}
}

package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestAPIKey_IsExpired(t *testing.T) {
	t.Run("returns false when ExpiresAt is nil (no expiry)", func(t *testing.T) {
		key := &APIKey{ID: uuid.New(), ExpiresAt: nil}
		assert.False(t, key.IsExpired())
	})

	t.Run("returns false when ExpiresAt is in the future", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		key := &APIKey{ID: uuid.New(), ExpiresAt: &future}
		assert.False(t, key.IsExpired())
	})

	t.Run("returns true when ExpiresAt is in the past", func(t *testing.T) {
		past := time.Now().Add(-24 * time.Hour)
		key := &APIKey{ID: uuid.New(), ExpiresAt: &past}
		assert.True(t, key.IsExpired())
	})
}

func TestAPIKey_IsValid(t *testing.T) {
	t.Run("returns true when active and not expired", func(t *testing.T) {
		key := &APIKey{ID: uuid.New(), Active: true, ExpiresAt: nil}
		assert.True(t, key.IsValid())
	})

	t.Run("returns false when inactive", func(t *testing.T) {
		key := &APIKey{ID: uuid.New(), Active: false, ExpiresAt: nil}
		assert.False(t, key.IsValid())
	})

	t.Run("returns false when expired", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		key := &APIKey{ID: uuid.New(), Active: true, ExpiresAt: &past}
		assert.False(t, key.IsValid())
	})

	t.Run("returns false when both inactive and expired", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		key := &APIKey{ID: uuid.New(), Active: false, ExpiresAt: &past}
		assert.False(t, key.IsValid())
	})
}

func TestAPIKey_HasScope(t *testing.T) {
	t.Run("returns true when scopes are nil (full access)", func(t *testing.T) {
		key := &APIKey{ID: uuid.New(), Scopes: nil}
		assert.True(t, key.HasScope("messages:send"))
	})

	t.Run("returns true when scopes are empty (full access)", func(t *testing.T) {
		key := &APIKey{ID: uuid.New(), Scopes: []string{}}
		assert.True(t, key.HasScope("messages:send"))
	})

	t.Run("returns true when scope is present", func(t *testing.T) {
		key := &APIKey{
			ID:     uuid.New(),
			Scopes: []string{"messages:send", "messages:read", "billing:read"},
		}
		assert.True(t, key.HasScope("messages:send"))
	})

	t.Run("returns false when scope is not present", func(t *testing.T) {
		key := &APIKey{
			ID:     uuid.New(),
			Scopes: []string{"messages:send", "messages:read"},
		}
		assert.False(t, key.HasScope("billing:write"))
	})
}

func TestAPIKey_IsIPAllowed(t *testing.T) {
	t.Run("returns true when AllowedIPs is empty (no restriction)", func(t *testing.T) {
		key := &APIKey{ID: uuid.New(), AllowedIPs: nil}
		assert.True(t, key.IsIPAllowed("192.168.1.1"))
	})

	t.Run("returns true for exact IP match", func(t *testing.T) {
		key := &APIKey{
			ID:         uuid.New(),
			AllowedIPs: []string{"10.0.0.1", "192.168.1.100"},
		}
		assert.True(t, key.IsIPAllowed("192.168.1.100"))
	})

	t.Run("returns false for non-matching IP", func(t *testing.T) {
		key := &APIKey{
			ID:         uuid.New(),
			AllowedIPs: []string{"10.0.0.1", "192.168.1.100"},
		}
		assert.False(t, key.IsIPAllowed("172.16.0.1"))
	})

	t.Run("returns true for IP within CIDR range", func(t *testing.T) {
		key := &APIKey{
			ID:         uuid.New(),
			AllowedIPs: []string{"10.0.0.0/24"},
		}
		assert.True(t, key.IsIPAllowed("10.0.0.55"))
	})

	t.Run("returns false for IP outside CIDR range", func(t *testing.T) {
		key := &APIKey{
			ID:         uuid.New(),
			AllowedIPs: []string{"10.0.0.0/24"},
		}
		assert.False(t, key.IsIPAllowed("10.0.1.1"))
	})

	t.Run("returns false for invalid IP string", func(t *testing.T) {
		key := &APIKey{
			ID:         uuid.New(),
			AllowedIPs: []string{"10.0.0.0/24"},
		}
		assert.False(t, key.IsIPAllowed("not-an-ip"))
	})

	t.Run("supports mixed exact IPs and CIDR ranges", func(t *testing.T) {
		key := &APIKey{
			ID:         uuid.New(),
			AllowedIPs: []string{"192.168.1.1", "10.0.0.0/16"},
		}
		assert.True(t, key.IsIPAllowed("192.168.1.1"))
		assert.True(t, key.IsIPAllowed("10.0.5.10"))
		assert.False(t, key.IsIPAllowed("172.16.0.1"))
	})
}

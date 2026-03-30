package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		active   bool
		expected bool
	}{
		{"active client", true, true},
		{"inactive client", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{Active: tt.active}
			assert.Equal(t, tt.expected, c.IsActive())
		})
	}
}

func TestClient_IsSubAccount(t *testing.T) {
	t.Run("is sub-account when parent set", func(t *testing.T) {
		parentID := uuid.New()
		c := &Client{ParentClientID: &parentID}
		assert.True(t, c.IsSubAccount())
	})

	t.Run("is not sub-account when parent nil", func(t *testing.T) {
		c := &Client{}
		assert.False(t, c.IsSubAccount())
	})
}

func TestClient_CanCreateSubAccount(t *testing.T) {
	tests := []struct {
		name         string
		isReseller   bool
		maxSub       int
		currentCount int
		expected     bool
	}{
		{"reseller with capacity", true, 5, 3, true},
		{"reseller at limit", true, 5, 5, false},
		{"reseller over limit", true, 5, 6, false},
		{"not reseller", false, 5, 0, false},
		{"reseller zero max", true, 0, 0, false},
		{"reseller zero current", true, 10, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{IsReseller: tt.isReseller, MaxSubAccounts: tt.maxSub}
			assert.Equal(t, tt.expected, c.CanCreateSubAccount(tt.currentCount))
		})
	}
}

func TestClient_GetMetadata(t *testing.T) {
	t.Run("nil metadata returns empty map", func(t *testing.T) {
		c := &Client{Metadata: nil}
		m := c.GetMetadata()
		assert.NotNil(t, m)
		assert.Empty(t, m)
	})

	t.Run("empty metadata returns empty map", func(t *testing.T) {
		c := &Client{Metadata: json.RawMessage{}}
		m := c.GetMetadata()
		assert.NotNil(t, m)
		assert.Empty(t, m)
	})

	t.Run("valid JSON metadata returns map", func(t *testing.T) {
		c := &Client{Metadata: json.RawMessage(`{"key":"value","foo":"bar"}`)}
		m := c.GetMetadata()
		assert.Equal(t, "value", m["key"])
		assert.Equal(t, "bar", m["foo"])
		assert.Len(t, m, 2)
	})

	t.Run("invalid JSON returns empty map", func(t *testing.T) {
		c := &Client{Metadata: json.RawMessage(`{invalid`)}
		m := c.GetMetadata()
		assert.NotNil(t, m)
		assert.Empty(t, m)
	})

	t.Run("non-string-value JSON returns empty map", func(t *testing.T) {
		c := &Client{Metadata: json.RawMessage(`{"key": 123}`)}
		m := c.GetMetadata()
		assert.NotNil(t, m)
		assert.Empty(t, m) // json.Unmarshal into map[string]string will fail
	})
}

func TestClient_SetMetadata(t *testing.T) {
	t.Run("nil metadata sets empty object", func(t *testing.T) {
		c := &Client{}
		err := c.SetMetadata(nil)
		require.NoError(t, err)
		assert.Equal(t, json.RawMessage("{}"), c.Metadata)
	})

	t.Run("empty map sets empty object", func(t *testing.T) {
		c := &Client{}
		err := c.SetMetadata(map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, json.RawMessage("{}"), c.Metadata)
	})

	t.Run("valid map sets JSON", func(t *testing.T) {
		c := &Client{}
		err := c.SetMetadata(map[string]string{"key": "value"})
		require.NoError(t, err)

		var result map[string]string
		err = json.Unmarshal(c.Metadata, &result)
		require.NoError(t, err)
		assert.Equal(t, "value", result["key"])
	})

	t.Run("roundtrip set then get", func(t *testing.T) {
		c := &Client{}
		original := map[string]string{"a": "1", "b": "2"}
		err := c.SetMetadata(original)
		require.NoError(t, err)

		got := c.GetMetadata()
		assert.Equal(t, original, got)
	})
}

func TestClient_IsWithinMonthlyQuota(t *testing.T) {
	t.Run("nil plan always within quota", func(t *testing.T) {
		c := &Client{Plan: nil, MonthlySMSCount: 999999}
		assert.True(t, c.IsWithinMonthlyQuota(100))
	})

	t.Run("within quota", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 1000},
			MonthlySMSCount: 900,
		}
		assert.True(t, c.IsWithinMonthlyQuota(100))
	})

	t.Run("exactly at quota", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 1000},
			MonthlySMSCount: 900,
		}
		assert.True(t, c.IsWithinMonthlyQuota(100)) // 900+100 == 1000
	})

	t.Run("over quota", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 1000},
			MonthlySMSCount: 900,
		}
		assert.False(t, c.IsWithinMonthlyQuota(101))
	})

	t.Run("zero additional", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 1000},
			MonthlySMSCount: 1000,
		}
		assert.True(t, c.IsWithinMonthlyQuota(0))
	})

	t.Run("already over quota with zero additional", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 1000},
			MonthlySMSCount: 1001,
		}
		assert.False(t, c.IsWithinMonthlyQuota(0))
	})
}

func TestClient_RemainingMonthlyQuota(t *testing.T) {
	t.Run("nil plan returns zero", func(t *testing.T) {
		c := &Client{Plan: nil}
		assert.Equal(t, 0, c.RemainingMonthlyQuota())
	})

	t.Run("has remaining", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 1000},
			MonthlySMSCount: 700,
		}
		assert.Equal(t, 300, c.RemainingMonthlyQuota())
	})

	t.Run("no remaining", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 1000},
			MonthlySMSCount: 1000,
		}
		assert.Equal(t, 0, c.RemainingMonthlyQuota())
	})

	t.Run("over quota returns zero not negative", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 1000},
			MonthlySMSCount: 1500,
		}
		assert.Equal(t, 0, c.RemainingMonthlyQuota())
	})

	t.Run("zero max SMS", func(t *testing.T) {
		c := &Client{
			Plan:            &Plan{MaxSMSPerMonth: 0},
			MonthlySMSCount: 0,
		}
		assert.Equal(t, 0, c.RemainingMonthlyQuota())
	})
}

func TestClient_NeedsMonthlyReset(t *testing.T) {
	t.Run("needs reset when reset time is in the past", func(t *testing.T) {
		c := &Client{
			MonthlySMSResetAt: time.Now().Add(-time.Hour),
		}
		assert.True(t, c.NeedsMonthlyReset())
	})

	t.Run("does not need reset when reset time is in the future", func(t *testing.T) {
		c := &Client{
			MonthlySMSResetAt: time.Now().Add(24 * time.Hour),
		}
		assert.False(t, c.NeedsMonthlyReset())
	})
}

func TestClient_ZeroValue(t *testing.T) {
	var c Client
	assert.Equal(t, uuid.Nil, c.ID)
	assert.Equal(t, "", c.Name)
	assert.Equal(t, "", c.APIKey)
	assert.Equal(t, "", c.Secret)
	assert.Equal(t, "", c.Email)
	assert.False(t, c.Active)
	assert.False(t, c.IsActive())
	assert.False(t, c.IsSubAccount())
	assert.Nil(t, c.ParentClientID)
	assert.Nil(t, c.PlanID)
	assert.Nil(t, c.Config)
	assert.Nil(t, c.Plan)
	assert.False(t, c.IsReseller)
	assert.False(t, c.IsSandbox)
	assert.Equal(t, 0, c.MaxSubAccounts)
	assert.Equal(t, 0, c.MonthlySMSCount)
}

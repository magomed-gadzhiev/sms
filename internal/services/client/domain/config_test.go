package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientConfig_ToRateLimits(t *testing.T) {
	cfg := &ClientConfig{
		RateLimitPerSecond: 10,
		RateLimitPerMinute: 100,
		RateLimitPerHour:   1000,
		RateLimitPerDay:    10000,
	}

	rl := cfg.ToRateLimits()

	require.NotNil(t, rl)
	assert.Equal(t, 10, rl.PerSecond)
	assert.Equal(t, 100, rl.PerMinute)
	assert.Equal(t, 1000, rl.PerHour)
	assert.Equal(t, 10000, rl.PerDay)
}

func TestClientConfig_ToRateLimits_ZeroValues(t *testing.T) {
	cfg := &ClientConfig{}

	rl := cfg.ToRateLimits()

	require.NotNil(t, rl)
	assert.Equal(t, 0, rl.PerSecond)
	assert.Equal(t, 0, rl.PerMinute)
	assert.Equal(t, 0, rl.PerHour)
	assert.Equal(t, 0, rl.PerDay)
}

func TestClientConfig_GetSettings(t *testing.T) {
	t.Run("nil settings returns empty map", func(t *testing.T) {
		cfg := &ClientConfig{Settings: nil}
		s := cfg.GetSettings()
		assert.NotNil(t, s)
		assert.Empty(t, s)
	})

	t.Run("empty settings returns empty map", func(t *testing.T) {
		cfg := &ClientConfig{Settings: json.RawMessage{}}
		s := cfg.GetSettings()
		assert.NotNil(t, s)
		assert.Empty(t, s)
	})

	t.Run("valid JSON settings returns map", func(t *testing.T) {
		cfg := &ClientConfig{Settings: json.RawMessage(`{"webhook_url":"https://example.com","timeout":"30"}`)}
		s := cfg.GetSettings()
		assert.Equal(t, "https://example.com", s["webhook_url"])
		assert.Equal(t, "30", s["timeout"])
		assert.Len(t, s, 2)
	})

	t.Run("invalid JSON returns empty map", func(t *testing.T) {
		cfg := &ClientConfig{Settings: json.RawMessage(`not json`)}
		s := cfg.GetSettings()
		assert.NotNil(t, s)
		assert.Empty(t, s)
	})

	t.Run("non-string-value JSON returns empty map", func(t *testing.T) {
		cfg := &ClientConfig{Settings: json.RawMessage(`{"key": 42}`)}
		s := cfg.GetSettings()
		assert.NotNil(t, s)
		assert.Empty(t, s)
	})
}

func TestClientConfig_SetSettings(t *testing.T) {
	t.Run("nil settings sets empty object", func(t *testing.T) {
		cfg := &ClientConfig{}
		err := cfg.SetSettings(nil)
		require.NoError(t, err)
		assert.Equal(t, json.RawMessage("{}"), cfg.Settings)
	})

	t.Run("empty map sets empty object", func(t *testing.T) {
		cfg := &ClientConfig{}
		err := cfg.SetSettings(map[string]string{})
		require.NoError(t, err)
		assert.Equal(t, json.RawMessage("{}"), cfg.Settings)
	})

	t.Run("valid map sets JSON", func(t *testing.T) {
		cfg := &ClientConfig{}
		err := cfg.SetSettings(map[string]string{"key": "val"})
		require.NoError(t, err)

		var result map[string]string
		err = json.Unmarshal(cfg.Settings, &result)
		require.NoError(t, err)
		assert.Equal(t, "val", result["key"])
	})

	t.Run("roundtrip set then get", func(t *testing.T) {
		cfg := &ClientConfig{}
		original := map[string]string{"a": "1", "b": "2", "c": "3"}
		err := cfg.SetSettings(original)
		require.NoError(t, err)

		got := cfg.GetSettings()
		assert.Equal(t, original, got)
	})

	t.Run("overwrite existing settings", func(t *testing.T) {
		cfg := &ClientConfig{Settings: json.RawMessage(`{"old":"data"}`)}
		err := cfg.SetSettings(map[string]string{"new": "data"})
		require.NoError(t, err)

		got := cfg.GetSettings()
		assert.Equal(t, "data", got["new"])
		_, ok := got["old"]
		assert.False(t, ok)
	})
}

func TestClientConfig_ZeroValue(t *testing.T) {
	var cfg ClientConfig
	assert.Equal(t, uuid.Nil, cfg.ID)
	assert.Equal(t, uuid.Nil, cfg.ClientID)
	assert.Equal(t, 0, cfg.RateLimitPerSecond)
	assert.Equal(t, 0, cfg.RateLimitPerMinute)
	assert.Equal(t, 0, cfg.RateLimitPerHour)
	assert.Equal(t, 0, cfg.RateLimitPerDay)
	assert.Nil(t, cfg.AllowedSources)
	assert.Nil(t, cfg.BlockedDestinations)
	assert.Nil(t, cfg.Settings)
}

func TestClientConfig_AllowedSourcesAndBlockedDestinations(t *testing.T) {
	cfg := ClientConfig{
		AllowedSources:      []string{"SENDER1", "SENDER2"},
		BlockedDestinations: []string{"+7900*", "+7901*"},
	}

	assert.Len(t, cfg.AllowedSources, 2)
	assert.Contains(t, cfg.AllowedSources, "SENDER1")
	assert.Len(t, cfg.BlockedDestinations, 2)
	assert.Contains(t, cfg.BlockedDestinations, "+7900*")
}

func TestRateLimits_ZeroValue(t *testing.T) {
	var rl RateLimits
	assert.Equal(t, 0, rl.PerSecond)
	assert.Equal(t, 0, rl.PerMinute)
	assert.Equal(t, 0, rl.PerHour)
	assert.Equal(t, 0, rl.PerDay)
}

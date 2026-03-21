package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupResult_IsDeliverable(t *testing.T) {
	t.Run("returns true for active number", func(t *testing.T) {
		result := &LookupResult{NumberStatus: NumberStatusActive}
		assert.True(t, result.IsDeliverable())
	})

	t.Run("returns true for absent number", func(t *testing.T) {
		result := &LookupResult{NumberStatus: NumberStatusAbsent}
		assert.True(t, result.IsDeliverable())
	})

	t.Run("returns true for unknown number", func(t *testing.T) {
		result := &LookupResult{NumberStatus: NumberStatusUnknown}
		assert.True(t, result.IsDeliverable())
	})

	t.Run("returns false for invalid number", func(t *testing.T) {
		result := &LookupResult{NumberStatus: NumberStatusInvalid}
		assert.False(t, result.IsDeliverable())
	})
}

func TestLookupResult_IsInvalid(t *testing.T) {
	t.Run("returns true for invalid number", func(t *testing.T) {
		result := &LookupResult{NumberStatus: NumberStatusInvalid}
		assert.True(t, result.IsInvalid())
	})

	t.Run("returns false for active number", func(t *testing.T) {
		result := &LookupResult{NumberStatus: NumberStatusActive}
		assert.False(t, result.IsInvalid())
	})

	t.Run("returns false for absent number", func(t *testing.T) {
		result := &LookupResult{NumberStatus: NumberStatusAbsent}
		assert.False(t, result.IsInvalid())
	})

	t.Run("returns false for unknown number", func(t *testing.T) {
		result := &LookupResult{NumberStatus: NumberStatusUnknown}
		assert.False(t, result.IsInvalid())
	})
}

func TestNewHLRProvider(t *testing.T) {
	t.Run("creates provider with correct defaults", func(t *testing.T) {
		config := map[string]interface{}{"api_key": "secret123"}
		regions := []string{"RU", "KZ"}

		provider := NewHLRProvider("TestProvider", "http", config, 1, regions, 0.05)

		assert.NotEqual(t, uuid.Nil, provider.ID)
		assert.Equal(t, "TestProvider", provider.Name)
		assert.Equal(t, "http", provider.AdapterType)
		assert.Equal(t, config, provider.Config)
		assert.Equal(t, 1, provider.Priority)
		assert.Equal(t, regions, provider.SupportedRegions)
		assert.Equal(t, 0.05, provider.CostPerLookup)
		assert.Equal(t, HLRProviderStatusHealthy, provider.Status)
		assert.Equal(t, 100.0, provider.SuccessRate)
		assert.True(t, provider.Active)
		assert.False(t, provider.CreatedAt.IsZero())
		assert.False(t, provider.UpdatedAt.IsZero())
	})
}

func TestHLRProvider_IsAvailable(t *testing.T) {
	t.Run("returns true when active and healthy", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		assert.True(t, p.IsAvailable())
	})

	t.Run("returns true when active and degraded", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.Status = HLRProviderStatusDegraded
		assert.True(t, p.IsAvailable())
	})

	t.Run("returns false when active but unhealthy", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.Status = HLRProviderStatusUnhealthy
		assert.False(t, p.IsAvailable())
	})

	t.Run("returns false when active but disabled status", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.Status = HLRProviderStatusDisabled
		assert.False(t, p.IsAvailable())
	})

	t.Run("returns false when inactive", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.Active = false
		assert.False(t, p.IsAvailable())
	})
}

func TestHLRProvider_SupportsRegion(t *testing.T) {
	t.Run("returns true for supported region", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, []string{"RU", "KZ", "BY"}, 0.01)
		assert.True(t, p.SupportsRegion("KZ"))
	})

	t.Run("returns false for unsupported region", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, []string{"RU", "KZ"}, 0.01)
		assert.False(t, p.SupportsRegion("US"))
	})

	t.Run("returns false when no regions configured", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		assert.False(t, p.SupportsRegion("RU"))
	})
}

func TestHLRProvider_UpdateStatus(t *testing.T) {
	t.Run("sets healthy when success rate >= 95", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.UpdateStatus(95.0)
		assert.Equal(t, HLRProviderStatusHealthy, p.Status)
		assert.Equal(t, 95.0, p.SuccessRate)
	})

	t.Run("sets degraded when success rate >= 80 and < 95", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.UpdateStatus(85.0)
		assert.Equal(t, HLRProviderStatusDegraded, p.Status)
		assert.Equal(t, 85.0, p.SuccessRate)
	})

	t.Run("sets unhealthy when success rate < 80", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.UpdateStatus(50.0)
		assert.Equal(t, HLRProviderStatusUnhealthy, p.Status)
		assert.Equal(t, 50.0, p.SuccessRate)
	})

	t.Run("boundary at exactly 80.0 sets degraded", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.UpdateStatus(80.0)
		assert.Equal(t, HLRProviderStatusDegraded, p.Status)
	})

	t.Run("boundary at 79.9 sets unhealthy", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.UpdateStatus(79.9)
		assert.Equal(t, HLRProviderStatusUnhealthy, p.Status)
	})
}

func TestHLRProvider_RecordSuccess(t *testing.T) {
	t.Run("records LastSuccessAt timestamp", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		require.Nil(t, p.LastSuccessAt)

		p.RecordSuccess()

		require.NotNil(t, p.LastSuccessAt)
		assert.False(t, p.LastSuccessAt.IsZero())
	})
}

func TestHLRProvider_RecordFailure(t *testing.T) {
	t.Run("records LastFailureAt timestamp", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		require.Nil(t, p.LastFailureAt)

		p.RecordFailure()

		require.NotNil(t, p.LastFailureAt)
		assert.False(t, p.LastFailureAt.IsZero())
	})
}

func TestHLRProvider_Disable(t *testing.T) {
	t.Run("sets inactive and disabled status", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		require.True(t, p.Active)

		p.Disable()

		assert.False(t, p.Active)
		assert.Equal(t, HLRProviderStatusDisabled, p.Status)
	})
}

func TestHLRProvider_Enable(t *testing.T) {
	t.Run("sets active and resets to healthy with 100% success rate", func(t *testing.T) {
		p := NewHLRProvider("P", "http", nil, 1, nil, 0.01)
		p.Disable()
		p.SuccessRate = 50.0

		p.Enable()

		assert.True(t, p.Active)
		assert.Equal(t, HLRProviderStatusHealthy, p.Status)
		assert.Equal(t, 100.0, p.SuccessRate)
	})
}

func TestHLRProvider_MaskedConfig(t *testing.T) {
	t.Run("masks sensitive fields", func(t *testing.T) {
		config := map[string]interface{}{
			"api_key":  "secret-key-123",
			"password": "my-password",
			"secret":   "top-secret",
			"token":    "bearer-token",
			"base_url": "https://api.example.com",
			"timeout":  30,
		}
		p := NewHLRProvider("P", "http", config, 1, nil, 0.01)

		masked := p.MaskedConfig()

		assert.Equal(t, "***", masked["api_key"])
		assert.Equal(t, "***", masked["password"])
		assert.Equal(t, "***", masked["secret"])
		assert.Equal(t, "***", masked["token"])
		assert.Equal(t, "https://api.example.com", masked["base_url"])
		assert.Equal(t, 30, masked["timeout"])
	})

	t.Run("does not modify original config", func(t *testing.T) {
		config := map[string]interface{}{
			"api_key": "secret-key",
		}
		p := NewHLRProvider("P", "http", config, 1, nil, 0.01)

		_ = p.MaskedConfig()

		assert.Equal(t, "secret-key", p.Config["api_key"])
	})
}

func TestNewLookupLogEntry(t *testing.T) {
	t.Run("creates entry from lookup result", func(t *testing.T) {
		providerID := uuid.New()
		msgID := uuid.New()
		clientID := uuid.New()
		result := &LookupResult{
			MSISDN:         "+79001234567",
			OperatorMCCMNC: "25001",
			OperatorName:   "MTS",
			NumberStatus:   NumberStatusActive,
			CountryCode:    "RU",
			NumberType:     NumberTypeMobile,
			IsPorted:       true,
			HLRProviderID:  &providerID,
			Cached:         false,
			QueriedAt:      time.Now(),
		}

		entry := NewLookupLogEntry(result, LookupSourceSMSRouting, clientID, "req-123", &msgID, 150)

		assert.NotEqual(t, uuid.Nil, entry.ID)
		assert.Equal(t, "+79001234567", entry.MSISDN)
		assert.Equal(t, "25001", entry.OperatorMCCMNC)
		assert.Equal(t, "MTS", entry.OperatorName)
		assert.Equal(t, "active", entry.NumberStatus)
		assert.Equal(t, "RU", entry.CountryCode)
		assert.Equal(t, "mobile", entry.NumberType)
		assert.True(t, entry.IsPorted)
		assert.Equal(t, &providerID, entry.HLRProviderID)
		assert.Equal(t, LookupSourceSMSRouting, entry.Source)
		assert.Equal(t, clientID, entry.ClientID)
		assert.False(t, entry.Cached)
		assert.Equal(t, 150, entry.LatencyMs)
		assert.Equal(t, "req-123", entry.RequestID)
		assert.Equal(t, &msgID, entry.MessageID)
		assert.False(t, entry.CreatedAt.IsZero())
	})
}

package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestHLRService(
	cache *mocks.MockHLRCache,
	providerRepo *mocks.MockHLRProviderRepository,
	logRepo *mocks.MockLookupLogRepository,
	factory *mocks.MockAdapterFactory,
) *HLRService {
	return NewHLRService(cache, providerRepo, logRepo, factory, 200*time.Millisecond)
}

func TestHLRService(t *testing.T) {
	clientID := uuid.New()
	requestID := "req-123"
	source := domain.LookupSourceAPILookup
	var messageID *uuid.UUID

	t.Run("LookupNumber", func(t *testing.T) {

		t.Run("cache_hit", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			cachedResult := &domain.LookupResult{
				MSISDN:         "79001234567",
				OperatorMCCMNC: "25001",
				OperatorName:   "MTS",
				NumberStatus:   domain.NumberStatusActive,
				CountryCode:    "RU",
				NumberType:     domain.NumberTypeMobile,
				IsPorted:       false,
				Cached:         true,
				QueriedAt:      time.Now(),
			}

			cache.On("Get", mock.Anything, "79001234567").Return(cachedResult, nil)
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "79001234567", result.MSISDN)
			assert.Equal(t, "25001", result.OperatorMCCMNC)
			assert.True(t, result.Cached)

			// Provider should NOT be called on cache hit
			providerRepo.AssertNotCalled(t, "GetByPriority", mock.Anything, mock.Anything)
			providerRepo.AssertNotCalled(t, "ListActive", mock.Anything)
			cache.AssertExpectations(t)

			// Give goroutine time to complete log insertion
			time.Sleep(50 * time.Millisecond)
		})

		t.Run("cache_miss", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			adapter := new(mocks.MockHLRProviderAdapter)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			providerID := uuid.New()
			provider := &domain.HLRProvider{
				ID:          providerID,
				Name:        "TestHLR",
				AdapterType: "test",
				Priority:    1,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			lookupResult := &domain.LookupResult{
				MSISDN:         "79001234567",
				OperatorMCCMNC: "25001",
				OperatorName:   "MTS",
				NumberStatus:   domain.NumberStatusActive,
				CountryCode:    "RU",
				NumberType:     domain.NumberTypeMobile,
				IsPorted:       false,
				QueriedAt:      time.Now(),
			}

			// Cache miss
			cache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			// Provider repo returns providers
			providerRepo.On("GetByPriority", mock.Anything, "RU").Return([]*domain.HLRProvider{provider}, nil)
			// Factory creates adapter
			factory.On("CreateAdapter", provider).Return(adapter, nil)
			// Adapter performs lookup
			adapter.On("Lookup", mock.Anything, "79001234567").Return(lookupResult, nil)
			// Cache stores result
			cache.On("Set", mock.Anything, "79001234567", mock.AnythingOfType("*domain.LookupResult")).Return(nil)
			// Log repo stores log entry
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "79001234567", result.MSISDN)
			assert.Equal(t, "25001", result.OperatorMCCMNC)
			assert.Equal(t, &providerID, result.HLRProviderID)

			cache.AssertCalled(t, "Get", mock.Anything, "79001234567")
			cache.AssertCalled(t, "Set", mock.Anything, "79001234567", mock.AnythingOfType("*domain.LookupResult"))
			adapter.AssertCalled(t, "Lookup", mock.Anything, "79001234567")

			// Give goroutine time to complete log insertion
			time.Sleep(50 * time.Millisecond)
		})

		t.Run("invalid_E164", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			result, err := svc.LookupNumber(context.Background(), "abc", false, clientID, requestID, source, messageID)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrInvalidMSISDN)

			// Provider should NOT be called for invalid numbers
			providerRepo.AssertNotCalled(t, "GetByPriority", mock.Anything, mock.Anything)
			providerRepo.AssertNotCalled(t, "ListActive", mock.Anything)
			cache.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
		})

		t.Run("provider_failover", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			adapter1 := new(mocks.MockHLRProviderAdapter)
			adapter2 := new(mocks.MockHLRProviderAdapter)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			provider1ID := uuid.New()
			provider1 := &domain.HLRProvider{
				ID:          provider1ID,
				Name:        "Provider1",
				AdapterType: "test",
				Priority:    1,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}
			provider2ID := uuid.New()
			provider2 := &domain.HLRProvider{
				ID:          provider2ID,
				Name:        "Provider2",
				AdapterType: "test",
				Priority:    2,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			lookupResult := &domain.LookupResult{
				MSISDN:         "79001234567",
				OperatorMCCMNC: "25001",
				NumberStatus:   domain.NumberStatusActive,
				CountryCode:    "RU",
				QueriedAt:      time.Now(),
			}

			// Cache miss
			cache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			// Providers returned by priority
			providerRepo.On("GetByPriority", mock.Anything, "RU").
				Return([]*domain.HLRProvider{provider1, provider2}, nil)
			// Create adapters
			factory.On("CreateAdapter", provider1).Return(adapter1, nil)
			factory.On("CreateAdapter", provider2).Return(adapter2, nil)
			// Provider 1 fails
			adapter1.On("Lookup", mock.Anything, "79001234567").Return(nil, errors.New("timeout"))
			// Provider 2 succeeds
			adapter2.On("Lookup", mock.Anything, "79001234567").Return(lookupResult, nil)
			// Cache stores result
			cache.On("Set", mock.Anything, "79001234567", mock.AnythingOfType("*domain.LookupResult")).Return(nil)
			// Log repo
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, &provider2ID, result.HLRProviderID)

			// Both adapters should have been called
			adapter1.AssertCalled(t, "Lookup", mock.Anything, "79001234567")
			adapter2.AssertCalled(t, "Lookup", mock.Anything, "79001234567")

			// Give goroutine time
			time.Sleep(50 * time.Millisecond)
		})

		t.Run("all_providers_fail", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			adapter1 := new(mocks.MockHLRProviderAdapter)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			provider1 := &domain.HLRProvider{
				ID:          uuid.New(),
				Name:        "Provider1",
				AdapterType: "test",
				Priority:    1,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			// Cache miss
			cache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			// Providers
			providerRepo.On("GetByPriority", mock.Anything, "RU").
				Return([]*domain.HLRProvider{provider1}, nil)
			// Create adapter
			factory.On("CreateAdapter", provider1).Return(adapter1, nil)
			// Provider fails
			adapter1.On("Lookup", mock.Anything, "79001234567").Return(nil, errors.New("connection refused"))

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrHLRLookupFailed)
		})

		t.Run("force_refresh_skips_cache", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			adapter := new(mocks.MockHLRProviderAdapter)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			providerID := uuid.New()
			provider := &domain.HLRProvider{
				ID:          providerID,
				Name:        "TestHLR",
				AdapterType: "test",
				Priority:    1,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			lookupResult := &domain.LookupResult{
				MSISDN:       "79001234567",
				NumberStatus: domain.NumberStatusActive,
				CountryCode:  "RU",
				QueriedAt:    time.Now(),
			}

			// Provider repo returns providers (cache NOT called because forceRefresh=true)
			providerRepo.On("GetByPriority", mock.Anything, "RU").Return([]*domain.HLRProvider{provider}, nil)
			factory.On("CreateAdapter", provider).Return(adapter, nil)
			adapter.On("Lookup", mock.Anything, "79001234567").Return(lookupResult, nil)
			cache.On("Set", mock.Anything, "79001234567", mock.AnythingOfType("*domain.LookupResult")).Return(nil)
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.LookupNumber(context.Background(), "79001234567", true, clientID, requestID, source, messageID)

			require.NoError(t, err)
			require.NotNil(t, result)

			// Cache.Get should NOT be called when forceRefresh=true
			cache.AssertNotCalled(t, "Get", mock.Anything, mock.Anything)
			// But Cache.Set should still be called
			cache.AssertCalled(t, "Set", mock.Anything, "79001234567", mock.AnythingOfType("*domain.LookupResult"))

			time.Sleep(50 * time.Millisecond)
		})

		t.Run("short_code_returns_nil", func(t *testing.T) {
			// IsShortCode checks len(cleaned) <= 6, but ValidateMSISDN requires >=7 digits.
			// So a short code will always fail E164 validation first.
			// We verify IsShortCode works correctly as a standalone function.
			assert.True(t, IsShortCode("12345"))
			assert.True(t, IsShortCode("+1234"))
			assert.False(t, IsShortCode("1234567"))
			assert.False(t, IsShortCode("+79001234567"))
		})

		t.Run("strips_leading_plus", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			cachedResult := &domain.LookupResult{
				MSISDN:       "79001234567",
				NumberStatus: domain.NumberStatusActive,
				Cached:       true,
				QueriedAt:    time.Now(),
			}

			// Expects cleaned MSISDN (without +)
			cache.On("Get", mock.Anything, "79001234567").Return(cachedResult, nil)
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.LookupNumber(context.Background(), "+79001234567", false, clientID, requestID, source, messageID)

			require.NoError(t, err)
			require.NotNil(t, result)
			cache.AssertCalled(t, "Get", mock.Anything, "79001234567")

			time.Sleep(50 * time.Millisecond)
		})

		t.Run("no_providers_available", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			cache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			providerRepo.On("GetByPriority", mock.Anything, "RU").Return([]*domain.HLRProvider{}, nil)
			providerRepo.On("ListActive", mock.Anything).Return([]*domain.HLRProvider{}, nil)

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrHLRProviderUnavailable)
		})
	})

	t.Run("ValidateMSISDN", func(t *testing.T) {

		t.Run("valid", func(t *testing.T) {
			validNumbers := []string{
				"79001234567",
				"+79001234567",
				"1234567890",
				"447911123456",
				"33612345678",
			}

			for _, num := range validNumbers {
				t.Run(num, func(t *testing.T) {
					assert.True(t, ValidateMSISDN(num), "expected %s to be valid", num)
				})
			}
		})

		t.Run("invalid", func(t *testing.T) {
			invalidNumbers := []string{
				"",
				"abc",
				"12345",   // too short (6 digits min after cleaning)
				"0123456", // starts with 0
			}

			for _, num := range invalidNumbers {
				t.Run(num, func(t *testing.T) {
					assert.False(t, ValidateMSISDN(num), "expected %s to be invalid", num)
				})
			}
		})
	})

	t.Run("GetAdapters", func(t *testing.T) {
		t.Run("returns_adapters_map", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			adapters := svc.GetAdapters()
			assert.NotNil(t, adapters)
			assert.Len(t, adapters, 0)
		})
	})

	t.Run("GetAdaptersMu", func(t *testing.T) {
		t.Run("returns_mutex", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			mu := svc.GetAdaptersMu()
			assert.NotNil(t, mu)
		})
	})

	t.Run("extractCountryCode", func(t *testing.T) {
		t.Run("US_number", func(t *testing.T) {
			assert.Equal(t, "US", extractCountryCode("12025551234"))
		})

		t.Run("RU_number", func(t *testing.T) {
			assert.Equal(t, "RU", extractCountryCode("79001234567"))
		})

		t.Run("FR_number", func(t *testing.T) {
			assert.Equal(t, "FR", extractCountryCode("33612345678"))
		})

		t.Run("ES_number", func(t *testing.T) {
			assert.Equal(t, "ES", extractCountryCode("34612345678"))
		})

		t.Run("IT_number", func(t *testing.T) {
			assert.Equal(t, "IT", extractCountryCode("39312345678"))
		})

		t.Run("GB_number", func(t *testing.T) {
			assert.Equal(t, "GB", extractCountryCode("447911123456"))
		})

		t.Run("DE_number", func(t *testing.T) {
			assert.Equal(t, "DE", extractCountryCode("491512345678"))
		})

		t.Run("UA_number", func(t *testing.T) {
			assert.Equal(t, "UA", extractCountryCode("380501234567"))
		})

		t.Run("BY_number", func(t *testing.T) {
			assert.Equal(t, "BY", extractCountryCode("375291234567"))
		})

		t.Run("short_number_returns_empty", func(t *testing.T) {
			assert.Equal(t, "", extractCountryCode("12345"))
		})

		t.Run("unknown_prefix_returns_empty", func(t *testing.T) {
			assert.Equal(t, "", extractCountryCode("90555123456"))
		})

		t.Run("38_non_UA_returns_empty", func(t *testing.T) {
			assert.Equal(t, "", extractCountryCode("38112345678"))
		})

		t.Run("37_non_BY_returns_empty", func(t *testing.T) {
			assert.Equal(t, "", extractCountryCode("37112345678"))
		})
	})

	t.Run("NewHLRService_default_timeout", func(t *testing.T) {
		t.Run("uses_default_when_zero", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := NewHLRService(cache, providerRepo, logRepo, factory, 0)
			assert.Equal(t, 200*time.Millisecond, svc.lookupTimeout)
		})

		t.Run("uses_provided_timeout", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := NewHLRService(cache, providerRepo, logRepo, factory, 500*time.Millisecond)
			assert.Equal(t, 500*time.Millisecond, svc.lookupTimeout)
		})
	})

	t.Run("LookupNumber_cache_error_continues", func(t *testing.T) {
		t.Run("cache_get_error_queries_providers", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			adapter := new(mocks.MockHLRProviderAdapter)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			providerID := uuid.New()
			provider := &domain.HLRProvider{
				ID:          providerID,
				Name:        "TestHLR",
				AdapterType: "test",
				Priority:    1,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			lookupResult := &domain.LookupResult{
				MSISDN:       "79001234567",
				NumberStatus: domain.NumberStatusActive,
				CountryCode:  "RU",
				QueriedAt:    time.Now(),
			}

			// Cache returns error
			cache.On("Get", mock.Anything, "79001234567").Return(nil, errors.New("redis error"))
			providerRepo.On("GetByPriority", mock.Anything, "RU").Return([]*domain.HLRProvider{provider}, nil)
			factory.On("CreateAdapter", provider).Return(adapter, nil)
			adapter.On("Lookup", mock.Anything, "79001234567").Return(lookupResult, nil)
			cache.On("Set", mock.Anything, "79001234567", mock.AnythingOfType("*domain.LookupResult")).Return(nil)
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.NoError(t, err)
			require.NotNil(t, result)

			time.Sleep(50 * time.Millisecond)
		})
	})

	t.Run("LookupNumber_cache_set_error_continues", func(t *testing.T) {
		t.Run("cache_set_error_still_returns_result", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			adapter := new(mocks.MockHLRProviderAdapter)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			providerID := uuid.New()
			provider := &domain.HLRProvider{
				ID:          providerID,
				Name:        "TestHLR",
				AdapterType: "test",
				Priority:    1,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			lookupResult := &domain.LookupResult{
				MSISDN:         "79001234567",
				OperatorMCCMNC: "25001",
				NumberStatus:   domain.NumberStatusActive,
				CountryCode:    "RU",
				QueriedAt:      time.Now(),
			}

			cache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			providerRepo.On("GetByPriority", mock.Anything, "RU").Return([]*domain.HLRProvider{provider}, nil)
			factory.On("CreateAdapter", provider).Return(adapter, nil)
			adapter.On("Lookup", mock.Anything, "79001234567").Return(lookupResult, nil)
			// Cache set fails
			cache.On("Set", mock.Anything, "79001234567", mock.AnythingOfType("*domain.LookupResult")).
				Return(errors.New("redis write error"))
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "25001", result.OperatorMCCMNC)

			time.Sleep(50 * time.Millisecond)
		})
	})

	t.Run("queryProviders_fallback_to_list_active", func(t *testing.T) {
		t.Run("GetByPriority_error_falls_back_to_ListActive", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			adapter := new(mocks.MockHLRProviderAdapter)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			providerID := uuid.New()
			provider := &domain.HLRProvider{
				ID:          providerID,
				Name:        "FallbackProvider",
				AdapterType: "test",
				Priority:    1,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			lookupResult := &domain.LookupResult{
				MSISDN:       "79001234567",
				NumberStatus: domain.NumberStatusActive,
				CountryCode:  "RU",
				QueriedAt:    time.Now(),
			}

			cache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			// GetByPriority fails
			providerRepo.On("GetByPriority", mock.Anything, "RU").
				Return(nil, errors.New("not configured"))
			// Falls back to ListActive
			providerRepo.On("ListActive", mock.Anything).
				Return([]*domain.HLRProvider{provider}, nil)
			factory.On("CreateAdapter", provider).Return(adapter, nil)
			adapter.On("Lookup", mock.Anything, "79001234567").Return(lookupResult, nil)
			cache.On("Set", mock.Anything, "79001234567", mock.AnythingOfType("*domain.LookupResult")).Return(nil)
			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).Return(nil)

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.NoError(t, err)
			require.NotNil(t, result)

			time.Sleep(50 * time.Millisecond)
		})

		t.Run("ListActive_error_returns_error", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			cache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			providerRepo.On("GetByPriority", mock.Anything, "RU").
				Return(nil, errors.New("error"))
			providerRepo.On("ListActive", mock.Anything).
				Return(nil, errors.New("db down"))

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "HLR провайдеров")
		})
	})

	t.Run("queryProviders_adapter_factory_error", func(t *testing.T) {
		t.Run("factory_error_skips_provider", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			provider1 := &domain.HLRProvider{
				ID:          uuid.New(),
				Name:        "BadAdapter",
				AdapterType: "broken",
				Priority:    1,
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			cache.On("Get", mock.Anything, "79001234567").Return(nil, nil)
			providerRepo.On("GetByPriority", mock.Anything, "RU").
				Return([]*domain.HLRProvider{provider1}, nil)
			factory.On("CreateAdapter", provider1).
				Return(nil, errors.New("unsupported adapter"))

			result, err := svc.LookupNumber(context.Background(), "79001234567", false, clientID, requestID, source, messageID)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, domain.ErrHLRLookupFailed)
		})
	})

	t.Run("getOrCreateAdapter_reuses_existing", func(t *testing.T) {
		t.Run("returns_cached_adapter", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)
			adapter := new(mocks.MockHLRProviderAdapter)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			providerID := uuid.New()
			provider := &domain.HLRProvider{
				ID:          providerID,
				Name:        "Test",
				AdapterType: "test",
				Status:      domain.HLRProviderStatusHealthy,
				Active:      true,
			}

			// Pre-populate adapter
			svc.adapters[providerID] = adapter

			got, err := svc.getOrCreateAdapter(provider)

			require.NoError(t, err)
			assert.Equal(t, adapter, got)
			// Factory should NOT be called
			factory.AssertNotCalled(t, "CreateAdapter", mock.Anything)
		})
	})

	t.Run("logLookup_error", func(t *testing.T) {
		t.Run("log_insert_error_does_not_panic", func(t *testing.T) {
			cache := new(mocks.MockHLRCache)
			providerRepo := new(mocks.MockHLRProviderRepository)
			logRepo := new(mocks.MockLookupLogRepository)
			factory := new(mocks.MockAdapterFactory)

			svc := newTestHLRService(cache, providerRepo, logRepo, factory)

			result := &domain.LookupResult{
				MSISDN:       "79001234567",
				NumberStatus: domain.NumberStatusActive,
				QueriedAt:    time.Now(),
			}

			logRepo.On("Insert", mock.Anything, mock.AnythingOfType("*domain.LookupLogEntry")).
				Return(errors.New("db error"))

			// Should not panic
			svc.logLookup(context.Background(), result, source, clientID, requestID, messageID, 10)

			time.Sleep(20 * time.Millisecond)
			logRepo.AssertExpectations(t)
		})
	})

	t.Run("NewHealthMonitor", func(t *testing.T) {
		t.Run("default_interval", func(t *testing.T) {
			providerRepo := new(mocks.MockHLRProviderRepository)
			factory := new(mocks.MockAdapterFactory)
			adapters := make(map[uuid.UUID]domain.HLRProviderAdapter)
			var mu sync.RWMutex

			hm := NewHealthMonitor(providerRepo, adapters, &mu, factory, 0)

			assert.NotNil(t, hm)
			assert.Equal(t, 30*time.Second, hm.interval)
		})

		t.Run("custom_interval", func(t *testing.T) {
			providerRepo := new(mocks.MockHLRProviderRepository)
			factory := new(mocks.MockAdapterFactory)
			adapters := make(map[uuid.UUID]domain.HLRProviderAdapter)
			var mu sync.RWMutex

			hm := NewHealthMonitor(providerRepo, adapters, &mu, factory, 10*time.Second)

			assert.NotNil(t, hm)
			assert.Equal(t, 10*time.Second, hm.interval)
		})
	})

	t.Run("HealthMonitor_StartStop", func(t *testing.T) {
		t.Run("starts_and_stops_without_panic", func(t *testing.T) {
			providerRepo := new(mocks.MockHLRProviderRepository)
			factory := new(mocks.MockAdapterFactory)
			adapters := make(map[uuid.UUID]domain.HLRProviderAdapter)
			var mu sync.RWMutex

			hm := NewHealthMonitor(providerRepo, adapters, &mu, factory, 100*time.Millisecond)

			// Should not panic
			hm.Start()
			time.Sleep(50 * time.Millisecond)
			hm.Stop()
			time.Sleep(50 * time.Millisecond)
		})
	})
}

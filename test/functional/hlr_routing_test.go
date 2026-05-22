//go:build functional

package functional_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/routing/application"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	routingrepo "github.com/smpp-server/smpp-server/internal/services/routing/infrastructure/repository"
)

// ---------------------------------------------------------------------------
// Mock: HLRProviderAdapter
// ---------------------------------------------------------------------------

type mockHLRAdapter struct {
	mock.Mock
}

func (m *mockHLRAdapter) Lookup(ctx context.Context, msisdn string) (*domain.LookupResult, error) {
	args := m.Called(ctx, msisdn)
	res := args.Get(0)
	if res == nil {
		return nil, args.Error(1)
	}
	return res.(*domain.LookupResult), args.Error(1)
}

func (m *mockHLRAdapter) Ping(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *mockHLRAdapter) Name() string {
	return m.Called().String(0)
}

// ---------------------------------------------------------------------------
// Mock: AdapterFactory -- returns pre-configured mock adapters by provider ID
// ---------------------------------------------------------------------------

type mockAdapterFactory struct {
	adapters map[uuid.UUID]domain.HLRProviderAdapter
}

func newMockAdapterFactory() *mockAdapterFactory {
	return &mockAdapterFactory{adapters: make(map[uuid.UUID]domain.HLRProviderAdapter)}
}

func (f *mockAdapterFactory) Register(providerID uuid.UUID, adapter domain.HLRProviderAdapter) {
	f.adapters[providerID] = adapter
}

func (f *mockAdapterFactory) CreateAdapter(provider *domain.HLRProvider) (domain.HLRProviderAdapter, error) {
	adapter, ok := f.adapters[provider.ID]
	if !ok {
		return nil, fmt.Errorf("no mock adapter registered for provider %s", provider.ID)
	}
	return adapter, nil
}

// ---------------------------------------------------------------------------
// Mock: HLRCache -- in-memory mock that implements domain.HLRCache
// ---------------------------------------------------------------------------

type mockHLRCache struct {
	store map[string]*domain.LookupResult
}

func newMockHLRCache() *mockHLRCache {
	return &mockHLRCache{store: make(map[string]*domain.LookupResult)}
}

func (c *mockHLRCache) Get(_ context.Context, msisdn string) (*domain.LookupResult, error) {
	r, ok := c.store[msisdn]
	if !ok {
		return nil, nil // cache miss
	}
	// Return a copy with Cached flag set, mirroring real Redis cache behaviour.
	cp := *r
	cp.Cached = true
	return &cp, nil
}

func (c *mockHLRCache) Set(_ context.Context, msisdn string, result *domain.LookupResult) error {
	c.store[msisdn] = result
	return nil
}

func (c *mockHLRCache) Delete(_ context.Context, msisdn string) error {
	delete(c.store, msisdn)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// ensureLookupLogPartition creates the monthly partition for lookup_log
// covering the current month if it does not exist already.
func ensureLookupLogPartition(t *testing.T, db *sqlx.DB) {
	t.Helper()

	now := time.Now()
	partName := fmt.Sprintf("lookup_log_%d_%02d", now.Year(), now.Month())
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	query := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF lookup_log FOR VALUES FROM ('%s') TO ('%s')`,
		partName,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, query); err != nil {
		t.Logf("partition %s may already exist: %v", partName, err)
	}
}

// cleanupHLRTables deletes test data from HLR-related tables.
func cleanupHLRTables(t *testing.T, db *sqlx.DB) {
	t.Helper()
	ctx := context.Background()
	// Order matters: lookup_log references hlr_providers.
	for _, tbl := range []string{"lookup_log", "hlr_providers"} {
		if _, err := db.ExecContext(ctx, "DELETE FROM "+tbl); err != nil {
			t.Logf("cleanup: failed to delete from %s: %v", tbl, err)
		}
	}
}

func insertHLRProvider(t *testing.T, repo domain.HLRProviderRepository, name string, priority int, regions []string) *domain.HLRProvider {
	t.Helper()

	provider := domain.NewHLRProvider(
		name,
		"mock",
		map[string]interface{}{"url": "http://mock"},
		priority,
		regions,
		0.01,
	)
	err := repo.Create(context.Background(), provider)
	require.NoError(t, err, "inserting HLR provider %s", name)
	return provider
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHLRRoutingChain(t *testing.T) {
	skipIfNoDB(t)

	db := setupTestDB(t)

	// Ensure lookup_log partition exists for the current month.
	ensureLookupLogPartition(t, db)

	// Clean tables before and after.
	cleanupHLRTables(t, db)
	t.Cleanup(func() { cleanupHLRTables(t, db) })

	providerRepo := routingrepo.NewHLRProviderRepository(db)
	logRepo := routingrepo.NewLookupLogRepository(db)

	t.Run("LookupWithCache", func(t *testing.T) {
		// Insert a provider into the DB.
		provider := insertHLRProvider(t, providerRepo, "mock-provider-cache", 1, []string{"RU"})

		// Build a mock adapter that returns a known result.
		adapter := new(mockHLRAdapter)
		expectedResult := &domain.LookupResult{
			MSISDN:         "79001234567",
			OperatorMCCMNC: "25001",
			OperatorName:   "MTS",
			NumberStatus:   domain.NumberStatusActive,
			CountryCode:    "RU",
			NumberType:     domain.NumberTypeMobile,
			IsPorted:       false,
			QueriedAt:      time.Now(),
		}
		adapter.On("Lookup", mock.Anything, "79001234567").Return(expectedResult, nil)

		factory := newMockAdapterFactory()
		factory.Register(provider.ID, adapter)

		cache := newMockHLRCache()

		hlrService := application.NewHLRService(
			cache,
			providerRepo,
			logRepo,
			factory,
			5*time.Second, // generous timeout for tests
		)

		ctx := context.Background()
		clientID := uuid.New()
		requestID := uuid.New().String()

		// First lookup -- cache miss, hits the mock adapter.
		result, err := hlrService.LookupNumber(
			ctx, "79001234567", false, clientID, requestID,
			domain.LookupSourceAPILookup, nil,
		)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "25001", result.OperatorMCCMNC)
		assert.Equal(t, "MTS", result.OperatorName)
		assert.Equal(t, domain.NumberStatusActive, result.NumberStatus)
		assert.False(t, result.Cached, "first lookup must not be cached")

		adapter.AssertCalled(t, "Lookup", mock.Anything, "79001234567")

		// Second lookup -- should come from cache (adapter should not be called again).
		result2, err := hlrService.LookupNumber(
			ctx, "79001234567", false, clientID, requestID,
			domain.LookupSourceAPILookup, nil,
		)
		require.NoError(t, err)
		require.NotNil(t, result2)
		assert.True(t, result2.Cached, "second lookup must be from cache")
		assert.Equal(t, "25001", result2.OperatorMCCMNC)

		// Adapter Lookup should have been called exactly once.
		adapter.AssertNumberOfCalls(t, "Lookup", 1)

		// Give log goroutine a moment to persist.
		time.Sleep(200 * time.Millisecond)

		// Verify lookup_log was written.
		count, err := logRepo.CountByClient(ctx, clientID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(1), "lookup_log must contain at least one entry")
	})

	t.Run("ProviderFailover", func(t *testing.T) {
		// Clean before subtest to avoid name conflicts.
		cleanupHLRTables(t, db)

		// Create two providers: primary (prio 1) fails, secondary (prio 2) succeeds.
		primary := insertHLRProvider(t, providerRepo, "primary-failover", 1, []string{"RU"})
		secondary := insertHLRProvider(t, providerRepo, "secondary-failover", 2, []string{"RU"})

		adapterPrimary := new(mockHLRAdapter)
		adapterPrimary.On("Lookup", mock.Anything, "79009876543").
			Return(nil, errors.New("provider timeout"))

		expectedResult := &domain.LookupResult{
			MSISDN:         "79009876543",
			OperatorMCCMNC: "25002",
			OperatorName:   "Megafon",
			NumberStatus:   domain.NumberStatusActive,
			CountryCode:    "RU",
			NumberType:     domain.NumberTypeMobile,
			IsPorted:       true,
			QueriedAt:      time.Now(),
		}
		adapterSecondary := new(mockHLRAdapter)
		adapterSecondary.On("Lookup", mock.Anything, "79009876543").
			Return(expectedResult, nil)

		factory := newMockAdapterFactory()
		factory.Register(primary.ID, adapterPrimary)
		factory.Register(secondary.ID, adapterSecondary)

		cache := newMockHLRCache()

		hlrService := application.NewHLRService(
			cache,
			providerRepo,
			logRepo,
			factory,
			5*time.Second,
		)

		ctx := context.Background()
		clientID := uuid.New()
		requestID := uuid.New().String()

		result, err := hlrService.LookupNumber(
			ctx, "79009876543", false, clientID, requestID,
			domain.LookupSourceSMSRouting, nil,
		)
		require.NoError(t, err, "failover lookup must succeed via secondary provider")
		require.NotNil(t, result)
		assert.Equal(t, "25002", result.OperatorMCCMNC, "result should come from secondary")
		assert.Equal(t, "Megafon", result.OperatorName)
		assert.True(t, result.IsPorted)
		assert.Equal(t, &secondary.ID, result.HLRProviderID, "HLRProviderID must point to secondary")

		// Both adapters should have been called.
		adapterPrimary.AssertCalled(t, "Lookup", mock.Anything, "79009876543")
		adapterSecondary.AssertCalled(t, "Lookup", mock.Anything, "79009876543")

		// Give log goroutine a moment.
		time.Sleep(200 * time.Millisecond)

		// Verify lookup_log recorded the successful result.
		count, err := logRepo.CountByClient(ctx, clientID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(1), "lookup_log must contain the successful entry")
	})
}

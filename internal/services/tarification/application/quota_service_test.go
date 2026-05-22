package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

// MockAggregatorQuotaRepository is a mock for domain.AggregatorQuotaRepository
type MockAggregatorQuotaRepository struct {
	mock.Mock
}

func (m *MockAggregatorQuotaRepository) Create(ctx context.Context, quota *domain.AggregatorQuota) error {
	args := m.Called(ctx, quota)
	return args.Error(0)
}

func (m *MockAggregatorQuotaRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.AggregatorQuota, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AggregatorQuota), args.Error(1)
}

func (m *MockAggregatorQuotaRepository) GetActive(ctx context.Context, aggregatorID uuid.UUID, now time.Time) (*domain.AggregatorQuota, error) {
	args := m.Called(ctx, aggregatorID, now)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AggregatorQuota), args.Error(1)
}

func (m *MockAggregatorQuotaRepository) Update(ctx context.Context, quota *domain.AggregatorQuota) error {
	args := m.Called(ctx, quota)
	return args.Error(0)
}

func (m *MockAggregatorQuotaRepository) IncrementUsage(ctx context.Context, quotaID uuid.UUID, segments int) (*domain.IncrementResult, error) {
	args := m.Called(ctx, quotaID, segments)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.IncrementResult), args.Error(1)
}

func (m *MockAggregatorQuotaRepository) ListByAggregator(ctx context.Context, aggregatorID uuid.UUID, limit, offset int) ([]*domain.AggregatorQuota, int, error) {
	args := m.Called(ctx, aggregatorID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.AggregatorQuota), args.Int(1), args.Error(2)
}

func (m *MockAggregatorQuotaRepository) SetNotified80(ctx context.Context, quotaID uuid.UUID) error {
	args := m.Called(ctx, quotaID)
	return args.Error(0)
}

func (m *MockAggregatorQuotaRepository) SetNotified100(ctx context.Context, quotaID uuid.UUID) error {
	args := m.Called(ctx, quotaID)
	return args.Error(0)
}

func (m *MockAggregatorQuotaRepository) ListAutoRenewable(ctx context.Context, beforeDate time.Time) ([]*domain.AggregatorQuota, error) {
	args := m.Called(ctx, beforeDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AggregatorQuota), args.Error(1)
}

// TestQuotaService_ConsumeQuota_WithinLimit: quota 1000, used 100, +5 segments, no overage
func TestQuotaService_ConsumeQuota_WithinLimit(t *testing.T) {
	repo := &MockAggregatorQuotaRepository{}
	svc := NewQuotaService(repo)

	aggregatorID := uuid.New()
	quotaID := uuid.New()

	quota := &domain.AggregatorQuota{
		ID:           quotaID,
		AggregatorID: aggregatorID,
		SegmentLimit: 1000,
		SegmentsUsed: 100,
		OverageRate:  "0.50",
		Currency:     "USD",
	}

	incrResult := &domain.IncrementResult{
		SegmentsUsed:   105,
		SegmentLimit:   1000,
		OverageRate:    "0.50",
		WasWithinQuota: true,
		OverageCount:   0,
	}

	repo.On("GetActive", mock.Anything, aggregatorID, mock.AnythingOfType("time.Time")).Return(quota, nil)
	repo.On("IncrementUsage", mock.Anything, quotaID, 5).Return(incrResult, nil)

	result, err := svc.ConsumeQuota(context.Background(), aggregatorID, 5)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.NoQuota)
	assert.False(t, result.HasOverage)
	assert.Equal(t, "0", result.OverageChargeAmount)
	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, quotaID, result.QuotaID)

	repo.AssertExpectations(t)
}

// TestQuotaService_ConsumeQuota_CrossesLimit: quota 1000, used 998, +5 segments, overage=3, amount="1.50"
func TestQuotaService_ConsumeQuota_CrossesLimit(t *testing.T) {
	repo := &MockAggregatorQuotaRepository{}
	svc := NewQuotaService(repo)

	aggregatorID := uuid.New()
	quotaID := uuid.New()

	quota := &domain.AggregatorQuota{
		ID:           quotaID,
		AggregatorID: aggregatorID,
		SegmentLimit: 1000,
		SegmentsUsed: 998,
		OverageRate:  "0.50",
		Currency:     "USD",
	}

	incrResult := &domain.IncrementResult{
		SegmentsUsed:   1003,
		SegmentLimit:   1000,
		OverageRate:    "0.50",
		WasWithinQuota: true,
		OverageCount:   3,
	}

	repo.On("GetActive", mock.Anything, aggregatorID, mock.AnythingOfType("time.Time")).Return(quota, nil)
	repo.On("IncrementUsage", mock.Anything, quotaID, 5).Return(incrResult, nil)

	result, err := svc.ConsumeQuota(context.Background(), aggregatorID, 5)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.NoQuota)
	assert.True(t, result.HasOverage)
	assert.Equal(t, "1.50", result.OverageChargeAmount)
	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, quotaID, result.QuotaID)

	repo.AssertExpectations(t)
}

// TestQuotaService_ConsumeQuota_NoActiveQuota: GetActive returns nil, result.NoQuota=true
func TestQuotaService_ConsumeQuota_NoActiveQuota(t *testing.T) {
	repo := &MockAggregatorQuotaRepository{}
	svc := NewQuotaService(repo)

	aggregatorID := uuid.New()

	repo.On("GetActive", mock.Anything, aggregatorID, mock.AnythingOfType("time.Time")).Return(nil, nil)

	result, err := svc.ConsumeQuota(context.Background(), aggregatorID, 5)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.NoQuota)
	assert.False(t, result.HasOverage)

	repo.AssertExpectations(t)
}

// TestQuotaService_ConsumeQuota_FullyInOverage: quota 1000, used 1010, +5 segments, all 5 are overage
func TestQuotaService_ConsumeQuota_FullyInOverage(t *testing.T) {
	repo := &MockAggregatorQuotaRepository{}
	svc := NewQuotaService(repo)

	aggregatorID := uuid.New()
	quotaID := uuid.New()

	quota := &domain.AggregatorQuota{
		ID:           quotaID,
		AggregatorID: aggregatorID,
		SegmentLimit: 1000,
		SegmentsUsed: 1010,
		OverageRate:  "0.50",
		Currency:     "EUR",
	}

	incrResult := &domain.IncrementResult{
		SegmentsUsed:   1015,
		SegmentLimit:   1000,
		OverageRate:    "0.50",
		WasWithinQuota: false,
		OverageCount:   5,
	}

	repo.On("GetActive", mock.Anything, aggregatorID, mock.AnythingOfType("time.Time")).Return(quota, nil)
	repo.On("IncrementUsage", mock.Anything, quotaID, 5).Return(incrResult, nil)

	result, err := svc.ConsumeQuota(context.Background(), aggregatorID, 5)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.NoQuota)
	assert.True(t, result.HasOverage)
	assert.Equal(t, "2.50", result.OverageChargeAmount)
	assert.Equal(t, "EUR", result.Currency)
	assert.Equal(t, quotaID, result.QuotaID)

	repo.AssertExpectations(t)
}

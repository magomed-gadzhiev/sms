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

func TestQuotaService_ConsumeQuota_AllSegmentsInOverage(t *testing.T) {
	repo := &MockAggregatorQuotaRepository{}
	svc := NewQuotaService(repo)

	aggID := uuid.New()
	quotaID := uuid.New()

	// Already past limit (1010 used of 1000)
	repo.On("GetActive", mock.Anything, aggID, mock.AnythingOfType("time.Time")).Return(&domain.AggregatorQuota{
		ID: quotaID, AggregatorID: aggID, SegmentLimit: 1000, SegmentsUsed: 1010,
		OverageRate: "0.25", Currency: "RUB",
	}, nil)
	repo.On("IncrementUsage", mock.Anything, quotaID, 5).Return(&domain.IncrementResult{
		SegmentsUsed: 1015, SegmentLimit: 1000, OverageRate: "0.25",
		WasWithinQuota: false, OverageCount: 5,
	}, nil)

	result, err := svc.ConsumeQuota(context.Background(), aggID, 5)
	require.NoError(t, err)
	assert.True(t, result.HasOverage)
	assert.Equal(t, "1.25", result.OverageChargeAmount) // 5 * 0.25
	repo.AssertExpectations(t)
}

func TestQuotaService_CreateAndGetActive(t *testing.T) {
	repo := &MockAggregatorQuotaRepository{}
	svc := NewQuotaService(repo)

	aggID := uuid.New()

	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.AggregatorQuota")).Return(nil)

	quota, err := svc.CreateQuota(context.Background(), aggID,
		parseQuotaDate("2026-05-01"), parseQuotaDate("2026-06-01"),
		500000, "0.10", "RUB", true)
	require.NoError(t, err)
	assert.Equal(t, aggID, quota.AggregatorID)
	assert.Equal(t, int64(500000), quota.SegmentLimit)
	assert.Equal(t, "0.10", quota.OverageRate)
	assert.True(t, quota.AutoRenew)
	repo.AssertExpectations(t)
}

func TestQuotaService_UpdateQuota(t *testing.T) {
	repo := &MockAggregatorQuotaRepository{}
	svc := NewQuotaService(repo)

	quotaID := uuid.New()
	existing := &domain.AggregatorQuota{
		ID: quotaID, SegmentLimit: 1000, OverageRate: "0.50", AutoRenew: true,
	}

	repo.On("GetByID", mock.Anything, quotaID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*domain.AggregatorQuota")).Return(nil)

	updated, err := svc.UpdateQuota(context.Background(), quotaID, 2000, "0.25", false)
	require.NoError(t, err)
	assert.Equal(t, int64(2000), updated.SegmentLimit)
	assert.Equal(t, "0.25", updated.OverageRate)
	assert.False(t, updated.AutoRenew)
	repo.AssertExpectations(t)
}

func TestQuotaService_UpdateQuota_NotFound(t *testing.T) {
	repo := &MockAggregatorQuotaRepository{}
	svc := NewQuotaService(repo)

	repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, nil)

	_, err := svc.UpdateQuota(context.Background(), uuid.New(), 2000, "0.25", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	repo.AssertExpectations(t)
}

func TestAggregatorQuota_DomainMethods(t *testing.T) {
	q := &domain.AggregatorQuota{
		SegmentLimit: 1000,
		SegmentsUsed: 800,
	}
	assert.False(t, q.IsExhausted())
	assert.InDelta(t, 80.0, q.UtilizationPercent(), 0.01)
	assert.Equal(t, int64(0), q.OverageSegments())

	q.SegmentsUsed = 1000
	assert.True(t, q.IsExhausted())
	assert.InDelta(t, 100.0, q.UtilizationPercent(), 0.01)

	q.SegmentsUsed = 1200
	assert.True(t, q.IsExhausted())
	assert.Equal(t, int64(200), q.OverageSegments())
	assert.InDelta(t, 120.0, q.UtilizationPercent(), 0.01)
}

func parseQuotaDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

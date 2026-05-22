package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/analytics/domain"
	"github.com/stretchr/testify/mock"
)

// MockMetricRepository is a mock implementation of domain.MetricRepository
type MockMetricRepository struct {
	mock.Mock
}

func (m *MockMetricRepository) Create(ctx context.Context, metric *domain.Metric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

func (m *MockMetricRepository) CreateAggregated(ctx context.Context, metric *domain.AggregatedMetric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

func (m *MockMetricRepository) GetAggregated(ctx context.Context, filters *domain.AggregateFilters) ([]*domain.AggregatedMetric, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AggregatedMetric), args.Error(1)
}

func (m *MockMetricRepository) GetStatistics(ctx context.Context, filters *domain.StatisticsFilters) (*domain.Statistics, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Statistics), args.Error(1)
}

func (m *MockMetricRepository) GetProviderPerformance(ctx context.Context, providerID uuid.UUID, from, to time.Time) (*domain.ProviderPerformance, error) {
	args := m.Called(ctx, providerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderPerformance), args.Error(1)
}

func (m *MockMetricRepository) UpdateAggregated(ctx context.Context, metric *domain.AggregatedMetric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

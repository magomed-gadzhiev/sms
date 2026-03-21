package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockLookupLogRepository is a mock implementation of domain.LookupLogRepository
type MockLookupLogRepository struct {
	mock.Mock
}

func (m *MockLookupLogRepository) Insert(ctx context.Context, entry *domain.LookupLogEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockLookupLogRepository) ListByClient(ctx context.Context, clientID uuid.UUID, from, to *time.Time, msisdnFilter, sourceFilter string, page, pageSize int) ([]*domain.LookupLogEntry, int64, error) {
	args := m.Called(ctx, clientID, from, to, msisdnFilter, sourceFilter, page, pageSize)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.LookupLogEntry), args.Get(1).(int64), args.Error(2)
}

func (m *MockLookupLogRepository) CountByClient(ctx context.Context, clientID uuid.UUID) (int64, error) {
	args := m.Called(ctx, clientID)
	return args.Get(0).(int64), args.Error(1)
}

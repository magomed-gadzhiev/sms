package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/cascade/domain"
	"github.com/stretchr/testify/mock"
)

// MockDeliveryRepository is a mock implementation of domain.DeliveryRepository
type MockDeliveryRepository struct {
	mock.Mock
}

func (m *MockDeliveryRepository) Create(ctx context.Context, d *domain.Delivery) error {
	args := m.Called(ctx, d)
	return args.Error(0)
}

func (m *MockDeliveryRepository) Get(ctx context.Context, id uuid.UUID) (*domain.Delivery, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Delivery), args.Error(1)
}

func (m *MockDeliveryRepository) GetByClientID(ctx context.Context, id, clientID uuid.UUID) (*domain.Delivery, error) {
	args := m.Called(ctx, id, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Delivery), args.Error(1)
}

func (m *MockDeliveryRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DeliveryStatus, deliveredVia string) error {
	args := m.Called(ctx, id, status, deliveredVia)
	return args.Error(0)
}

func (m *MockDeliveryRepository) UpdateStatusCAS(ctx context.Context, id uuid.UUID, expectedStatus domain.DeliveryStatus, newStatus domain.DeliveryStatus, deliveredVia string) error {
	args := m.Called(ctx, id, expectedStatus, newStatus, deliveredVia)
	return args.Error(0)
}

func (m *MockDeliveryRepository) UpdateStep(ctx context.Context, id uuid.UUID, step int) error {
	args := m.Called(ctx, id, step)
	return args.Error(0)
}

func (m *MockDeliveryRepository) UpdateCost(ctx context.Context, id uuid.UUID, totalCost float64) error {
	args := m.Called(ctx, id, totalCost)
	return args.Error(0)
}

func (m *MockDeliveryRepository) List(ctx context.Context, filter domain.DeliveryFilter) ([]*domain.Delivery, int, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Delivery), args.Int(1), args.Error(2)
}

func (m *MockDeliveryRepository) HasActiveByStrategy(ctx context.Context, strategyID uuid.UUID) (bool, error) {
	args := m.Called(ctx, strategyID)
	return args.Bool(0), args.Error(1)
}

func (m *MockDeliveryRepository) Stats(ctx context.Context, filter domain.StatsFilter) (*domain.DeliveryStats, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DeliveryStats), args.Error(1)
}

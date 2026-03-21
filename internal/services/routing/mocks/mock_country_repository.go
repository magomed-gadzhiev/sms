package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockCountryRepository is a mock implementation of domain.CountryRepository
type MockCountryRepository struct {
	mock.Mock
}

func (m *MockCountryRepository) Create(ctx context.Context, country *domain.Country) error {
	args := m.Called(ctx, country)
	return args.Error(0)
}

func (m *MockCountryRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Country, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Country), args.Error(1)
}

func (m *MockCountryRepository) GetByISOCode(ctx context.Context, isoCode string) (*domain.Country, error) {
	args := m.Called(ctx, isoCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Country), args.Error(1)
}

func (m *MockCountryRepository) Update(ctx context.Context, country *domain.Country) error {
	args := m.Called(ctx, country)
	return args.Error(0)
}

func (m *MockCountryRepository) List(ctx context.Context, limit, offset int) ([]*domain.Country, int, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Country), args.Int(1), args.Error(2)
}

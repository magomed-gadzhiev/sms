package mocks

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockHLRProviderAdapter is a mock implementation of domain.HLRProviderAdapter
type MockHLRProviderAdapter struct {
	mock.Mock
}

func (m *MockHLRProviderAdapter) Lookup(ctx context.Context, msisdn string) (*domain.LookupResult, error) {
	args := m.Called(ctx, msisdn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LookupResult), args.Error(1)
}

func (m *MockHLRProviderAdapter) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockHLRProviderAdapter) Name() string {
	args := m.Called()
	return args.String(0)
}

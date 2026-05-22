package mocks

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockHLRCache is a mock implementation of domain.HLRCache
type MockHLRCache struct {
	mock.Mock
}

func (m *MockHLRCache) Get(ctx context.Context, msisdn string) (*domain.LookupResult, error) {
	args := m.Called(ctx, msisdn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LookupResult), args.Error(1)
}

func (m *MockHLRCache) Set(ctx context.Context, msisdn string, result *domain.LookupResult) error {
	args := m.Called(ctx, msisdn, result)
	return args.Error(0)
}

func (m *MockHLRCache) Delete(ctx context.Context, msisdn string) error {
	args := m.Called(ctx, msisdn)
	return args.Error(0)
}

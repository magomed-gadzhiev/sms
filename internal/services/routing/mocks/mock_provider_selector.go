package mocks

import (
	"context"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockProviderSelector is a mock implementation of application.ProviderSelector
type MockProviderSelector struct {
	mock.Mock
}

func (m *MockProviderSelector) SelectProvider(ctx context.Context, route *domain.Route, providerInfos []*domain.ProviderInfo) (*domain.ProviderInfo, error) {
	args := m.Called(ctx, route, providerInfos)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProviderInfo), args.Error(1)
}

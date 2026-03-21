package mocks

import (
	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/stretchr/testify/mock"
)

// MockAdapterFactory is a mock implementation of application.AdapterFactory
type MockAdapterFactory struct {
	mock.Mock
}

func (m *MockAdapterFactory) CreateAdapter(provider *domain.HLRProvider) (domain.HLRProviderAdapter, error) {
	args := m.Called(provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(domain.HLRProviderAdapter), args.Error(1)
}

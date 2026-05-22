package mocks

import (
	"github.com/stretchr/testify/mock"
)

// MockAPIKeyGenerator мок для интерфейса APIKeyGenerator
type MockAPIKeyGenerator struct {
	mock.Mock
}

func (m *MockAPIKeyGenerator) GenerateAPIKey() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockAPIKeyGenerator) GetKeyPrefix(key string) string {
	args := m.Called(key)
	return args.String(0)
}

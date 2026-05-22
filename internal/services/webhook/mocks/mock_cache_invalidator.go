package mocks

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockCacheInvalidator is a mock implementation of application.CacheInvalidator
type MockCacheInvalidator struct {
	mock.Mock
}

func (m *MockCacheInvalidator) InvalidateCache(clientID uuid.UUID) {
	m.Called(clientID)
}

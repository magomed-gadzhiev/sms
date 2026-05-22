package mocks

import (
	"github.com/stretchr/testify/mock"
)

// MockPasswordHasher мок для интерфейса PasswordHasher
type MockPasswordHasher struct {
	mock.Mock
}

func (m *MockPasswordHasher) HashPassword(password string) (string, error) {
	args := m.Called(password)
	return args.String(0), args.Error(1)
}

func (m *MockPasswordHasher) CheckPassword(password, hash string) bool {
	args := m.Called(password, hash)
	return args.Bool(0)
}

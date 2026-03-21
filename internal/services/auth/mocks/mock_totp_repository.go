package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	"github.com/stretchr/testify/mock"
)

// MockTOTPRepository мок для интерфейса TOTPRepository
type MockTOTPRepository struct {
	mock.Mock
}

func (m *MockTOTPRepository) SaveTOTPSecret(ctx context.Context, userID uuid.UUID, secretEncrypted []byte) error {
	args := m.Called(ctx, userID, secretEncrypted)
	return args.Error(0)
}

func (m *MockTOTPRepository) GetTOTPConfig(ctx context.Context, userID uuid.UUID) (*domain.TOTPConfig, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TOTPConfig), args.Error(1)
}

func (m *MockTOTPRepository) EnableTOTP(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockTOTPRepository) DisableTOTP(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockTOTPRepository) SaveRecoveryCodes(ctx context.Context, codes []*domain.TOTPRecoveryCode) error {
	args := m.Called(ctx, codes)
	return args.Error(0)
}

func (m *MockTOTPRepository) UseRecoveryCode(ctx context.Context, userID uuid.UUID, codeHash string) error {
	args := m.Called(ctx, userID, codeHash)
	return args.Error(0)
}

func (m *MockTOTPRepository) DeleteRecoveryCodes(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

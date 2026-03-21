package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/stretchr/testify/mock"
)

type MockTarificationLogRepository struct {
	mock.Mock
}

func (m *MockTarificationLogRepository) Create(ctx context.Context, log *domain.TarificationLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockTarificationLogRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.TarificationLog, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TarificationLog), args.Error(1)
}

func (m *MockTarificationLogRepository) GetByMessageID(ctx context.Context, messageID uuid.UUID) (*domain.TarificationLog, error) {
	args := m.Called(ctx, messageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TarificationLog), args.Error(1)
}

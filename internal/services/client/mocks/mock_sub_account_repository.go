package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/client/domain"
	"github.com/stretchr/testify/mock"
)

// MockSubAccountRepository is a mock implementation of SubAccountRepositoryInterface.
type MockSubAccountRepository struct {
	mock.Mock
}

func (m *MockSubAccountRepository) ListByParentID(ctx context.Context, parentID uuid.UUID) ([]*domain.Client, error) {
	args := m.Called(ctx, parentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Client), args.Error(1)
}

func (m *MockSubAccountRepository) CountByParentID(ctx context.Context, parentID uuid.UUID) (int, error) {
	args := m.Called(ctx, parentID)
	return args.Int(0), args.Error(1)
}

func (m *MockSubAccountRepository) GetSubAccount(ctx context.Context, subAccountID, parentID uuid.UUID) (*domain.Client, error) {
	args := m.Called(ctx, subAccountID, parentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Client), args.Error(1)
}

func (m *MockSubAccountRepository) DeleteSubAccount(ctx context.Context, subAccountID uuid.UUID) error {
	args := m.Called(ctx, subAccountID)
	return args.Error(0)
}

func (m *MockSubAccountRepository) ExistsByEmailUnderParent(ctx context.Context, parentID uuid.UUID, email string) (bool, error) {
	args := m.Called(ctx, parentID, email)
	return args.Bool(0), args.Error(1)
}

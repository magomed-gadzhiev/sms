package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/auth/domain"
	"github.com/stretchr/testify/mock"
)

// MockRoleRepository мок для интерфейса RoleRepository
type MockRoleRepository struct {
	mock.Mock
}

func (m *MockRoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoleRepository) GetByName(ctx context.Context, name string) (*domain.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoleRepository) GetByIDWithPermissions(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoleRepository) List(ctx context.Context, limit, offset int32) ([]*domain.Role, int32, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int32), args.Error(2)
	}
	return args.Get(0).([]*domain.Role), args.Get(1).(int32), args.Error(2)
}

func (m *MockRoleRepository) Create(ctx context.Context, name, description string, permissionIDs []uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, name, description, permissionIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoleRepository) Update(ctx context.Context, roleID uuid.UUID, name, description string, permissionIDs []uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, roleID, name, description, permissionIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoleRepository) Delete(ctx context.Context, roleID uuid.UUID) error {
	args := m.Called(ctx, roleID)
	return args.Error(0)
}

func (m *MockRoleRepository) ListAllPermissions(ctx context.Context) ([]domain.Permission, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Permission), args.Error(1)
}

func (m *MockRoleRepository) GetUserCount(ctx context.Context, roleID uuid.UUID) (int32, error) {
	args := m.Called(ctx, roleID)
	return args.Get(0).(int32), args.Error(1)
}

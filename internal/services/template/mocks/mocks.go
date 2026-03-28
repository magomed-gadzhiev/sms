package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/template/domain"
	"github.com/stretchr/testify/mock"
)

// MockTemplateRepository is a mock implementation of application.TemplateRepository.
type MockTemplateRepository struct {
	mock.Mock
}

func (m *MockTemplateRepository) Create(ctx context.Context, t *domain.Template) (*domain.Template, error) {
	args := m.Called(ctx, t)
	if v := args.Get(0); v != nil {
		return v.(*domain.Template), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTemplateRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Template, error) {
	args := m.Called(ctx, id, clientID)
	if v := args.Get(0); v != nil {
		return v.(*domain.Template), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTemplateRepository) GetByIDAdmin(ctx context.Context, id uuid.UUID) (*domain.Template, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*domain.Template), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTemplateRepository) ListByClientID(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Template, int, error) {
	args := m.Called(ctx, clientID, status, limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]*domain.Template), args.Int(1), args.Error(2)
	}
	return nil, args.Int(1), args.Error(2)
}

func (m *MockTemplateRepository) Update(ctx context.Context, t *domain.Template) (*domain.Template, error) {
	args := m.Called(ctx, t)
	if v := args.Get(0); v != nil {
		return v.(*domain.Template), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTemplateRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, rejectionReason string) (*domain.Template, error) {
	args := m.Called(ctx, id, status, rejectionReason)
	if v := args.Get(0); v != nil {
		return v.(*domain.Template), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTemplateRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	args := m.Called(ctx, id, clientID)
	return args.Error(0)
}

// MockAuditRepository is a mock implementation of application.AuditRepository.
type MockAuditRepository struct {
	mock.Mock
}

func (m *MockAuditRepository) Create(ctx context.Context, entry *domain.AuditEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockAuditRepository) ListByTemplateID(ctx context.Context, templateID uuid.UUID, limit, offset int) ([]*domain.AuditEntry, int, error) {
	args := m.Called(ctx, templateID, limit, offset)
	if v := args.Get(0); v != nil {
		return v.([]*domain.AuditEntry), args.Int(1), args.Error(2)
	}
	return nil, args.Int(1), args.Error(2)
}

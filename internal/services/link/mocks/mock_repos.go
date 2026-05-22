package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/smpp-server/smpp-server/internal/services/link/domain"
)

// MockLinkRepo mocks application.LinkRepo
type MockLinkRepo struct {
	mock.Mock
}

func (m *MockLinkRepo) CreateShortLink(ctx context.Context, link *domain.ShortLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *MockLinkRepo) GetByCode(ctx context.Context, code string) (*domain.ShortLink, error) {
	args := m.Called(ctx, code)
	if v := args.Get(0); v != nil {
		return v.(*domain.ShortLink), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockLinkRepo) GetClientActiveDomain(ctx context.Context, clientID uuid.UUID) (*domain.ClientDomain, error) {
	args := m.Called(ctx, clientID)
	if v := args.Get(0); v != nil {
		return v.(*domain.ClientDomain), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockDomainRepo mocks application.DomainRepo
type MockDomainRepo struct {
	mock.Mock
}

func (m *MockDomainRepo) Create(ctx context.Context, d *domain.ClientDomain) error {
	args := m.Called(ctx, d)
	return args.Error(0)
}

func (m *MockDomainRepo) ListByClient(ctx context.Context, clientID uuid.UUID) ([]*domain.ClientDomain, error) {
	args := m.Called(ctx, clientID)
	if v := args.Get(0); v != nil {
		return v.([]*domain.ClientDomain), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDomainRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockClickRepo mocks application.ClickRepo
type MockClickRepo struct {
	mock.Mock
}

func (m *MockClickRepo) Insert(ctx context.Context, event *domain.ClickEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

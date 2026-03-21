package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
	"github.com/stretchr/testify/mock"
)

// MockPricingRuleRepository is a mock implementation of domain.PricingRuleRepository.
type MockPricingRuleRepository struct {
	mock.Mock
}

func (m *MockPricingRuleRepository) Create(ctx context.Context, rule *domain.PricingRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockPricingRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PricingRule, error) {
	args := m.Called(ctx, id)
	var pr *domain.PricingRule
	if v := args.Get(0); v != nil {
		pr = v.(*domain.PricingRule)
	}
	return pr, args.Error(1)
}

func (m *MockPricingRuleRepository) GetByClientID(ctx context.Context, clientID *uuid.UUID, activeOnly bool) ([]*domain.PricingRule, error) {
	args := m.Called(ctx, clientID, activeOnly)
	var rules []*domain.PricingRule
	if v := args.Get(0); v != nil {
		rules = v.([]*domain.PricingRule)
	}
	return rules, args.Error(1)
}

func (m *MockPricingRuleRepository) GetGlobalRules(ctx context.Context, activeOnly bool) ([]*domain.PricingRule, error) {
	args := m.Called(ctx, activeOnly)
	var rules []*domain.PricingRule
	if v := args.Get(0); v != nil {
		rules = v.([]*domain.PricingRule)
	}
	return rules, args.Error(1)
}

func (m *MockPricingRuleRepository) GetMatchingRule(ctx context.Context, clientID *uuid.UUID, destination string) (*domain.PricingRule, error) {
	args := m.Called(ctx, clientID, destination)
	var pr *domain.PricingRule
	if v := args.Get(0); v != nil {
		pr = v.(*domain.PricingRule)
	}
	return pr, args.Error(1)
}

func (m *MockPricingRuleRepository) Update(ctx context.Context, rule *domain.PricingRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockPricingRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

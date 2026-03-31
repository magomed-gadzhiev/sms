package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/billing/domain"
	"github.com/smpp-server/smpp-server/internal/services/billing/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPricingService(t *testing.T) {
	t.Run("GetPriceForDestination", func(t *testing.T) {
		t.Run("matching_rule_returns_price", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			destination := "+79161234567"

			rule := &domain.PricingRule{
				ID:                 uuid.New(),
				ClientID:           nil,
				DestinationPattern: "^\\+7916",
				PricePerMessage:    "0.050000",
				Currency:           "RUB",
				Priority:           10,
				Active:             true,
			}

			pricingRepo.On("GetMatchingRule", ctx, (*uuid.UUID)(nil), destination).Return(rule, nil)

			price, currency, err := svc.GetPriceForDestination(ctx, nil, destination)

			require.NoError(t, err)
			assert.Equal(t, "0.050000", price)
			assert.Equal(t, "RUB", currency)

			pricingRepo.AssertExpectations(t)
		})

		t.Run("no_rule_returns_default_price", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			destination := "+11234567890"

			pricingRepo.On("GetMatchingRule", ctx, (*uuid.UUID)(nil), destination).Return(nil, domain.ErrPricingRuleNotFound)

			price, currency, err := svc.GetPriceForDestination(ctx, nil, destination)

			require.NoError(t, err)
			assert.Equal(t, "0.01", price)
			assert.Equal(t, "RUB", currency)

			pricingRepo.AssertExpectations(t)
		})

		t.Run("client_override_returns_client_specific_price", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			clientID := uuid.New()
			destination := "+79161234567"

			clientRule := &domain.PricingRule{
				ID:                 uuid.New(),
				ClientID:           &clientID,
				DestinationPattern: "^\\+7",
				PricePerMessage:    "0.030000",
				Currency: "EUR",
				Priority:           20,
				Active:             true,
			}

			pricingRepo.On("GetMatchingRule", ctx, &clientID, destination).Return(clientRule, nil)

			price, currency, err := svc.GetPriceForDestination(ctx, &clientID, destination)

			require.NoError(t, err)
			assert.Equal(t, "0.030000", price)
			assert.Equal(t, "EUR", currency)

			pricingRepo.AssertExpectations(t)
		})

		t.Run("repository_error_propagated", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			destination := "+79161234567"

			pricingRepo.On("GetMatchingRule", ctx, (*uuid.UUID)(nil), destination).Return(nil, assert.AnError)

			price, currency, err := svc.GetPriceForDestination(ctx, nil, destination)

			require.Error(t, err)
			assert.Empty(t, price)
			assert.Empty(t, currency)
			assert.Contains(t, err.Error(), "failed to get pricing rule")

			pricingRepo.AssertExpectations(t)
		})
	})

	t.Run("GetPricingRules", func(t *testing.T) {
		t.Run("returns_rules_for_client", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			clientID := uuid.New()

			expectedRules := []*domain.PricingRule{
				{ID: uuid.New(), ClientID: &clientID, DestinationPattern: "^\\+7", PricePerMessage: "0.05", Currency: "RUB"},
				{ID: uuid.New(), ClientID: &clientID, DestinationPattern: "^\\+1", PricePerMessage: "0.03", Currency: "RUB"},
			}

			pricingRepo.On("GetByClientID", ctx, &clientID, false).Return(expectedRules, nil)

			rules, err := svc.GetPricingRules(ctx, &clientID)

			require.NoError(t, err)
			assert.Len(t, rules, 2)
			assert.Equal(t, expectedRules[0].ID, rules[0].ID)
			assert.Equal(t, expectedRules[1].ID, rules[1].ID)

			pricingRepo.AssertExpectations(t)
		})

		t.Run("returns_global_rules_when_nil_client", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			expectedRules := []*domain.PricingRule{
				{ID: uuid.New(), ClientID: nil, DestinationPattern: "^\\+", PricePerMessage: "0.01", Currency: "RUB"},
			}

			pricingRepo.On("GetByClientID", ctx, (*uuid.UUID)(nil), false).Return(expectedRules, nil)

			rules, err := svc.GetPricingRules(ctx, nil)

			require.NoError(t, err)
			assert.Len(t, rules, 1)

			pricingRepo.AssertExpectations(t)
		})
	})

	t.Run("CreatePricingRule", func(t *testing.T) {
		t.Run("creates_active_rule", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			clientID := uuid.New()

			pricingRepo.On("Create", ctx, mock.AnythingOfType("*domain.PricingRule")).Return(nil)

			rule, err := svc.CreatePricingRule(ctx, &clientID, "^\\+7", "0.050000", "RUB", 10, true)

			require.NoError(t, err)
			require.NotNil(t, rule)
			assert.Equal(t, &clientID, rule.ClientID)
			assert.Equal(t, "^\\+7", rule.DestinationPattern)
			assert.Equal(t, "0.050000", rule.PricePerMessage)
			assert.Equal(t, "RUB", rule.Currency)
			assert.Equal(t, 10, rule.Priority)
			assert.True(t, rule.Active)

			pricingRepo.AssertExpectations(t)
		})

		t.Run("creates_inactive_rule", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			pricingRepo.On("Create", ctx, mock.AnythingOfType("*domain.PricingRule")).Return(nil)

			rule, err := svc.CreatePricingRule(ctx, nil, "^\\+1", "0.020000", "RUB", 5, false)

			require.NoError(t, err)
			require.NotNil(t, rule)
			assert.Nil(t, rule.ClientID)
			assert.False(t, rule.Active)

			pricingRepo.AssertExpectations(t)
		})

		t.Run("validation_error_empty_pattern", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			rule, err := svc.CreatePricingRule(ctx, nil, "", "0.050000", "RUB", 10, true)

			require.Error(t, err)
			assert.Nil(t, rule)
			assert.Contains(t, err.Error(), "invalid pricing rule")

			pricingRepo.AssertNotCalled(t, "Create")
		})
	})

	t.Run("DeletePricingRule", func(t *testing.T) {
		t.Run("deletes_successfully", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			ruleID := uuid.New()

			pricingRepo.On("Delete", ctx, ruleID).Return(nil)

			err := svc.DeletePricingRule(ctx, ruleID)

			require.NoError(t, err)

			pricingRepo.AssertExpectations(t)
		})

		t.Run("delete_not_found", func(t *testing.T) {
			pricingRepo := new(mocks.MockPricingRuleRepository)
			svc := NewPricingService(pricingRepo)
			ctx := context.Background()

			ruleID := uuid.New()

			pricingRepo.On("Delete", ctx, ruleID).Return(domain.ErrPricingRuleNotFound)

			err := svc.DeletePricingRule(ctx, ruleID)

			require.Error(t, err)

			pricingRepo.AssertExpectations(t)
		})
	})
}

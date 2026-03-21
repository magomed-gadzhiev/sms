package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	"github.com/smpp-server/smpp-server/internal/services/tarification/mocks"
)

type planFixtures struct {
	planRepo    *mocks.MockTariffPlanRepository
	periodRepo  *mocks.MockTariffPeriodRepository
	tierRepo    *mocks.MockTariffTierRepository
	pricingRepo *mocks.MockPricingPeriodRepository
	prepaidRepo *mocks.MockPrepaidFeeRepository
	service     *application.TariffPlanService
}

func newPlanFixtures() *planFixtures {
	f := &planFixtures{
		planRepo:    new(mocks.MockTariffPlanRepository),
		periodRepo:  new(mocks.MockTariffPeriodRepository),
		tierRepo:    new(mocks.MockTariffTierRepository),
		pricingRepo: new(mocks.MockPricingPeriodRepository),
		prepaidRepo: new(mocks.MockPrepaidFeeRepository),
	}
	f.service = application.NewTariffPlanService(
		f.planRepo, f.periodRepo, f.tierRepo, f.pricingRepo, f.prepaidRepo,
	)
	return f
}

func TestTariffPlanService(t *testing.T) {
	t.Run("CreatePlan", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			operatorID := uuid.New()

			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, operatorID, domain.CategoryShared).
				Return(nil, domain.ErrTariffPlanNotFound)
			f.planRepo.On("Create", ctx, mock.AnythingOfType("*domain.TariffPlan")).
				Return(nil)

			plan, err := f.service.CreatePlan(ctx, operatorID, domain.CategoryShared, domain.StrategyFixed)

			require.NoError(t, err)
			require.NotNil(t, plan)
			assert.Equal(t, operatorID, plan.OperatorID)
			assert.Equal(t, domain.CategoryShared, plan.SenderCategory)
			assert.Equal(t, domain.StrategyFixed, plan.Strategy)
			assert.True(t, plan.Active)
			f.planRepo.AssertExpectations(t)
		})

		t.Run("duplicate_active_plan", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			operatorID := uuid.New()

			existing := &domain.TariffPlan{
				ID:         uuid.New(),
				OperatorID: operatorID,
				Active:     true,
			}
			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, operatorID, domain.CategoryShared).
				Return(existing, nil)

			plan, err := f.service.CreatePlan(ctx, operatorID, domain.CategoryShared, domain.StrategyFixed)

			assert.Nil(t, plan)
			assert.ErrorIs(t, err, domain.ErrTariffPlanDuplicate)
		})

		t.Run("invalid_strategy", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			operatorID := uuid.New()

			plan, err := f.service.CreatePlan(ctx, operatorID, domain.CategoryShared, "bad_strategy")

			assert.Nil(t, plan)
			assert.Error(t, err)
		})

		t.Run("repo_create_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			operatorID := uuid.New()

			f.planRepo.On("GetActiveByOperatorAndCategory", ctx, operatorID, domain.CategoryShared).
				Return(nil, domain.ErrTariffPlanNotFound)
			f.planRepo.On("Create", ctx, mock.AnythingOfType("*domain.TariffPlan")).
				Return(errors.New("db error"))

			plan, err := f.service.CreatePlan(ctx, operatorID, domain.CategoryShared, domain.StrategyFixed)

			assert.Nil(t, plan)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create tariff plan")
		})
	})

	t.Run("UpdatePlan", func(t *testing.T) {
		t.Run("activate_success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			existing := &domain.TariffPlan{
				ID:     planID,
				Active: false,
			}
			f.planRepo.On("GetByID", ctx, planID).Return(existing, nil)
			f.planRepo.On("Update", ctx, mock.AnythingOfType("*domain.TariffPlan")).Return(nil)

			plan, err := f.service.UpdatePlan(ctx, planID, true)

			require.NoError(t, err)
			require.NotNil(t, plan)
			assert.True(t, plan.Active)
			f.planRepo.AssertExpectations(t)
		})

		t.Run("deactivate_success_no_active_period", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			existing := &domain.TariffPlan{ID: planID, Active: true}
			f.planRepo.On("GetByID", ctx, planID).Return(existing, nil)
			f.periodRepo.On("HasActivePeriod", ctx, planID, mock.AnythingOfType("time.Time")).
				Return(false, nil)
			f.planRepo.On("Update", ctx, mock.AnythingOfType("*domain.TariffPlan")).Return(nil)

			plan, err := f.service.UpdatePlan(ctx, planID, false)

			require.NoError(t, err)
			assert.False(t, plan.Active)
			f.planRepo.AssertExpectations(t)
			f.periodRepo.AssertExpectations(t)
		})

		t.Run("deactivate_fails_with_active_period", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			existing := &domain.TariffPlan{ID: planID, Active: true}
			f.planRepo.On("GetByID", ctx, planID).Return(existing, nil)
			f.periodRepo.On("HasActivePeriod", ctx, planID, mock.AnythingOfType("time.Time")).
				Return(true, nil)

			plan, err := f.service.UpdatePlan(ctx, planID, false)

			assert.Nil(t, plan)
			assert.ErrorIs(t, err, domain.ErrTariffPlanHasActivePeriod)
		})

		t.Run("get_plan_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			f.planRepo.On("GetByID", ctx, planID).Return(nil, errors.New("not found"))

			plan, err := f.service.UpdatePlan(ctx, planID, true)

			assert.Nil(t, plan)
			assert.Error(t, err)
		})

		t.Run("has_active_period_check_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			existing := &domain.TariffPlan{ID: planID, Active: true}
			f.planRepo.On("GetByID", ctx, planID).Return(existing, nil)
			f.periodRepo.On("HasActivePeriod", ctx, planID, mock.AnythingOfType("time.Time")).
				Return(false, errors.New("db error"))

			plan, err := f.service.UpdatePlan(ctx, planID, false)

			assert.Nil(t, plan)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to check active period")
		})

		t.Run("update_repo_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			existing := &domain.TariffPlan{ID: planID, Active: false}
			f.planRepo.On("GetByID", ctx, planID).Return(existing, nil)
			f.planRepo.On("Update", ctx, mock.AnythingOfType("*domain.TariffPlan")).Return(errors.New("db error"))

			plan, err := f.service.UpdatePlan(ctx, planID, true)

			assert.Nil(t, plan)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to update tariff plan")
		})
	})

	t.Run("GetPlan", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			expected := &domain.TariffPlan{ID: planID, Active: true}
			f.planRepo.On("GetByID", ctx, planID).Return(expected, nil)

			plan, err := f.service.GetPlan(ctx, planID)

			require.NoError(t, err)
			assert.Equal(t, planID, plan.ID)
		})

		t.Run("not_found", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			f.planRepo.On("GetByID", ctx, planID).Return(nil, domain.ErrTariffPlanNotFound)

			plan, err := f.service.GetPlan(ctx, planID)

			assert.Nil(t, plan)
			assert.Error(t, err)
		})
	})

	t.Run("ListPlans", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			opID := uuid.New()

			plans := []*domain.TariffPlan{
				{ID: uuid.New(), OperatorID: opID},
				{ID: uuid.New(), OperatorID: opID},
			}
			f.planRepo.On("List", ctx, &opID, true, 10, 0).Return(plans, 2, nil)

			result, total, err := f.service.ListPlans(ctx, &opID, true, 10, 0)

			require.NoError(t, err)
			assert.Equal(t, 2, total)
			assert.Len(t, result, 2)
		})

		t.Run("error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()

			f.planRepo.On("List", ctx, (*uuid.UUID)(nil), false, 10, 0).Return(nil, 0, errors.New("db error"))

			result, total, err := f.service.ListPlans(ctx, nil, false, 10, 0)

			assert.Nil(t, result)
			assert.Equal(t, 0, total)
			assert.Error(t, err)
		})
	})

	t.Run("CreatePeriod", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()
			start := time.Now()
			end := start.AddDate(0, 1, 0)

			f.planRepo.On("GetByID", ctx, planID).Return(&domain.TariffPlan{ID: planID}, nil)
			f.periodRepo.On("Create", ctx, mock.AnythingOfType("*domain.TariffPeriod")).Return(nil)

			period, err := f.service.CreatePeriod(ctx, planID, start, end)

			require.NoError(t, err)
			require.NotNil(t, period)
			assert.Equal(t, planID, period.TariffPlanID)
			f.planRepo.AssertExpectations(t)
			f.periodRepo.AssertExpectations(t)
		})

		t.Run("plan_not_found", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()

			f.planRepo.On("GetByID", ctx, planID).Return(nil, domain.ErrTariffPlanNotFound)

			period, err := f.service.CreatePeriod(ctx, planID, time.Now(), time.Now().AddDate(0, 1, 0))

			assert.Nil(t, period)
			assert.Error(t, err)
		})

		t.Run("invalid_dates", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()
			now := time.Now()

			f.planRepo.On("GetByID", ctx, planID).Return(&domain.TariffPlan{ID: planID}, nil)

			// end before start
			period, err := f.service.CreatePeriod(ctx, planID, now, now.AddDate(0, -1, 0))

			assert.Nil(t, period)
			assert.Error(t, err)
		})

		t.Run("repo_create_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()
			start := time.Now()
			end := start.AddDate(0, 1, 0)

			f.planRepo.On("GetByID", ctx, planID).Return(&domain.TariffPlan{ID: planID}, nil)
			f.periodRepo.On("Create", ctx, mock.AnythingOfType("*domain.TariffPeriod")).Return(errors.New("db error"))

			period, err := f.service.CreatePeriod(ctx, planID, start, end)

			assert.Nil(t, period)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create tariff period")
		})
	})

	t.Run("CreateTier", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			periodID := uuid.New()

			// Period is in the future (not active)
			period := &domain.TariffPeriod{
				ID:        periodID,
				StartDate: time.Now().AddDate(0, 1, 0),
				EndDate:   time.Now().AddDate(0, 2, 0),
			}
			f.periodRepo.On("GetByID", ctx, periodID).Return(period, nil)
			f.tierRepo.On("Create", ctx, mock.AnythingOfType("*domain.TariffTier")).Return(nil)

			tier, err := f.service.CreateTier(ctx, periodID, 0, "0.050000")

			require.NoError(t, err)
			require.NotNil(t, tier)
			assert.Equal(t, 0, tier.FromCount)
			assert.Equal(t, "0.050000", tier.PricePerSegment)
		})

		t.Run("period_not_found", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			periodID := uuid.New()

			f.periodRepo.On("GetByID", ctx, periodID).Return(nil, domain.ErrTariffPeriodNotFound)

			tier, err := f.service.CreateTier(ctx, periodID, 0, "0.050000")

			assert.Nil(t, tier)
			assert.Error(t, err)
		})

		t.Run("period_is_active", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			periodID := uuid.New()

			// Period is currently active
			period := &domain.TariffPeriod{
				ID:        periodID,
				StartDate: time.Now().AddDate(0, -1, 0),
				EndDate:   time.Now().AddDate(0, 1, 0),
			}
			f.periodRepo.On("GetByID", ctx, periodID).Return(period, nil)

			tier, err := f.service.CreateTier(ctx, periodID, 0, "0.050000")

			assert.Nil(t, tier)
			assert.ErrorIs(t, err, domain.ErrTariffTierPeriodActive)
		})

		t.Run("invalid_tier_data", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			periodID := uuid.New()

			period := &domain.TariffPeriod{
				ID:        periodID,
				StartDate: time.Now().AddDate(0, 1, 0),
				EndDate:   time.Now().AddDate(0, 2, 0),
			}
			f.periodRepo.On("GetByID", ctx, periodID).Return(period, nil)

			// Empty price
			tier, err := f.service.CreateTier(ctx, periodID, 0, "")

			assert.Nil(t, tier)
			assert.Error(t, err)
		})

		t.Run("repo_create_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			periodID := uuid.New()

			period := &domain.TariffPeriod{
				ID:        periodID,
				StartDate: time.Now().AddDate(0, 1, 0),
				EndDate:   time.Now().AddDate(0, 2, 0),
			}
			f.periodRepo.On("GetByID", ctx, periodID).Return(period, nil)
			f.tierRepo.On("Create", ctx, mock.AnythingOfType("*domain.TariffTier")).Return(errors.New("db error"))

			tier, err := f.service.CreateTier(ctx, periodID, 0, "0.050000")

			assert.Nil(t, tier)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create tariff tier")
		})
	})

	t.Run("UpdateTier", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tierID := uuid.New()
			periodID := uuid.New()

			existing := &domain.TariffTier{
				ID:              tierID,
				TariffPeriodID:  periodID,
				FromCount:       0,
				PricePerSegment: "0.050000",
			}
			period := &domain.TariffPeriod{
				ID:        periodID,
				StartDate: time.Now().AddDate(0, 1, 0),
				EndDate:   time.Now().AddDate(0, 2, 0),
			}

			f.tierRepo.On("GetByID", ctx, tierID).Return(existing, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(period, nil)
			f.tierRepo.On("Update", ctx, mock.AnythingOfType("*domain.TariffTier")).Return(nil)

			tier, err := f.service.UpdateTier(ctx, tierID, 100, "0.080000")

			require.NoError(t, err)
			assert.Equal(t, 100, tier.FromCount)
			assert.Equal(t, "0.080000", tier.PricePerSegment)
		})

		t.Run("tier_not_found", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tierID := uuid.New()

			f.tierRepo.On("GetByID", ctx, tierID).Return(nil, domain.ErrTariffTierNotFound)

			tier, err := f.service.UpdateTier(ctx, tierID, 0, "0.050000")

			assert.Nil(t, tier)
			assert.Error(t, err)
		})

		t.Run("period_active_blocks_update", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tierID := uuid.New()
			periodID := uuid.New()

			existing := &domain.TariffTier{ID: tierID, TariffPeriodID: periodID}
			period := &domain.TariffPeriod{
				ID:        periodID,
				StartDate: time.Now().AddDate(0, -1, 0),
				EndDate:   time.Now().AddDate(0, 1, 0),
			}

			f.tierRepo.On("GetByID", ctx, tierID).Return(existing, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(period, nil)

			tier, err := f.service.UpdateTier(ctx, tierID, 0, "0.050000")

			assert.Nil(t, tier)
			assert.ErrorIs(t, err, domain.ErrTariffTierPeriodActive)
		})

		t.Run("period_get_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tierID := uuid.New()
			periodID := uuid.New()

			existing := &domain.TariffTier{ID: tierID, TariffPeriodID: periodID}
			f.tierRepo.On("GetByID", ctx, tierID).Return(existing, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(nil, errors.New("db error"))

			tier, err := f.service.UpdateTier(ctx, tierID, 0, "0.050000")

			assert.Nil(t, tier)
			assert.Error(t, err)
		})

		t.Run("invalid_tier_data", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tierID := uuid.New()
			periodID := uuid.New()

			existing := &domain.TariffTier{ID: tierID, TariffPeriodID: periodID}
			period := &domain.TariffPeriod{
				ID:        periodID,
				StartDate: time.Now().AddDate(0, 1, 0),
				EndDate:   time.Now().AddDate(0, 2, 0),
			}

			f.tierRepo.On("GetByID", ctx, tierID).Return(existing, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(period, nil)

			// Empty price is invalid
			tier, err := f.service.UpdateTier(ctx, tierID, 0, "")

			assert.Nil(t, tier)
			assert.Error(t, err)
		})

		t.Run("update_repo_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tierID := uuid.New()
			periodID := uuid.New()

			existing := &domain.TariffTier{ID: tierID, TariffPeriodID: periodID, FromCount: 0, PricePerSegment: "0.050000"}
			period := &domain.TariffPeriod{
				ID:        periodID,
				StartDate: time.Now().AddDate(0, 1, 0),
				EndDate:   time.Now().AddDate(0, 2, 0),
			}

			f.tierRepo.On("GetByID", ctx, tierID).Return(existing, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(period, nil)
			f.tierRepo.On("Update", ctx, mock.AnythingOfType("*domain.TariffTier")).Return(errors.New("db error"))

			tier, err := f.service.UpdateTier(ctx, tierID, 100, "0.080000")

			assert.Nil(t, tier)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to update tariff tier")
		})
	})

	t.Run("CreatePricingPeriod", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tariffPeriodID := uuid.New()

			tariffPeriod := &domain.TariffPeriod{
				ID:        tariffPeriodID,
				StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			}
			start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

			f.periodRepo.On("GetByID", ctx, tariffPeriodID).Return(tariffPeriod, nil)
			f.pricingRepo.On("Create", ctx, mock.AnythingOfType("*domain.PricingPeriod")).Return(nil)

			pp, err := f.service.CreatePricingPeriod(ctx, tariffPeriodID, start, end)

			require.NoError(t, err)
			require.NotNil(t, pp)
			assert.Equal(t, tariffPeriodID, pp.TariffPeriodID)
		})

		t.Run("tariff_period_not_found", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tariffPeriodID := uuid.New()

			f.periodRepo.On("GetByID", ctx, tariffPeriodID).Return(nil, domain.ErrTariffPeriodNotFound)

			pp, err := f.service.CreatePricingPeriod(ctx, tariffPeriodID, time.Now(), time.Now().AddDate(0, 1, 0))

			assert.Nil(t, pp)
			assert.Error(t, err)
		})

		t.Run("out_of_bounds", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tariffPeriodID := uuid.New()

			tariffPeriod := &domain.TariffPeriod{
				ID:        tariffPeriodID,
				StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
			}
			// Pricing period extends beyond tariff period
			start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)

			f.periodRepo.On("GetByID", ctx, tariffPeriodID).Return(tariffPeriod, nil)

			pp, err := f.service.CreatePricingPeriod(ctx, tariffPeriodID, start, end)

			assert.Nil(t, pp)
			assert.ErrorIs(t, err, domain.ErrPricingPeriodOutOfBounds)
		})

		t.Run("repo_create_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			tariffPeriodID := uuid.New()

			tariffPeriod := &domain.TariffPeriod{
				ID:        tariffPeriodID,
				StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			}
			start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

			f.periodRepo.On("GetByID", ctx, tariffPeriodID).Return(tariffPeriod, nil)
			f.pricingRepo.On("Create", ctx, mock.AnythingOfType("*domain.PricingPeriod")).Return(errors.New("db error"))

			pp, err := f.service.CreatePricingPeriod(ctx, tariffPeriodID, start, end)

			assert.Nil(t, pp)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create pricing period")
		})
	})

	t.Run("CreatePrepaidFee", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()
			periodID := uuid.New()

			f.planRepo.On("GetByID", ctx, planID).Return(&domain.TariffPlan{ID: planID}, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(&domain.TariffPeriod{ID: periodID}, nil)
			f.prepaidRepo.On("Create", ctx, mock.AnythingOfType("*domain.PrepaidFee")).Return(nil)

			fee, err := f.service.CreatePrepaidFee(ctx, planID, periodID, "100.00", "USD")

			require.NoError(t, err)
			require.NotNil(t, fee)
			assert.Equal(t, "100.00", fee.Amount)
			assert.Equal(t, "USD", fee.Currency)
			assert.False(t, fee.Charged)
		})

		t.Run("plan_not_found", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()
			periodID := uuid.New()

			f.planRepo.On("GetByID", ctx, planID).Return(nil, domain.ErrTariffPlanNotFound)

			fee, err := f.service.CreatePrepaidFee(ctx, planID, periodID, "100.00", "USD")

			assert.Nil(t, fee)
			assert.Error(t, err)
		})

		t.Run("period_not_found", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()
			periodID := uuid.New()

			f.planRepo.On("GetByID", ctx, planID).Return(&domain.TariffPlan{ID: planID}, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(nil, domain.ErrTariffPeriodNotFound)

			fee, err := f.service.CreatePrepaidFee(ctx, planID, periodID, "100.00", "USD")

			assert.Nil(t, fee)
			assert.Error(t, err)
		})

		t.Run("invalid_fee_data", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()
			periodID := uuid.New()

			f.planRepo.On("GetByID", ctx, planID).Return(&domain.TariffPlan{ID: planID}, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(&domain.TariffPeriod{ID: periodID}, nil)

			// Empty currency
			fee, err := f.service.CreatePrepaidFee(ctx, planID, periodID, "100.00", "")

			assert.Nil(t, fee)
			assert.Error(t, err)
		})

		t.Run("repo_create_error", func(t *testing.T) {
			f := newPlanFixtures()
			ctx := context.Background()
			planID := uuid.New()
			periodID := uuid.New()

			f.planRepo.On("GetByID", ctx, planID).Return(&domain.TariffPlan{ID: planID}, nil)
			f.periodRepo.On("GetByID", ctx, periodID).Return(&domain.TariffPeriod{ID: periodID}, nil)
			f.prepaidRepo.On("Create", ctx, mock.AnythingOfType("*domain.PrepaidFee")).Return(errors.New("db error"))

			fee, err := f.service.CreatePrepaidFee(ctx, planID, periodID, "100.00", "USD")

			assert.Nil(t, fee)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create prepaid fee")
		})
	})
}

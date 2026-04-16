//go:build functional

package functional_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/application"
	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
	tariffMocks "github.com/smpp-server/smpp-server/internal/services/tarification/mocks"
)

func TestTariffPlanLifecycle(t *testing.T) {
	skipIfNoDB(t)

	operatorID := uuid.New()
	ctx := context.Background()

	planRepo := &tariffMocks.MockTariffPlanRepository{}
	periodRepo := &tariffMocks.MockTariffPeriodRepository{}
	tierRepo := &tariffMocks.MockTariffTierRepository{}
	pricingRepo := &tariffMocks.MockPricingPeriodRepository{}
	prepaidRepo := &tariffMocks.MockPrepaidFeeRepository{}

	svc := application.NewTariffPlanService(planRepo, periodRepo, tierRepo, pricingRepo, prepaidRepo)

	t.Run("CreatePlan", func(t *testing.T) {
		// No existing active plan for this operator + category.
		planRepo.On("GetActiveByOperatorAndCategory", mock.Anything, operatorID, domain.CategoryShared).
			Return(nil, domain.ErrTariffPlanNotFound).Once()
		planRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TariffPlan")).
			Return(nil).Once()

		plan, err := svc.CreatePlan(ctx, operatorID, domain.CategoryShared, domain.StrategyFixed)
		require.NoError(t, err)
		require.NotNil(t, plan)

		assert.Equal(t, operatorID, plan.OperatorID)
		assert.Equal(t, domain.CategoryShared, plan.SenderCategory)
		assert.Equal(t, domain.StrategyFixed, plan.Strategy)
		assert.True(t, plan.Active)

		planRepo.AssertExpectations(t)
	})

	t.Run("CreateDuplicatePlanFails", func(t *testing.T) {
		existingPlan := domain.NewTariffPlan(operatorID, domain.CategoryPaidRegistered, domain.StrategyThreshold)
		planRepo.On("GetActiveByOperatorAndCategory", mock.Anything, operatorID, domain.CategoryPaidRegistered).
			Return(existingPlan, nil).Once()

		_, err := svc.CreatePlan(ctx, operatorID, domain.CategoryPaidRegistered, domain.StrategyThreshold)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrTariffPlanDuplicate)

		planRepo.AssertExpectations(t)
	})

	t.Run("CreatePlanInvalidCategoryFails", func(t *testing.T) {
		_, err := svc.CreatePlan(ctx, operatorID, "invalid_category", domain.StrategyFixed)
		require.Error(t, err)
	})

	t.Run("CreatePlanInvalidStrategyFails", func(t *testing.T) {
		_, err := svc.CreatePlan(ctx, operatorID, domain.CategoryShared, "invalid_strategy")
		require.Error(t, err)
	})
}

func TestTariffPlanActivation(t *testing.T) {
	skipIfNoDB(t)

	operatorID := uuid.New()
	ctx := context.Background()

	planRepo := &tariffMocks.MockTariffPlanRepository{}
	periodRepo := &tariffMocks.MockTariffPeriodRepository{}
	tierRepo := &tariffMocks.MockTariffTierRepository{}
	pricingRepo := &tariffMocks.MockPricingPeriodRepository{}
	prepaidRepo := &tariffMocks.MockPrepaidFeeRepository{}

	svc := application.NewTariffPlanService(planRepo, periodRepo, tierRepo, pricingRepo, prepaidRepo)

	t.Run("DeactivatePlanWithoutActivePeriod", func(t *testing.T) {
		plan := domain.NewTariffPlan(operatorID, domain.CategoryShared, domain.StrategyFixed)

		planRepo.On("GetByID", mock.Anything, plan.ID).
			Return(plan, nil).Once()
		periodRepo.On("HasActivePeriod", mock.Anything, plan.ID, mock.AnythingOfType("time.Time")).
			Return(false, nil).Once()
		planRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.TariffPlan")).
			Return(nil).Once()

		updated, err := svc.UpdatePlan(ctx, plan.ID, false)
		require.NoError(t, err)
		assert.False(t, updated.Active)

		planRepo.AssertExpectations(t)
		periodRepo.AssertExpectations(t)
	})

	t.Run("DeactivatePlanWithActivePeriodFails", func(t *testing.T) {
		plan := domain.NewTariffPlan(operatorID, domain.CategoryShared, domain.StrategyFixed)

		planRepo.On("GetByID", mock.Anything, plan.ID).
			Return(plan, nil).Once()
		periodRepo.On("HasActivePeriod", mock.Anything, plan.ID, mock.AnythingOfType("time.Time")).
			Return(true, nil).Once()

		_, err := svc.UpdatePlan(ctx, plan.ID, false)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrTariffPlanHasActivePeriod)

		planRepo.AssertExpectations(t)
		periodRepo.AssertExpectations(t)
	})

	t.Run("ReactivatePlan", func(t *testing.T) {
		plan := domain.NewTariffPlan(operatorID, domain.CategoryShared, domain.StrategyFixed)
		plan.Active = false

		planRepo.On("GetByID", mock.Anything, plan.ID).
			Return(plan, nil).Once()
		planRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.TariffPlan")).
			Return(nil).Once()

		updated, err := svc.UpdatePlan(ctx, plan.ID, true)
		require.NoError(t, err)
		assert.True(t, updated.Active)

		planRepo.AssertExpectations(t)
	})
}

func TestTariffPeriodAndTiers(t *testing.T) {
	skipIfNoDB(t)

	operatorID := uuid.New()
	ctx := context.Background()

	planRepo := &tariffMocks.MockTariffPlanRepository{}
	periodRepo := &tariffMocks.MockTariffPeriodRepository{}
	tierRepo := &tariffMocks.MockTariffTierRepository{}
	pricingRepo := &tariffMocks.MockPricingPeriodRepository{}
	prepaidRepo := &tariffMocks.MockPrepaidFeeRepository{}

	svc := application.NewTariffPlanService(planRepo, periodRepo, tierRepo, pricingRepo, prepaidRepo)

	t.Run("CreatePeriod", func(t *testing.T) {
		plan := domain.NewTariffPlan(operatorID, domain.CategoryShared, domain.StrategyFixed)

		planRepo.On("GetByID", mock.Anything, plan.ID).
			Return(plan, nil).Once()
		periodRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TariffPeriod")).
			Return(nil).Once()

		startDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

		period, err := svc.CreatePeriod(ctx, plan.ID, startDate, &endDate)
		require.NoError(t, err)
		require.NotNil(t, period)

		assert.Equal(t, plan.ID, period.TariffPlanID)
		assert.Equal(t, startDate, period.StartDate)
		assert.Equal(t, &endDate, period.EndDate)

		planRepo.AssertExpectations(t)
		periodRepo.AssertExpectations(t)
	})

	t.Run("CreatePeriodInvalidDatesFails", func(t *testing.T) {
		plan := domain.NewTariffPlan(operatorID, domain.CategoryShared, domain.StrategyFixed)

		planRepo.On("GetByID", mock.Anything, plan.ID).
			Return(plan, nil).Once()

		startDate := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

		_, err := svc.CreatePeriod(ctx, plan.ID, startDate, &endDate)
		require.Error(t, err)

		planRepo.AssertExpectations(t)
	})

	t.Run("CreateTierForFuturePeriod", func(t *testing.T) {
		// Period is in the future (not active).
		futureStart := time.Now().AddDate(0, 1, 0)
		futureEnd := time.Now().AddDate(0, 3, 0)
		period := domain.NewTariffPeriod(uuid.New(), futureStart, &futureEnd)

		periodRepo.On("GetByID", mock.Anything, period.ID).
			Return(period, nil).Once()
		tierRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TariffTier")).
			Return(nil).Once()

		tier, err := svc.CreateTier(ctx, period.ID, 0, "0.050000")
		require.NoError(t, err)
		require.NotNil(t, tier)

		assert.Equal(t, period.ID, tier.TariffPeriodID)
		assert.Equal(t, 0, tier.FromCount)
		assert.Equal(t, "0.050000", tier.PricePerSegment)

		periodRepo.AssertExpectations(t)
		tierRepo.AssertExpectations(t)
	})

	t.Run("CreateTierForActivePeriodFails", func(t *testing.T) {
		// Period is currently active.
		activeEnd := time.Now().AddDate(0, 0, 10)
		activePeriod := domain.NewTariffPeriod(uuid.New(),
			time.Now().AddDate(0, 0, -10),
			&activeEnd)

		periodRepo.On("GetByID", mock.Anything, activePeriod.ID).
			Return(activePeriod, nil).Once()

		_, err := svc.CreateTier(ctx, activePeriod.ID, 0, "0.050000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrTariffTierPeriodActive)

		periodRepo.AssertExpectations(t)
	})

	t.Run("UpdateTierForFuturePeriod", func(t *testing.T) {
		futureStart := time.Now().AddDate(0, 1, 0)
		futureEnd := time.Now().AddDate(0, 3, 0)
		period := domain.NewTariffPeriod(uuid.New(), futureStart, &futureEnd)

		tier := domain.NewTariffTier(period.ID, 0, "0.050000")

		tierRepo.On("GetByID", mock.Anything, tier.ID).
			Return(tier, nil).Once()
		periodRepo.On("GetByID", mock.Anything, period.ID).
			Return(period, nil).Once()
		tierRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.TariffTier")).
			Return(nil).Once()

		updated, err := svc.UpdateTier(ctx, tier.ID, 1000, "0.040000")
		require.NoError(t, err)
		assert.Equal(t, 1000, updated.FromCount)
		assert.Equal(t, "0.040000", updated.PricePerSegment)

		tierRepo.AssertExpectations(t)
		periodRepo.AssertExpectations(t)
	})

	t.Run("UpdateTierForActivePeriodFails", func(t *testing.T) {
		activeEnd := time.Now().AddDate(0, 0, 10)
		activePeriod := domain.NewTariffPeriod(uuid.New(),
			time.Now().AddDate(0, 0, -10),
			&activeEnd)

		tier := domain.NewTariffTier(activePeriod.ID, 0, "0.050000")

		tierRepo.On("GetByID", mock.Anything, tier.ID).
			Return(tier, nil).Once()
		periodRepo.On("GetByID", mock.Anything, activePeriod.ID).
			Return(activePeriod, nil).Once()

		_, err := svc.UpdateTier(ctx, tier.ID, 1000, "0.040000")
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrTariffTierPeriodActive)

		tierRepo.AssertExpectations(t)
		periodRepo.AssertExpectations(t)
	})
}

func TestTariffPricingPeriod(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()

	planRepo := &tariffMocks.MockTariffPlanRepository{}
	periodRepo := &tariffMocks.MockTariffPeriodRepository{}
	tierRepo := &tariffMocks.MockTariffTierRepository{}
	pricingRepo := &tariffMocks.MockPricingPeriodRepository{}
	prepaidRepo := &tariffMocks.MockPrepaidFeeRepository{}

	svc := application.NewTariffPlanService(planRepo, periodRepo, tierRepo, pricingRepo, prepaidRepo)

	t.Run("CreatePricingPeriodWithinBounds", func(t *testing.T) {
		tariffStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		tariffEnd := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
		tariffPeriod := domain.NewTariffPeriod(uuid.New(), tariffStart, &tariffEnd)

		periodRepo.On("GetByID", mock.Anything, tariffPeriod.ID).
			Return(tariffPeriod, nil).Once()
		pricingRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.PricingPeriod")).
			Return(nil).Once()

		pricingStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		pricingEnd := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)

		pp, err := svc.CreatePricingPeriod(ctx, tariffPeriod.ID, pricingStart, pricingEnd)
		require.NoError(t, err)
		require.NotNil(t, pp)

		assert.Equal(t, tariffPeriod.ID, pp.TariffPeriodID)
		assert.Equal(t, pricingStart, pp.StartDate)
		assert.Equal(t, pricingEnd, pp.EndDate)

		periodRepo.AssertExpectations(t)
		pricingRepo.AssertExpectations(t)
	})

	t.Run("CreatePricingPeriodOutOfBoundsFails", func(t *testing.T) {
		tariffStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		tariffEnd := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
		tariffPeriod := domain.NewTariffPeriod(uuid.New(), tariffStart, &tariffEnd)

		periodRepo.On("GetByID", mock.Anything, tariffPeriod.ID).
			Return(tariffPeriod, nil).Once()

		// Pricing period extends beyond tariff period.
		pricingStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		pricingEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

		_, err := svc.CreatePricingPeriod(ctx, tariffPeriod.ID, pricingStart, pricingEnd)
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrPricingPeriodOutOfBounds)

		periodRepo.AssertExpectations(t)
	})
}

func TestTariffPrepaidFee(t *testing.T) {
	skipIfNoDB(t)

	ctx := context.Background()

	planRepo := &tariffMocks.MockTariffPlanRepository{}
	periodRepo := &tariffMocks.MockTariffPeriodRepository{}
	tierRepo := &tariffMocks.MockTariffTierRepository{}
	pricingRepo := &tariffMocks.MockPricingPeriodRepository{}
	prepaidRepo := &tariffMocks.MockPrepaidFeeRepository{}

	svc := application.NewTariffPlanService(planRepo, periodRepo, tierRepo, pricingRepo, prepaidRepo)

	t.Run("CreatePrepaidFee", func(t *testing.T) {
		operatorID := uuid.New()
		plan := domain.NewTariffPlan(operatorID, domain.CategoryShared, domain.StrategyPrepaidThreshold)
		prepaidEnd := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
		period := domain.NewTariffPeriod(plan.ID,
			time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			&prepaidEnd)

		planRepo.On("GetByID", mock.Anything, plan.ID).
			Return(plan, nil).Once()
		periodRepo.On("GetByID", mock.Anything, period.ID).
			Return(period, nil).Once()
		prepaidRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.PrepaidFee")).
			Return(nil).Once()

		fee, err := svc.CreatePrepaidFee(ctx, plan.ID, period.ID, "500.000000", "RUB")
		require.NoError(t, err)
		require.NotNil(t, fee)

		assert.Equal(t, plan.ID, fee.TariffPlanID)
		assert.Equal(t, period.ID, fee.TariffPeriodID)
		assert.Equal(t, "500.000000", fee.Amount)
		assert.Equal(t, "RUB", fee.Currency)
		assert.False(t, fee.Charged)

		planRepo.AssertExpectations(t)
		periodRepo.AssertExpectations(t)
		prepaidRepo.AssertExpectations(t)
	})
}

func TestTariffFullWorkflow(t *testing.T) {
	skipIfNoDB(t)

	operatorID := uuid.New()
	ctx := context.Background()

	planRepo := &tariffMocks.MockTariffPlanRepository{}
	periodRepo := &tariffMocks.MockTariffPeriodRepository{}
	tierRepo := &tariffMocks.MockTariffTierRepository{}
	pricingRepo := &tariffMocks.MockPricingPeriodRepository{}
	prepaidRepo := &tariffMocks.MockPrepaidFeeRepository{}

	svc := application.NewTariffPlanService(planRepo, periodRepo, tierRepo, pricingRepo, prepaidRepo)

	// Step 1: Create a tariff plan.
	planRepo.On("GetActiveByOperatorAndCategory", mock.Anything, operatorID, domain.CategoryShared).
		Return(nil, domain.ErrTariffPlanNotFound).Once()
	planRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TariffPlan")).
		Return(nil).Once()

	plan, err := svc.CreatePlan(ctx, operatorID, domain.CategoryShared, domain.StrategyThreshold)
	require.NoError(t, err)

	// Step 2: Create a tariff period.
	planRepo.On("GetByID", mock.Anything, plan.ID).
		Return(plan, nil)
	periodRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TariffPeriod")).
		Return(nil).Once()

	futureStart := time.Now().AddDate(0, 1, 0)
	futureEnd := time.Now().AddDate(0, 4, 0)
	period, err := svc.CreatePeriod(ctx, plan.ID, futureStart, futureEnd)
	require.NoError(t, err)

	// Step 3: Create tiered pricing for the period.
	periodRepo.On("GetByID", mock.Anything, period.ID).
		Return(period, nil)
	tierRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TariffTier")).
		Return(nil)

	// Tier 1: 0-999 messages at 0.05 per segment.
	tier1, err := svc.CreateTier(ctx, period.ID, 0, "0.050000")
	require.NoError(t, err)
	assert.Equal(t, 0, tier1.FromCount)

	// Tier 2: 1000+ messages at 0.03 per segment (volume discount).
	tier2, err := svc.CreateTier(ctx, period.ID, 1000, "0.030000")
	require.NoError(t, err)
	assert.Equal(t, 1000, tier2.FromCount)

	// Step 4: Create pricing period within tariff period.
	pricingRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.PricingPeriod")).
		Return(nil).Once()

	pricingStart := futureStart
	pricingEnd := futureStart.AddDate(0, 1, 0)
	pp, err := svc.CreatePricingPeriod(ctx, period.ID, pricingStart, pricingEnd)
	require.NoError(t, err)
	assert.Equal(t, period.ID, pp.TariffPeriodID)

	// Step 5: Create prepaid fee.
	prepaidRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.PrepaidFee")).
		Return(nil).Once()

	fee, err := svc.CreatePrepaidFee(ctx, plan.ID, period.ID, "1000.000000", "RUB")
	require.NoError(t, err)
	assert.Equal(t, "1000.000000", fee.Amount)
	assert.Equal(t, "RUB", fee.Currency)
	assert.False(t, fee.Charged)

	// Step 6: Verify plan retrieval.
	fetched, err := svc.GetPlan(ctx, plan.ID)
	require.NoError(t, err)
	assert.Equal(t, operatorID, fetched.OperatorID)
	assert.Equal(t, domain.CategoryShared, fetched.SenderCategory)
	assert.Equal(t, domain.StrategyThreshold, fetched.Strategy)
	assert.True(t, fetched.Active)

	// Step 7: List plans.
	planRepo.On("List", mock.Anything, (*uuid.UUID)(nil), true, 100, 0).
		Return([]*domain.TariffPlan{plan}, 1, nil).Once()

	plans, total, err := svc.ListPlans(ctx, nil, true, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, plans, 1)

	planRepo.AssertExpectations(t)
	periodRepo.AssertExpectations(t)
	tierRepo.AssertExpectations(t)
	pricingRepo.AssertExpectations(t)
	prepaidRepo.AssertExpectations(t)
}

package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewTariffPlan(t *testing.T) {
	t.Run("creates tariff plan with correct defaults", func(t *testing.T) {
		operatorID := uuid.New()
		plan := NewTariffPlan(operatorID, CategoryShared, StrategyFixed)

		assert.NotEqual(t, uuid.Nil, plan.ID)
		assert.Equal(t, operatorID, plan.OperatorID)
		assert.Equal(t, CategoryShared, plan.SenderCategory)
		assert.Equal(t, StrategyFixed, plan.Strategy)
		assert.True(t, plan.Active)
		assert.False(t, plan.CreatedAt.IsZero())
		assert.False(t, plan.UpdatedAt.IsZero())
	})
}

func TestTariffPlan_Validate(t *testing.T) {
	t.Run("valid plan returns nil", func(t *testing.T) {
		plan := NewTariffPlan(uuid.New(), CategoryShared, StrategyFixed)
		err := plan.Validate()
		assert.NoError(t, err)
	})

	t.Run("nil OperatorID returns error", func(t *testing.T) {
		plan := NewTariffPlan(uuid.Nil, CategoryShared, StrategyFixed)
		err := plan.Validate()
		assert.ErrorIs(t, err, ErrTariffPlanNotFound)
	})

	t.Run("valid categories are accepted", func(t *testing.T) {
		validCategories := []SenderCategory{
			CategoryShared,
			CategoryPaidRegistered,
			CategoryFreeRegistered,
		}
		for _, cat := range validCategories {
			plan := NewTariffPlan(uuid.New(), cat, StrategyFixed)
			err := plan.Validate()
			assert.NoError(t, err, "category %q should be valid", cat)
		}
	})

	t.Run("invalid category returns error", func(t *testing.T) {
		plan := NewTariffPlan(uuid.New(), SenderCategory("premium"), StrategyFixed)
		err := plan.Validate()
		assert.ErrorIs(t, err, ErrTariffPlanInvalidCategory)
	})

	t.Run("valid strategies are accepted", func(t *testing.T) {
		validStrategies := []TarificationStrategy{
			StrategyFixed,
			StrategyThreshold,
			StrategyThresholdRecalc,
			StrategyPrepaidThreshold,
		}
		for _, strategy := range validStrategies {
			plan := NewTariffPlan(uuid.New(), CategoryShared, strategy)
			err := plan.Validate()
			assert.NoError(t, err, "strategy %q should be valid", strategy)
		}
	})

	t.Run("invalid strategy returns error", func(t *testing.T) {
		plan := NewTariffPlan(uuid.New(), CategoryShared, TarificationStrategy("dynamic"))
		err := plan.Validate()
		assert.ErrorIs(t, err, ErrTariffPlanInvalidStrategy)
	})

	t.Run("nil OperatorID is checked before category", func(t *testing.T) {
		plan := NewTariffPlan(uuid.Nil, SenderCategory("invalid"), StrategyFixed)
		err := plan.Validate()
		// OperatorID check comes first
		assert.ErrorIs(t, err, ErrTariffPlanNotFound)
	})
}

func TestTarificationStrategy_Constants(t *testing.T) {
	t.Run("strategy constants have expected values", func(t *testing.T) {
		assert.Equal(t, TarificationStrategy("fixed"), StrategyFixed)
		assert.Equal(t, TarificationStrategy("threshold"), StrategyThreshold)
		assert.Equal(t, TarificationStrategy("threshold_recalc"), StrategyThresholdRecalc)
		assert.Equal(t, TarificationStrategy("prepaid_threshold"), StrategyPrepaidThreshold)
	})
}

func TestSenderCategory_Constants(t *testing.T) {
	t.Run("sender category constants have expected values", func(t *testing.T) {
		assert.Equal(t, SenderCategory("shared"), CategoryShared)
		assert.Equal(t, SenderCategory("paid_registered"), CategoryPaidRegistered)
		assert.Equal(t, SenderCategory("free_registered"), CategoryFreeRegistered)
	})
}

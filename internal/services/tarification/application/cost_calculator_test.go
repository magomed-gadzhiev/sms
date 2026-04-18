package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/tarification/domain"
)

func strPtr(s string) *string { return &s }

func newCalc(t *testing.T) *CostCalculator {
	t.Helper()
	cache, err := NewTiersCache(128)
	require.NoError(t, err)
	return NewCostCalculator(cache)
}

func TestCostCalculator_Fixed(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelFixed,
		PriceValue:   strPtr("1.5"),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	cost, err := calc.Calculate(rr, 0, 3)
	require.NoError(t, err)
	require.InDelta(t, 4.5, cost, 1e-9)
}

func TestCostCalculator_Fixed_ZeroSegments(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelFixed,
		PriceValue:   strPtr("2.0"),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	cost, err := calc.Calculate(rr, 0, 0)
	require.NoError(t, err)
	require.InDelta(t, 0.0, cost, 1e-9)
}

func TestCostCalculator_Fixed_NilPriceValue(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelFixed,
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	_, err := calc.Calculate(rr, 0, 1)
	require.Error(t, err)
}

func TestCostCalculator_Tiered_AllInFirstTier(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelTiered,
		TiersJSON:    []byte(`{"period":"calendar_month","tiers":[{"up_to":10000,"price":1.0},{"up_to":null,"price":0.5}]}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	cost, err := calc.Calculate(rr, 0, 100)
	require.NoError(t, err)
	require.InDelta(t, 100.0, cost, 1e-9)
}

func TestCostCalculator_Tiered_CrossingBoundary(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelTiered,
		TiersJSON:    []byte(`{"period":"calendar_month","tiers":[{"up_to":10000,"price":1.0},{"up_to":null,"price":0.5}]}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	// usageBefore=9950, segments=100 → 50 × 1.0 + 50 × 0.5 = 75
	cost, err := calc.Calculate(rr, 9950, 100)
	require.NoError(t, err)
	require.InDelta(t, 75.0, cost, 1e-9)
}

func TestCostCalculator_Tiered_FullyInSecondTier(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelTiered,
		TiersJSON:    []byte(`{"period":"calendar_month","tiers":[{"up_to":10000,"price":1.0},{"up_to":null,"price":0.5}]}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	// usageBefore=20000, segments=100 → 100 × 0.5 = 50
	cost, err := calc.Calculate(rr, 20000, 100)
	require.NoError(t, err)
	require.InDelta(t, 50.0, cost, 1e-9)
}

func TestCostCalculator_Tiered_ThreeTiers_CrossBoth(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelTiered,
		TiersJSON:    []byte(`{"period":"calendar_month","tiers":[{"up_to":100,"price":2.0},{"up_to":200,"price":1.0},{"up_to":null,"price":0.5}]}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	// usageBefore=50, segments=200 → 50×2.0 + 100×1.0 + 50×0.5 = 225
	cost, err := calc.Calculate(rr, 50, 200)
	require.NoError(t, err)
	require.InDelta(t, 225.0, cost, 1e-9)
}

func TestCostCalculator_Prepaid_WithinLimit(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelPrepaidThreshold,
		TiersJSON:    []byte(`{"period":"calendar_month","prepaid_amount":1000,"included_segments":5000,"overage_price":0.5}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	cost, err := calc.Calculate(rr, 4000, 500)
	require.NoError(t, err)
	require.InDelta(t, 0.0, cost, 1e-9)
}

func TestCostCalculator_Prepaid_CrossingLimit(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelPrepaidThreshold,
		TiersJSON:    []byte(`{"period":"calendar_month","prepaid_amount":1000,"included_segments":5000,"overage_price":0.5}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	// usageBefore=4900, segments=200 → 100 бесплатно, 100 × 0.5 = 50
	cost, err := calc.Calculate(rr, 4900, 200)
	require.NoError(t, err)
	require.InDelta(t, 50.0, cost, 1e-9)
}

func TestCostCalculator_Prepaid_FullyOver(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.ModelPrepaidThreshold,
		TiersJSON:    []byte(`{"period":"calendar_month","prepaid_amount":1000,"included_segments":5000,"overage_price":0.5}`),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	// usageBefore=6000, segments=100 → весь батч в overage: 100 × 0.5 = 50
	cost, err := calc.Calculate(rr, 6000, 100)
	require.NoError(t, err)
	require.InDelta(t, 50.0, cost, 1e-9)
}

func TestCostCalculator_UnknownModel(t *testing.T) {
	calc := newCalc(t)
	rr := &domain.ResolvedRule{
		PriceModel:   domain.PriceModelType("quantum"),
		SourceRuleID: uuid.New(),
		RulesVersion: 1,
	}
	_, err := calc.Calculate(rr, 0, 1)
	require.Error(t, err)
}

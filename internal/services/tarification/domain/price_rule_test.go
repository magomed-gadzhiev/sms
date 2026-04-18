package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func TestPriceRule_Validate_OK_PlatformFixed(t *testing.T) {
	p := PriceRule{
		ID:         uuid.New(),
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
		PriceValue: strPtr("1.5"),
	}
	require.NoError(t, p.Validate())
}

func TestPriceRule_Validate_OK_SubaccountTiered(t *testing.T) {
	owner := uuid.New()
	p := PriceRule{
		ID:         uuid.New(),
		OwnerType:  OwnerSubaccount,
		OwnerID:    &owner,
		ValidFrom:  time.Now(),
		PriceModel: ModelTiered,
		TiersJSON:  []byte(`{"tiers":[{"up_to":null,"price":1.0}]}`),
	}
	require.NoError(t, p.Validate())
}

func TestPriceRule_Validate_Fails_PlatformWithOwnerID(t *testing.T) {
	owner := uuid.New()
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		OwnerID:    &owner,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
		PriceValue: strPtr("1.0"),
	}
	require.ErrorIs(t, p.Validate(), ErrOwnerIDMismatch)
}

func TestPriceRule_Validate_Fails_AggregatorWithoutOwnerID(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerAggregator,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
		PriceValue: strPtr("1.0"),
	}
	require.ErrorIs(t, p.Validate(), ErrOwnerIDMismatch)
}

func TestPriceRule_Validate_Fails_FixedWithoutValue(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
	}
	require.ErrorIs(t, p.Validate(), ErrPriceSpecMismatch)
}

func TestPriceRule_Validate_Fails_FixedWithTiers(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelFixed,
		PriceValue: strPtr("1.0"),
		TiersJSON:  []byte(`{"tiers":[]}`),
	}
	require.ErrorIs(t, p.Validate(), ErrPriceSpecMismatch)
}

func TestPriceRule_Validate_Fails_TieredWithoutTiers(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelTiered,
	}
	require.ErrorIs(t, p.Validate(), ErrPriceSpecMismatch)
}

func TestPriceRule_Validate_Fails_TieredWithPriceValue(t *testing.T) {
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  time.Now(),
		PriceModel: ModelTiered,
		PriceValue: strPtr("1.0"),
		TiersJSON:  []byte(`{"tiers":[]}`),
	}
	require.ErrorIs(t, p.Validate(), ErrPriceSpecMismatch)
}

func TestPriceRule_Validate_Fails_ValidToBeforeFrom(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-time.Hour)
	p := PriceRule{
		OwnerType:  OwnerPlatform,
		ValidFrom:  now,
		ValidTo:    &earlier,
		PriceModel: ModelFixed,
		PriceValue: strPtr("1.0"),
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidPeriod)
}

func TestPriceRule_SpecificityBitmap(t *testing.T) {
	tests := []struct {
		name                                 string
		country, operator, category, traffic string
		expected                             int
	}{
		{"all empty", "", "", "", "", 0},
		{"country only", "RU", "", "", "", 1},
		{"operator only", "", "MTS", "", "", 2},
		{"country+operator", "RU", "MTS", "", "", 3},
		{"category only", "", "", "paid", "", 4},
		{"traffic only", "", "", "", "transactional", 8},
		{"all", "RU", "MTS", "paid", "transactional", 15},
		{"category + traffic beats country + operator", "", "", "paid", "transactional", 12},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := PriceRule{}
			if tc.country != "" {
				p.Country = strPtr(tc.country)
			}
			if tc.operator != "" {
				p.Operator = strPtr(tc.operator)
			}
			if tc.category != "" {
				p.SenderCategory = strPtr(tc.category)
			}
			if tc.traffic != "" {
				p.TrafficType = strPtr(tc.traffic)
			}
			require.Equal(t, tc.expected, p.SpecificityBitmap())
		})
	}
}

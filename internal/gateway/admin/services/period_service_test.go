package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/services"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

func TestComputeScopeKey(t *testing.T) {
	kz := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	bee := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	acme := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	cases := []struct {
		name     string
		dims     services.PeriodDimensions
		wantKey  string
		wantPrio int
	}{
		{
			name:     "global",
			dims:     services.PeriodDimensions{},
			wantKey:  "global",
			wantPrio: 0,
		},
		{
			name:     "country only",
			dims:     services.PeriodDimensions{CountryID: &kz},
			wantKey:  "country:" + kz.String(),
			wantPrio: 10,
		},
		{
			name:     "country + operator",
			dims:     services.PeriodDimensions{CountryID: &kz, OperatorID: &bee},
			wantKey:  "country:" + kz.String() + "|operator:" + bee.String(),
			wantPrio: 20,
		},
		{
			name:     "country + operator + sender_category",
			dims:     services.PeriodDimensions{CountryID: &kz, OperatorID: &bee, SenderCategory: ptr("paid_registered")},
			wantKey:  "country:" + kz.String() + "|operator:" + bee.String() + "|sender_category:paid_registered",
			wantPrio: 30,
		},
		{
			name: "country + operator + sender_category + traffic_type",
			dims: services.PeriodDimensions{
				CountryID: &kz, OperatorID: &bee,
				SenderCategory: ptr("paid_registered"), TrafficType: ptr("transactional"),
			},
			wantKey:  "country:" + kz.String() + "|operator:" + bee.String() + "|sender_category:paid_registered|traffic_type:transactional",
			wantPrio: 40,
		},
		{
			name:     "country + client",
			dims:     services.PeriodDimensions{CountryID: &kz, ClientID: &acme},
			wantKey:  "client:" + acme.String() + "|country:" + kz.String(),
			wantPrio: 110,
		},
		{
			name:     "global + client",
			dims:     services.PeriodDimensions{ClientID: &acme},
			wantKey:  "client:" + acme.String(),
			wantPrio: 100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantKey, services.ComputeScopeKey(tc.dims))
			assert.Equal(t, tc.wantPrio, services.ComputeScopePriority(tc.dims))
		})
	}
}

func TestValidateDimensionHierarchy(t *testing.T) {
	id := uuid.New()

	assert.NoError(t, services.ValidateDimensionHierarchy(services.PeriodDimensions{}))
	assert.NoError(t, services.ValidateDimensionHierarchy(services.PeriodDimensions{CountryID: &id}))
	assert.NoError(t, services.ValidateDimensionHierarchy(services.PeriodDimensions{CountryID: &id, OperatorID: &id}))

	// operator requires country
	err := services.ValidateDimensionHierarchy(services.PeriodDimensions{OperatorID: &id})
	assert.ErrorIs(t, err, services.ErrMissingCountry)

	// sender_category requires operator
	err = services.ValidateDimensionHierarchy(services.PeriodDimensions{CountryID: &id, SenderCategory: ptr("shared")})
	assert.ErrorIs(t, err, services.ErrMissingOperator)

	// traffic_type requires sender_category
	err = services.ValidateDimensionHierarchy(services.PeriodDimensions{CountryID: &id, OperatorID: &id, TrafficType: ptr("transactional")})
	assert.ErrorIs(t, err, services.ErrMissingSenderCategory)
}

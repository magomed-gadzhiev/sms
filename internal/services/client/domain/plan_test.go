package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlan_HasFeature(t *testing.T) {
	plan := &Plan{
		Features: PlanFeatures{
			Analytics:    true,
			Webhooks:     false,
			HLR:          true,
			SmartRouting: false,
			SubAccounts:  true,
			WhiteLabel:   false,
		},
	}

	tests := []struct {
		feature  string
		expected bool
	}{
		{"analytics", true},
		{"webhooks", false},
		{"hlr", true},
		{"smart_routing", false},
		{"sub_accounts", true},
		{"white_label", false},
		{"unknown_feature", false},
		{"", false},
		{"ANALYTICS", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.feature, func(t *testing.T) {
			assert.Equal(t, tt.expected, plan.HasFeature(tt.feature))
		})
	}
}

func TestPlan_HasFeature_AllEnabled(t *testing.T) {
	plan := &Plan{
		Features: PlanFeatures{
			Analytics:    true,
			Webhooks:     true,
			HLR:          true,
			SmartRouting: true,
			SubAccounts:  true,
			WhiteLabel:   true,
		},
	}

	features := []string{"analytics", "webhooks", "hlr", "smart_routing", "sub_accounts", "white_label"}
	for _, f := range features {
		assert.True(t, plan.HasFeature(f), "feature %s should be enabled", f)
	}
}

func TestPlan_HasFeature_AllDisabled(t *testing.T) {
	plan := &Plan{Features: PlanFeatures{}}

	features := []string{"analytics", "webhooks", "hlr", "smart_routing", "sub_accounts", "white_label"}
	for _, f := range features {
		assert.False(t, plan.HasFeature(f), "feature %s should be disabled", f)
	}
}

func TestPlan_MarshalFeatures(t *testing.T) {
	plan := &Plan{
		Features: PlanFeatures{
			Analytics:    true,
			Webhooks:     false,
			HLR:          true,
			SmartRouting: false,
			SubAccounts:  true,
			WhiteLabel:   false,
		},
	}

	data, err := plan.MarshalFeatures()
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var result PlanFeatures
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, plan.Features, result)
}

func TestPlan_UnmarshalFeatures(t *testing.T) {
	plan := &Plan{}
	data := []byte(`{"analytics":true,"webhooks":true,"hlr":false,"smart_routing":true,"sub_accounts":false,"white_label":true}`)

	err := plan.UnmarshalFeatures(data)
	require.NoError(t, err)

	assert.True(t, plan.Features.Analytics)
	assert.True(t, plan.Features.Webhooks)
	assert.False(t, plan.Features.HLR)
	assert.True(t, plan.Features.SmartRouting)
	assert.False(t, plan.Features.SubAccounts)
	assert.True(t, plan.Features.WhiteLabel)
}

func TestPlan_UnmarshalFeatures_InvalidJSON(t *testing.T) {
	plan := &Plan{}
	err := plan.UnmarshalFeatures([]byte(`{invalid`))
	assert.Error(t, err)
}

func TestPlan_UnmarshalFeatures_EmptyObject(t *testing.T) {
	plan := &Plan{}
	err := plan.UnmarshalFeatures([]byte(`{}`))
	require.NoError(t, err)

	assert.False(t, plan.Features.Analytics)
	assert.False(t, plan.Features.Webhooks)
	assert.False(t, plan.Features.HLR)
	assert.False(t, plan.Features.SmartRouting)
	assert.False(t, plan.Features.SubAccounts)
	assert.False(t, plan.Features.WhiteLabel)
}

func TestPlan_MarshalUnmarshalRoundtrip(t *testing.T) {
	original := &Plan{
		Features: PlanFeatures{
			Analytics:    true,
			Webhooks:     true,
			HLR:          true,
			SmartRouting: true,
			SubAccounts:  true,
			WhiteLabel:   true,
		},
	}

	data, err := original.MarshalFeatures()
	require.NoError(t, err)

	target := &Plan{}
	err = target.UnmarshalFeatures(data)
	require.NoError(t, err)

	assert.Equal(t, original.Features, target.Features)
}

func TestPlan_ZeroValue(t *testing.T) {
	var plan Plan
	assert.Equal(t, uuid.Nil, plan.ID)
	assert.Equal(t, "", plan.Name)
	assert.Equal(t, "", plan.DisplayName)
	assert.Equal(t, 0.0, plan.MonthlyPriceRub)
	assert.Equal(t, 0, plan.MaxSMSPerMonth)
	assert.Equal(t, 0, plan.MaxSMPPConnections)
	assert.Equal(t, 0, plan.MaxUsers)
	assert.Equal(t, 0, plan.RateLimitPerSecond)
	assert.Equal(t, 0, plan.RateLimitPerMinute)
	assert.Equal(t, 0, plan.RateLimitPerHour)
	assert.Equal(t, 0, plan.RateLimitPerDay)
	assert.False(t, plan.Active)
	assert.False(t, plan.Features.Analytics)
}

func TestPlanFeatures_ZeroValue(t *testing.T) {
	var f PlanFeatures
	assert.False(t, f.Analytics)
	assert.False(t, f.Webhooks)
	assert.False(t, f.HLR)
	assert.False(t, f.SmartRouting)
	assert.False(t, f.SubAccounts)
	assert.False(t, f.WhiteLabel)
}

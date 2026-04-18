package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTiersCache_GetOrParseTiered_CachesSameVersion(t *testing.T) {
	cache, err := NewTiersCache(128)
	require.NoError(t, err)

	ruleID := uuid.New()
	raw := []byte(`{"period":"calendar_month","tiers":[{"up_to":10000,"price":1.0},{"up_to":null,"price":0.5}]}`)

	spec1, err := cache.GetOrParseTiered(ruleID, 1, raw)
	require.NoError(t, err)
	require.Len(t, spec1.Tiers, 2)
	require.Equal(t, "calendar_month", spec1.Period)

	spec2, err := cache.GetOrParseTiered(ruleID, 1, raw)
	require.NoError(t, err)
	require.Same(t, spec1, spec2, "same key → same pointer (cache hit)")
}

func TestTiersCache_GetOrParseTiered_DifferentVersionRepars(t *testing.T) {
	cache, err := NewTiersCache(128)
	require.NoError(t, err)

	ruleID := uuid.New()
	rawV1 := []byte(`{"period":"calendar_month","tiers":[{"up_to":null,"price":1.0}]}`)
	rawV2 := []byte(`{"period":"calendar_month","tiers":[{"up_to":null,"price":2.0}]}`)

	s1, err := cache.GetOrParseTiered(ruleID, 1, rawV1)
	require.NoError(t, err)
	s2, err := cache.GetOrParseTiered(ruleID, 2, rawV2)
	require.NoError(t, err)
	require.NotSame(t, s1, s2)
	require.InDelta(t, 1.0, s1.Tiers[0].Price, 1e-9)
	require.InDelta(t, 2.0, s2.Tiers[0].Price, 1e-9)
}

func TestTiersCache_GetOrParsePrepaid(t *testing.T) {
	cache, err := NewTiersCache(128)
	require.NoError(t, err)

	ruleID := uuid.New()
	raw := []byte(`{"period":"calendar_month","prepaid_amount":1000,"included_segments":5000,"overage_price":0.5}`)

	spec, err := cache.GetOrParsePrepaid(ruleID, 1, raw)
	require.NoError(t, err)
	require.InDelta(t, 1000.0, spec.PrepaidAmount, 1e-9)
	require.Equal(t, int64(5000), spec.IncludedSegments)
	require.InDelta(t, 0.5, spec.OveragePrice, 1e-9)
}

func TestTiersCache_InvalidJSON(t *testing.T) {
	cache, _ := NewTiersCache(128)
	_, err := cache.GetOrParseTiered(uuid.New(), 1, []byte(`{bad json`))
	require.Error(t, err)
}

func TestTiersCache_BulkClearOnOverflow(t *testing.T) {
	cache, _ := NewTiersCache(2)
	raw := []byte(`{"period":"calendar_month","tiers":[{"up_to":null,"price":1.0}]}`)

	_, _ = cache.GetOrParseTiered(uuid.New(), 1, raw)
	_, _ = cache.GetOrParseTiered(uuid.New(), 1, raw)
	// 3rd entry triggers bulk clear (soft limit)
	third, err := cache.GetOrParseTiered(uuid.New(), 1, raw)
	require.NoError(t, err)
	require.NotNil(t, third)
	// После clear в кеше только последний entry
	require.LessOrEqual(t, len(cache.tiered), 2)
}

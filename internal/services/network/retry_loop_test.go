package network

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

type recordingProviderApplier struct {
	calls atomic.Int32
	err   error
}

func (r *recordingProviderApplier) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	r.calls.Add(1)
	return r.err
}

type recordingRouteApplier struct {
	calls atomic.Int32
	err   error
}

func (r *recordingRouteApplier) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	r.calls.Add(1)
	return r.err
}

func TestRetryPendingOnce_RetriesPendingRows_AndClearsOnSuccess(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)
	psID := storagetest.SeedProviderSet(t, pool, resellerID, "ps-retry-success")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "rs-retry-success")
	storagetest.SeedSRAErrorState(t, pool, clientID, &psID, &rsID, "old failure")

	pm := &recordingProviderApplier{}
	rm := &recordingRouteApplier{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, RetryPendingOnce(ctx, pool, pm, rm))
	assert.GreaterOrEqual(t, pm.calls.Load(), int32(1), "provider applier должен быть вызван хотя бы 1 раз")
	assert.GreaterOrEqual(t, rm.calls.Load(), int32(1), "route applier должен быть вызван хотя бы 1 раз")

	var errText *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_materialize_error_text FROM subaccount_routing_assignment WHERE client_id=$1`,
		clientID,
	).Scan(&errText))
	assert.Nil(t, errText, "retry-state must be cleared on success")
}

func TestRetryPendingOnce_KeepsErrorOnContinuedFailure(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)
	psID := storagetest.SeedProviderSet(t, pool, resellerID, "ps-retry-keep")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "rs-retry-keep")
	storagetest.SeedSRAErrorState(t, pool, clientID, &psID, &rsID, "first failure")

	pm := &recordingProviderApplier{err: errors.New("still broken")}
	rm := &recordingRouteApplier{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	beforeProvRetry := testutil.ToFloat64(MaterializeFailureTotal.WithLabelValues("provider", "retry"))
	require.NoError(t, RetryPendingOnce(ctx, pool, pm, rm))

	afterProvRetry := testutil.ToFloat64(MaterializeFailureTotal.WithLabelValues("provider", "retry"))
	assert.GreaterOrEqual(t, afterProvRetry-beforeProvRetry, float64(1),
		"MaterializeFailureTotal{kind=provider,source=retry} must bump on retry-tick failure")

	var retryCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT materialize_retry_count FROM subaccount_routing_assignment WHERE client_id=$1`,
		clientID,
	).Scan(&retryCount))
	assert.GreaterOrEqual(t, retryCount, 2, "retry_count должен инкрементироваться при повторном fail")

	var errText *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_materialize_error_text FROM subaccount_routing_assignment WHERE client_id=$1`,
		clientID,
	).Scan(&errText))
	require.NotNil(t, errText, "error text должен сохраниться при повторном fail")
	assert.Contains(t, *errText, "still broken")
}

func TestRetryPendingOnce_RouteFails_RecordsRetryRouteCounter(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)
	psID := storagetest.SeedProviderSet(t, pool, resellerID, "ps-retry-route-fail")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "rs-retry-route-fail")
	storagetest.SeedSRAErrorState(t, pool, clientID, &psID, &rsID, "first failure")

	pm := &recordingProviderApplier{}
	rm := &recordingRouteApplier{err: errors.New("route still broken")}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	beforeRouteRetry := testutil.ToFloat64(MaterializeFailureTotal.WithLabelValues("route", "retry"))
	require.NoError(t, RetryPendingOnce(ctx, pool, pm, rm))

	afterRouteRetry := testutil.ToFloat64(MaterializeFailureTotal.WithLabelValues("route", "retry"))
	assert.GreaterOrEqual(t, afterRouteRetry-beforeRouteRetry, float64(1),
		"MaterializeFailureTotal{kind=route,source=retry} must bump on retry-tick route failure")
}

func TestRetryPendingOnce_SkipsRowsAtCap(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)

	// Row A: below cap — should be picked up and succeed (error_at cleared).
	clientA := storagetest.SeedSubAccount(t, pool, resellerID)
	psA := storagetest.SeedProviderSet(t, pool, resellerID, "ps-cap-below")
	rsA := storagetest.SeedRouteSet(t, pool, resellerID, "rs-cap-below")
	storagetest.SeedSRAErrorStateWithCount(t, pool, clientA, &psA, &rsA, "below cap", 99)

	// Row B: at cap — must be skipped entirely (error_at preserved).
	clientB := storagetest.SeedSubAccount(t, pool, resellerID)
	psB := storagetest.SeedProviderSet(t, pool, resellerID, "ps-cap-at")
	rsB := storagetest.SeedRouteSet(t, pool, resellerID, "rs-cap-at")
	storagetest.SeedSRAErrorStateWithCount(t, pool, clientB, &psB, &rsB, "at cap", 100)

	pm := &recordingProviderApplier{}
	rm := &recordingRouteApplier{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, RetryPendingOnce(ctx, pool, pm, rm))

	// Row A must be cleared (materializer succeeded).
	var errAtA *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_materialize_error_at FROM subaccount_routing_assignment WHERE client_id=$1`,
		clientA,
	).Scan(&errAtA))
	assert.Nil(t, errAtA, "row A (retry_count=99) must have error_at cleared after success")

	// Row B must remain untouched (skipped by WHERE materialize_retry_count < 100).
	var errAtB *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT last_materialize_error_at FROM subaccount_routing_assignment WHERE client_id=$1`,
		clientB,
	).Scan(&errAtB))
	assert.NotNil(t, errAtB, "row B (retry_count=100) must remain with error_at set (skipped by cap)")

	// SRARetryGiveUpGauge must reflect at least 1 give-up row (row B).
	giveUp := testutil.ToFloat64(SRARetryGiveUpGauge)
	assert.GreaterOrEqual(t, giveUp, float64(1), "SRARetryGiveUpGauge must be >= 1 (row B at cap)")
}


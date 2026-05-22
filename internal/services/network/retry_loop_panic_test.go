package network

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// panickingProviderApplier симулирует panic внутри materializer'а
// (например, nil-deref в pgx scan). Должен быть пойман per-row recover'ом
// в RetryPendingOnce — иначе worker process падает.
type panickingProviderApplier struct{}

func (panickingProviderApplier) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	panic("synthetic panic in provider materializer")
}

// TestRetryPendingOnce_PerRowPanicRecovered_LoopContinues — Plan 5 Task 4 (B2):
// если materializer panic'ует, RetryPendingOnce должен:
//  1. вернуться без panic (loop не убит → worker process выжил),
//  2. инкрементировать MaterializeFailureTotal{retry_panic,retry}.
func TestRetryPendingOnce_PerRowPanicRecovered_LoopContinues(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)
	psID := storagetest.SeedProviderSet(t, pool, resellerID, "ps-retry-panic")
	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "rs-retry-panic")
	storagetest.SeedSRAErrorState(t, pool, clientID, &psID, &rsID, "stuck before panic")

	pm := panickingProviderApplier{}
	rm := &recordingRouteApplier{err: errors.New("route also broken")}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	beforePanic := testutil.ToFloat64(MaterializeFailureTotal.WithLabelValues("retry_panic", "retry"))

	// Не должно паниковать — recover ловит panic в анонимной функции внутри цикла.
	require.NotPanics(t, func() {
		err := RetryPendingOnce(ctx, pool, pm, rm)
		require.NoError(t, err, "RetryPendingOnce должен возвращать nil даже при per-row panic")
	}, "per-row panic должен быть пойман recover'ом")

	afterPanic := testutil.ToFloat64(MaterializeFailureTotal.WithLabelValues("retry_panic", "retry"))
	assert.GreaterOrEqual(t, afterPanic-beforePanic, float64(1),
		"MaterializeFailureTotal{retry_panic,retry} должен инкрементироваться при per-row panic")
}

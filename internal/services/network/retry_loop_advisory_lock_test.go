package network

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// slowProviderApplier удерживает обработку ~delay чтобы две goroutine
// гарантированно overlap'или friction window между SELECT и UPDATE
// retry_count в applier'е. Позволяет проверить advisory-lock claim
// на SELECT-time race.
type slowProviderApplier struct {
	mu            sync.Mutex
	callsByClient map[string]int
	delay         time.Duration
}

func (s *slowProviderApplier) ApplyToClient(_ context.Context, clientID uuid.UUID, _ *uuid.UUID) error {
	s.mu.Lock()
	s.callsByClient[clientID.String()]++
	s.mu.Unlock()
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return nil
}

type slowRouteApplier struct {
	mu            sync.Mutex
	callsByClient map[string]int
}

func (s *slowRouteApplier) ApplyToClient(_ context.Context, clientID uuid.UUID, _ *uuid.UUID) error {
	s.mu.Lock()
	s.callsByClient[clientID.String()]++
	s.mu.Unlock()
	return nil
}

// TestRetryPendingOnce_AdvisoryLockPreventsSelectTimeRace — проверяет, что
// pg_try_advisory_xact_lock в SELECT WHERE предотвращает duplicate-work при
// двух параллельных RetryPendingOnce (имитация multi-replica worker).
//
// Без advisory-lock'а: 2 goroutine SELECT'ят те же N rows → 2*N applier
// calls. С advisory-lock'ом: ровно одна реплика claim'ит row на SELECT-time
// → N calls (по одному на client_id).
//
// Caveat: cycle-time race (gap между rows.Close и UPDATE retry_count)
// теоретически может дать +1 call (acceptable per Plan 7 Task 2 design),
// но в этом тесте не должен срабатывать т.к. delay >> gap window.
func TestRetryPendingOnce_AdvisoryLockPreventsSelectTimeRace(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	resellerID := storagetest.SeedReseller(t, pool)
	const N = 5
	clientIDs := make([]uuid.UUID, N)
	for i := 0; i < N; i++ {
		clientIDs[i] = storagetest.SeedSubAccount(t, pool, resellerID)
		psID := storagetest.SeedProviderSet(t, pool, resellerID, fmt.Sprintf("ps-lock-%d", i))
		rsID := storagetest.SeedRouteSet(t, pool, resellerID, fmt.Sprintf("rs-lock-%d", i))
		storagetest.SeedSRAErrorState(t, pool, clientIDs[i], &psID, &rsID, "stuck")
	}

	pm := &slowProviderApplier{callsByClient: map[string]int{}, delay: 200 * time.Millisecond}
	rm := &slowRouteApplier{callsByClient: map[string]int{}}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, RetryPendingOnce(ctx, pool, pm, rm))
		}()
	}
	wg.Wait()

	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, id := range clientIDs {
		// Ровно 1 call expected — advisory-lock claim'ит row на SELECT-time
		// эксклюзивно одной реплике, вторая получает пустой ResultSet.
		assert.Equal(t, 1, pm.callsByClient[id.String()],
			"client %s обработан %d раз, должно быть ровно 1 (advisory-lock на SELECT)",
			id, pm.callsByClient[id.String()])
	}
}

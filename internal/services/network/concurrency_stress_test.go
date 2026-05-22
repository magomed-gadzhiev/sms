package network_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// TestRouteSetMaterializer_ConcurrentApplyToClient — два параллельных вызова
// ApplyToClient для одного client_id должны завершиться без deadlock'а или
// порчи данных. Final client_routes должно отражать ровно один из двух route-set'ов
// (last-writer-wins), не интерливинг двух наборов provider_id одновременно.
//
// Защита: pg_advisory_xact_lock(client_id) в начале транзакции ApplyToClient
// сериализует параллельные вызовы для одного client_id. Без блокировки DELETE+INSERT
// двух транзакций может интерливироваться: T1.DELETE, T2.DELETE, T1.INSERT(provA),
// T2.INSERT(provB) → COMMIT обоих → final state содержит provA И provB.
func TestRouteSetMaterializer_ConcurrentApplyToClient(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "concur-A")
	provB := storagetest.SeedProvider(t, pool, "concur-B")

	rsA := storagetest.SeedRouteSet(t, pool, resellerID, "RS-A")
	storagetest.SeedRouteSetItem(t, pool, rsA, provA, 10, nil)
	rsB := storagetest.SeedRouteSet(t, pool, resellerID, "RS-B")
	storagetest.SeedRouteSetItem(t, pool, rsB, provB, 10, nil)

	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	mat := network.NewRouteSetMaterializer(pool, itemsRepo)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = mat.ApplyToClient(ctx, subID, &rsA)
	}()
	go func() {
		defer wg.Done()
		errs[1] = mat.ApplyToClient(ctx, subID, &rsB)
	}()
	wg.Wait()

	require.NoError(t, errs[0], "concurrent ApplyToClient #1 (rsA)")
	require.NoError(t, errs[1], "concurrent ApplyToClient #2 (rsB)")

	rows, err := pool.Query(ctx,
		`SELECT DISTINCT provider_id FROM client_routes WHERE client_id = $1 AND source = 'template'`,
		subID,
	)
	require.NoError(t, err)
	defer rows.Close()
	var providerIDs []uuid.UUID
	for rows.Next() {
		var pid uuid.UUID
		require.NoError(t, rows.Scan(&pid))
		providerIDs = append(providerIDs, pid)
	}
	require.NoError(t, rows.Err())

	require.LessOrEqual(t, len(providerIDs), 1,
		"concurrent ApplyToClient must not interleave: got %d distinct providers (%v); expected exactly one (last-writer-wins)",
		len(providerIDs), providerIDs)
	require.Equal(t, 1, len(providerIDs),
		"expected exactly one provider after two successful writes (last-writer-wins), got %d", len(providerIDs))

	winner := providerIDs[0]
	require.Truef(t, winner == provA || winner == provB,
		"winner provider %s must be one of provA=%s or provB=%s", winner, provA, provB)
}

// TestProviderSetMaterializer_ConcurrentApplyToClient — симметричный тест для
// ProviderSetMaterializer.ApplyToClient. Два параллельных provider-set'а с разными
// провайдерами для одного client_id → final client_providers содержит ровно один набор.
func TestProviderSetMaterializer_ConcurrentApplyToClient(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	resellerID := storagetest.SeedReseller(t, pool)
	subID := storagetest.SeedSubAccount(t, pool, resellerID)
	provA := storagetest.SeedProvider(t, pool, "concur-PA")
	provB := storagetest.SeedProvider(t, pool, "concur-PB")

	setRepo := storage.NewResellerProviderSetRepository(pool)
	itemsRepo := storage.NewResellerProviderSetItemsRepository(pool)

	psA, err := setRepo.Create(ctx, resellerID, "PS-A", false)
	require.NoError(t, err)
	require.NoError(t, itemsRepo.ReplaceItems(ctx, psA.ID, []storage.ProviderSetItemInput{
		{ProviderID: provA, Priority: 10},
	}))

	psB, err := setRepo.Create(ctx, resellerID, "PS-B", false)
	require.NoError(t, err)
	require.NoError(t, itemsRepo.ReplaceItems(ctx, psB.ID, []storage.ProviderSetItemInput{
		{ProviderID: provB, Priority: 10},
	}))

	mat := network.NewProviderSetMaterializer(pool)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = mat.ApplyToClient(ctx, subID, &psA.ID)
	}()
	go func() {
		defer wg.Done()
		errs[1] = mat.ApplyToClient(ctx, subID, &psB.ID)
	}()
	wg.Wait()

	require.NoError(t, errs[0], "concurrent ApplyToClient #1 (psA)")
	require.NoError(t, errs[1], "concurrent ApplyToClient #2 (psB)")

	rows, err := pool.Query(ctx,
		`SELECT DISTINCT provider_id FROM client_providers WHERE client_id = $1 AND ownership = 'inherited'`,
		subID,
	)
	require.NoError(t, err)
	defer rows.Close()
	var providerIDs []uuid.UUID
	for rows.Next() {
		var pid uuid.UUID
		require.NoError(t, rows.Scan(&pid))
		providerIDs = append(providerIDs, pid)
	}
	require.NoError(t, rows.Err())

	require.LessOrEqual(t, len(providerIDs), 1,
		"concurrent ApplyToClient must not interleave: got %d distinct providers (%v); expected exactly one (last-writer-wins)",
		len(providerIDs), providerIDs)
	require.Equal(t, 1, len(providerIDs),
		"expected exactly one provider after two successful writes, got %d", len(providerIDs))
}

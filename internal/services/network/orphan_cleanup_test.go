package network_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

// TestSRAOrphanCleanup_BothNull_DeletesRow — после установки обоих set_id в NULL
// trigger должен удалить SRA-row автоматически (миграция 000140).
//
// Контекст: ON DELETE SET NULL на FK provider_set_id/route_set_id оставляет
// (NULL, NULL) row, который засоряет List/Overview. Plan 3 Task 6.
func TestSRAOrphanCleanup_BothNull_DeletesRow(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)

	setRepo := storage.NewResellerProviderSetRepository(pool)
	ps, err := setRepo.Create(ctx, resellerID, "PS-orphan-both", false)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
         VALUES ($1, $2, NULL, now())`, clientID, ps.ID)
	require.NoError(t, err)

	// Сбросить provider_set_id в NULL — оба теперь NULL → trigger DELETE'ит row.
	_, err = pool.Exec(ctx,
		`UPDATE subaccount_routing_assignment SET provider_set_id = NULL WHERE client_id = $1`, clientID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM subaccount_routing_assignment WHERE client_id = $1`, clientID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "orphan SRA row must be deleted by trigger")
}

// TestSRAOrphanCleanup_OneNull_KeepsRow — row с одним non-NULL set остаётся.
func TestSRAOrphanCleanup_OneNull_KeepsRow(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	resellerID := storagetest.SeedReseller(t, pool)
	clientID := storagetest.SeedSubAccount(t, pool, resellerID)
	provID := storagetest.SeedProvider(t, pool, "orphan-one")

	setRepo := storage.NewResellerProviderSetRepository(pool)
	ps, err := setRepo.Create(ctx, resellerID, "PS-orphan-one", false)
	require.NoError(t, err)

	rsID := storagetest.SeedRouteSet(t, pool, resellerID, "RS-orphan-one")
	storagetest.SeedRouteSetItem(t, pool, rsID, provID, 10, nil)

	_, err = pool.Exec(ctx,
		`INSERT INTO subaccount_routing_assignment (client_id, provider_set_id, route_set_id, assigned_at)
         VALUES ($1, $2, $3, now())`, clientID, ps.ID, rsID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE subaccount_routing_assignment SET route_set_id = NULL WHERE client_id = $1`, clientID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM subaccount_routing_assignment WHERE client_id = $1`, clientID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "row with one non-NULL set must survive")
}

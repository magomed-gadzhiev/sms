package network_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestConflictValidator_NoConflict(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")

	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	rsRepo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	ps, err := psRepo.Create(ctx, resellerID, "PS", false)
	require.NoError(t, err)
	require.NoError(t, psItems.ReplaceItems(ctx, ps.ID, []storage.ProviderSetItemInput{{ProviderID: provA, Priority: 0, ExposeProviderName: true}}))
	rs, err := rsRepo.Create(ctx, resellerID, "RS", false)
	require.NoError(t, err)
	storagetest.SeedRouteSetItem(t, pool, rs.ID, provA, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	v := network.NewConflictValidator(psItems, rsItems)
	missing, err := v.MissingProviders(ctx, ps.ID, rs.ID)
	require.NoError(t, err)
	require.Empty(t, missing)
}

func TestConflictValidator_ProviderMissingFromSet(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")
	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	rsRepo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	ps, err := psRepo.Create(ctx, resellerID, "PS", false)
	require.NoError(t, err)
	require.NoError(t, psItems.ReplaceItems(ctx, ps.ID, []storage.ProviderSetItemInput{{ProviderID: provA, Priority: 0, ExposeProviderName: true}}))
	rs, err := rsRepo.Create(ctx, resellerID, "RS", false)
	require.NoError(t, err)
	storagetest.SeedRouteSetItem(t, pool, rs.ID, provB, 10, nil)

	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	v := network.NewConflictValidator(psItems, rsItems)
	missing, err := v.MissingProviders(ctx, ps.ID, rs.ID)
	require.NoError(t, err)
	require.Len(t, missing, 1)
	require.Equal(t, provB, missing[0])
}

func TestConflictValidator_NilProviderSet_AllMissing(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")

	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	rsRepo := storage.NewResellerRouteSetRepository(pool)
	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	rs, err := rsRepo.Create(ctx, resellerID, "RS", false)
	require.NoError(t, err)
	storagetest.SeedRouteSetItem(t, pool, rs.ID, provA, 10, nil)
	storagetest.SeedRouteSetItem(t, pool, rs.ID, provB, 20, nil)

	v := network.NewConflictValidator(psItems, rsItems)
	missing, err := v.MissingProvidersByIDs(ctx, nil, &rs.ID)
	require.NoError(t, err)
	require.Len(t, missing, 2)
	require.ElementsMatch(t, []uuid.UUID{provA, provB}, missing)
}

func TestConflictValidator_NilRouteSet_NoConflict(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	psRepo := storage.NewResellerProviderSetRepository(pool)
	psItems := storage.NewResellerProviderSetItemsRepository(pool)
	rsItems := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	ps, err := psRepo.Create(ctx, resellerID, "PS", false)
	require.NoError(t, err)

	v := network.NewConflictValidator(psItems, rsItems)
	// nil route-set — конфликта не существует определению
	missing, err := v.MissingProvidersByIDs(ctx, &ps.ID, nil)
	require.NoError(t, err)
	require.Empty(t, missing)
}

package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRouteSetRepo_CreateAndGet(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	repo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, resellerID, "Default routes", false)
	require.NoError(t, err)
	require.Equal(t, "Default routes", created.Name)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, resellerID, got.ResellerID)
}

func TestRouteSetRepo_DefaultUniqueness(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	repo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	_, err := repo.Create(ctx, resellerID, "A", true)
	require.NoError(t, err)
	_, err = repo.Create(ctx, resellerID, "B", true)
	require.Error(t, err)
}

func TestRouteSetRepo_ListByReseller(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	r1 := storagetest.SeedReseller(t, pool)
	r2 := storagetest.SeedReseller(t, pool)
	repo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	repo.Create(ctx, r1, "S1", false) //nolint:errcheck
	repo.Create(ctx, r1, "S2", false) //nolint:errcheck
	repo.Create(ctx, r2, "Other", false) //nolint:errcheck
	list, err := repo.ListByReseller(ctx, r1)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestRouteSetRepo_UpdateAndDelete(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	repo := storage.NewResellerRouteSetRepository(pool)
	ctx := context.Background()
	s, _ := repo.Create(ctx, resellerID, "old", false)
	require.NoError(t, repo.Update(ctx, s.ID, "new", true))
	got, _ := repo.GetByID(ctx, s.ID)
	require.Equal(t, "new", got.Name)
	require.True(t, got.IsDefault)

	require.NoError(t, repo.Delete(ctx, s.ID))
	_, err := repo.GetByID(ctx, s.ID)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

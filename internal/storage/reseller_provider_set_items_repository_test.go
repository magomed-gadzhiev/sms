package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderSetItems_ReplaceAndList(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	setRepo := NewResellerProviderSetRepository(pool)
	itemsRepo := NewResellerProviderSetItemsRepository(pool)
	providerA := seedTestProvider(t, pool, "ProvA")
	providerB := seedTestProvider(t, pool, "ProvB")
	ctx := context.Background()

	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)

	err = itemsRepo.ReplaceItems(ctx, set.ID, []ProviderSetItemInput{
		{ProviderID: providerA, Priority: 10, ExposeCost: true, ExposeProviderName: true},
		{ProviderID: providerB, Priority: 5, ExposeCost: false, ExposeProviderName: true},
	})
	require.NoError(t, err)

	items, err := itemsRepo.ListBySet(ctx, set.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)
}

func TestProviderSetItems_ReplaceClearsOld(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	setRepo := NewResellerProviderSetRepository(pool)
	itemsRepo := NewResellerProviderSetItemsRepository(pool)
	provA := seedTestProvider(t, pool, "A")
	provB := seedTestProvider(t, pool, "B")
	ctx := context.Background()

	set, err := setRepo.Create(ctx, resellerID, "S2", false)
	require.NoError(t, err)

	err = itemsRepo.ReplaceItems(ctx, set.ID, []ProviderSetItemInput{{ProviderID: provA, Priority: 10}})
	require.NoError(t, err)

	err = itemsRepo.ReplaceItems(ctx, set.ID, []ProviderSetItemInput{{ProviderID: provB, Priority: 5}})
	require.NoError(t, err)

	items, err := itemsRepo.ListBySet(ctx, set.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, provB, items[0].ProviderID)
}

func TestProviderSetItems_ListProvidersInSet(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	setRepo := NewResellerProviderSetRepository(pool)
	itemsRepo := NewResellerProviderSetItemsRepository(pool)
	provA := seedTestProvider(t, pool, "ListProv1")
	provB := seedTestProvider(t, pool, "ListProv2")
	ctx := context.Background()

	set, err := setRepo.Create(ctx, resellerID, "S3", false)
	require.NoError(t, err)

	err = itemsRepo.ReplaceItems(ctx, set.ID, []ProviderSetItemInput{
		{ProviderID: provA, Priority: 10},
		{ProviderID: provB, Priority: 5},
	})
	require.NoError(t, err)

	ids, err := itemsRepo.ListProvidersInSet(ctx, set.ID)
	require.NoError(t, err)
	require.Len(t, ids, 2)
	require.ElementsMatch(t, []uuid.UUID{provA, provB}, ids)
}

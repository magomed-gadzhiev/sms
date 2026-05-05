package storage_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRouteSetItems_CreateFullAndLoad(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, err := setRepo.Create(ctx, resellerID, "Routes", false)
	require.NoError(t, err)

	in := storage.RouteSetItemFull{
		Name: "RU operators", ProviderID: provA, Priority: 10, Share: 100,
		RouteType: "sms", Status: "active",
		ConditionGroups: []storage.RouteSetConditionGroup{{
			LogicOp: "IF",
			Conditions: []storage.RouteSetCondition{
				{Type: "country", Value: "RU"},
				{Type: "traffic_type", Value: "transactional"},
			},
		}},
		Schedules: []storage.RouteSetSchedule{{Weekdays: 127, Timezone: "Europe/Moscow"}},
	}
	created, err := itemsRepo.CreateFull(ctx, set.ID, in)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, created.ID)

	loaded, err := itemsRepo.LoadFullByItem(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "RU operators", loaded.Name)
	require.Len(t, loaded.ConditionGroups, 1)
	require.Len(t, loaded.ConditionGroups[0].Conditions, 2)
	require.Len(t, loaded.Schedules, 1)
	require.EqualValues(t, 127, loaded.Schedules[0].Weekdays)
}

func TestRouteSetItems_ListBySet_OrderedByPriorityDesc(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	prov := storagetest.SeedProvider(t, pool, "P")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)
	_, err = itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{Name: "r1", ProviderID: prov, Priority: 10, Share: 100, RouteType: "sms", Status: "active"})
	require.NoError(t, err)
	_, err = itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{Name: "r2", ProviderID: prov, Priority: 5, Share: 100, RouteType: "sms", Status: "active"})
	require.NoError(t, err)
	list, err := itemsRepo.ListBySet(ctx, set.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "r1", list[0].Name)
}

func TestRouteSetItems_UpdateFull_ReplacesGroupsAndSchedules(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	prov := storagetest.SeedProvider(t, pool, "P")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)
	created, err := itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{
		Name: "v1", ProviderID: prov, Priority: 10, Share: 100, RouteType: "sms", Status: "active",
		ConditionGroups: []storage.RouteSetConditionGroup{{LogicOp: "IF", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "RU"}}}},
	})
	require.NoError(t, err)
	require.NoError(t, itemsRepo.UpdateFull(ctx, created.ID, storage.RouteSetItemFull{
		Name: "v2", ProviderID: prov, Priority: 20, Share: 100, RouteType: "sms", Status: "active",
		ConditionGroups: []storage.RouteSetConditionGroup{{LogicOp: "IF", Conditions: []storage.RouteSetCondition{{Type: "country", Value: "KZ"}}}},
	}))
	loaded, err := itemsRepo.LoadFullByItem(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "v2", loaded.Name)
	require.EqualValues(t, 20, loaded.Priority)
	require.Equal(t, "KZ", loaded.ConditionGroups[0].Conditions[0].Value)
}

func TestRouteSetItems_Reorder(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	prov := storagetest.SeedProvider(t, pool, "P")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)
	a, err := itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{Name: "a", ProviderID: prov, Priority: 10, Share: 100, RouteType: "sms", Status: "active"})
	require.NoError(t, err)
	b, err := itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{Name: "b", ProviderID: prov, Priority: 20, Share: 100, RouteType: "sms", Status: "active"})
	require.NoError(t, err)
	require.NoError(t, itemsRepo.Reorder(ctx, set.ID, []storage.RouteSetReorderEntry{
		{ItemID: a.ID, Priority: 100},
		{ItemID: b.ID, Priority: 50},
	}))
	list, err := itemsRepo.ListBySet(ctx, set.ID)
	require.NoError(t, err)
	require.Equal(t, "a", list[0].Name)
	require.EqualValues(t, 100, list[0].Priority)
}

func TestRouteSetItems_ProvidersInSet_Distinct(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	resellerID := storagetest.SeedReseller(t, pool)
	provA := storagetest.SeedProvider(t, pool, "A")
	provB := storagetest.SeedProvider(t, pool, "B")
	setRepo := storage.NewResellerRouteSetRepository(pool)
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	ctx := context.Background()
	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)
	_, err = itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{ProviderID: provA, Share: 100, RouteType: "sms", Status: "active"})
	require.NoError(t, err)
	_, err = itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{ProviderID: provB, Share: 100, RouteType: "sms", Status: "active"})
	require.NoError(t, err)
	_, err = itemsRepo.CreateFull(ctx, set.ID, storage.RouteSetItemFull{ProviderID: provA, Share: 50, RouteType: "sms", Status: "active"})
	require.NoError(t, err)
	ids, err := itemsRepo.ProvidersInSet(ctx, set.ID)
	require.NoError(t, err)
	require.Len(t, ids, 2)
}

// Not-found tests

func TestRouteSetItems_LoadFullByItem_NotFound(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	_, err := itemsRepo.LoadFullByItem(context.Background(), uuid.New())
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestRouteSetItems_UpdateFull_NotFound(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	prov := storagetest.SeedProvider(t, pool, "P")
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	err := itemsRepo.UpdateFull(context.Background(), uuid.New(), storage.RouteSetItemFull{
		ProviderID: prov, Share: 100, RouteType: "sms", Status: "active",
	})
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestRouteSetItems_Delete_NotFound(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	itemsRepo := storage.NewResellerRouteSetItemsRepository(pool)
	err := itemsRepo.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, storage.ErrNotFound)
}

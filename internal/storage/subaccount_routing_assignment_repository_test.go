package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSRA_UpsertAndGet(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	subID := seedTestSubAccount(t, pool, resellerID)
	setRepo := NewResellerProviderSetRepository(pool)
	repo := NewSubAccountRoutingAssignmentRepository(pool)
	ctx := context.Background()

	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)

	err = repo.Upsert(ctx, subID, &set.ID, nil)
	require.NoError(t, err)

	got, err := repo.GetByClient(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, set.ID, *got.ProviderSetID)
	require.Nil(t, got.RouteSetID)
}

func TestSRA_Upsert_UpdatesExisting(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	subID := seedTestSubAccount(t, pool, resellerID)
	setRepo := NewResellerProviderSetRepository(pool)
	repo := NewSubAccountRoutingAssignmentRepository(pool)
	ctx := context.Background()

	setA, err := setRepo.Create(ctx, resellerID, "SetA", false)
	require.NoError(t, err)
	setB, err := setRepo.Create(ctx, resellerID, "SetB", false)
	require.NoError(t, err)

	// Первый upsert — вставка.
	require.NoError(t, repo.Upsert(ctx, subID, &setA.ID, nil))
	got, err := repo.GetByClient(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, setA.ID, *got.ProviderSetID)

	// Второй upsert — обновление.
	require.NoError(t, repo.Upsert(ctx, subID, &setB.ID, nil))
	got, err = repo.GetByClient(ctx, subID)
	require.NoError(t, err)
	require.Equal(t, setB.ID, *got.ProviderSetID)
}

func TestSRA_GetByClient_ErrNotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSubAccountRoutingAssignmentRepository(pool)
	_, err := repo.GetByClient(context.Background(), uuid.New())
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestSRA_ListByProviderSet(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	sub1 := seedTestSubAccount(t, pool, resellerID)
	sub2 := seedTestSubAccount(t, pool, resellerID)
	setRepo := NewResellerProviderSetRepository(pool)
	repo := NewSubAccountRoutingAssignmentRepository(pool)
	ctx := context.Background()

	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)

	require.NoError(t, repo.Upsert(ctx, sub1, &set.ID, nil))
	require.NoError(t, repo.Upsert(ctx, sub2, &set.ID, nil))

	clients, err := repo.ListClientsByProviderSet(ctx, set.ID)
	require.NoError(t, err)
	require.Len(t, clients, 2)

	// Убедимся, что оба ID присутствуют.
	clientSet := map[uuid.UUID]bool{sub1: false, sub2: false}
	for _, id := range clients {
		clientSet[id] = true
	}
	require.True(t, clientSet[sub1], "sub1 должен быть в списке")
	require.True(t, clientSet[sub2], "sub2 должен быть в списке")
}

func TestSRA_ListByProviderSet_Empty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	setRepo := NewResellerProviderSetRepository(pool)
	repo := NewSubAccountRoutingAssignmentRepository(pool)
	ctx := context.Background()

	set, err := setRepo.Create(ctx, resellerID, "Empty", false)
	require.NoError(t, err)

	clients, err := repo.ListClientsByProviderSet(ctx, set.ID)
	require.NoError(t, err)
	require.Empty(t, clients)
}

func TestSRA_ListByReseller(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	otherResellerID := seedTestReseller(t, pool)
	sub1 := seedTestSubAccount(t, pool, resellerID)
	sub2 := seedTestSubAccount(t, pool, resellerID)
	subOther := seedTestSubAccount(t, pool, otherResellerID)
	setRepo := NewResellerProviderSetRepository(pool)
	repo := NewSubAccountRoutingAssignmentRepository(pool)
	ctx := context.Background()

	set, err := setRepo.Create(ctx, resellerID, "S", false)
	require.NoError(t, err)
	setOther, err := setRepo.Create(ctx, otherResellerID, "O", false)
	require.NoError(t, err)

	require.NoError(t, repo.Upsert(ctx, sub1, &set.ID, nil))
	require.NoError(t, repo.Upsert(ctx, sub2, &set.ID, nil))
	require.NoError(t, repo.Upsert(ctx, subOther, &setOther.ID, nil))

	list, err := repo.ListByReseller(ctx, resellerID)
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Все записи должны принадлежать суб-аккаунтам resellerID.
	ids := map[uuid.UUID]bool{sub1: false, sub2: false}
	for _, a := range list {
		ids[a.ClientID] = true
	}
	require.True(t, ids[sub1])
	require.True(t, ids[sub2])
}

func TestSRA_ListByReseller_Empty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	resellerID := seedTestReseller(t, pool)
	repo := NewSubAccountRoutingAssignmentRepository(pool)

	list, err := repo.ListByReseller(context.Background(), resellerID)
	require.NoError(t, err)
	require.Empty(t, list)
}

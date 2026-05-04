package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestProviderSetRepo_CreateAndGet(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, resellerID, "Стандарт", false)
	require.NoError(t, err)
	require.Equal(t, "Стандарт", created.Name)
	require.False(t, created.IsDefault)
	require.NotEqual(t, uuid.Nil, created.ID)
	require.Equal(t, resellerID, created.ResellerID)

	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.Name, fetched.Name)
	require.Equal(t, resellerID, fetched.ResellerID)
}

func TestProviderSetRepo_DefaultUniqueness(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, resellerID, "A", true)
	require.NoError(t, err)

	// Второй is_default=true для того же агрегатора — должно нарушить partial unique index.
	_, err = repo.Create(ctx, resellerID, "B", true)
	require.Error(t, err, "ожидали ошибку нарушения unique constraint на is_default")
}

func TestProviderSetRepo_GetByID_NotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, ErrNotFound)
}

func TestProviderSetRepo_Update(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()

	s, err := repo.Create(ctx, resellerID, "old", false)
	require.NoError(t, err)

	require.NoError(t, repo.Update(ctx, s.ID, "new", true))

	got, err := repo.GetByID(ctx, s.ID)
	require.NoError(t, err)
	require.Equal(t, "new", got.Name)
	require.True(t, got.IsDefault)
}

func TestProviderSetRepo_Update_NotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewResellerProviderSetRepository(pool)
	err := repo.Update(context.Background(), uuid.New(), "x", false)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestProviderSetRepo_Delete(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()

	s, err := repo.Create(ctx, resellerID, "x", false)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, s.ID))

	_, err = repo.GetByID(ctx, s.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestProviderSetRepo_Delete_NotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewResellerProviderSetRepository(pool)
	err := repo.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, ErrNotFound)
}

func TestProviderSetRepo_ListByReseller(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	resellerID := seedTestReseller(t, pool)
	otherID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, resellerID, "A", false)
	require.NoError(t, err)
	_, err = repo.Create(ctx, resellerID, "B", false)
	require.NoError(t, err)
	_, err = repo.Create(ctx, otherID, "Z", false)
	require.NoError(t, err)

	list, err := repo.ListByReseller(ctx, resellerID)
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Убеждаемся, что все записи принадлежат нужному агрегатору.
	for _, ps := range list {
		require.Equal(t, resellerID, ps.ResellerID)
	}
}

func TestProviderSetRepo_ListByReseller_Empty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	resellerID := seedTestReseller(t, pool)
	repo := NewResellerProviderSetRepository(pool)

	list, err := repo.ListByReseller(context.Background(), resellerID)
	require.NoError(t, err)
	require.Empty(t, list)
}

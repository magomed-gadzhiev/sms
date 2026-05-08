package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/network_analytics/domain"
)

// setupViewsTestDB opens a pool from TEST_DB_DSN. Skips the test when unset so
// pre-commit / CI without a sandbox connection stays green.
func setupViewsTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN not set — пропускаем интеграционный тест ViewsRepo")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err, "pgxpool.New failed")
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestViewsRepo_Delete_TemplateReturnsErrViewIsTemplate(t *testing.T) {
	pool := setupViewsTestDB(t)
	repo := NewViewsRepo(pool)

	// Seeded template id=1 ("Владелец — полный обзор", user_id IS NULL).
	err := repo.Delete(context.Background(), 1, 51, 42)
	require.Error(t, err)
	require.Truef(t, errors.Is(err, domain.ErrViewIsTemplate),
		"expected ErrViewIsTemplate, got: %v", err)
}

func TestViewsRepo_Delete_NonexistentReturnsErrViewNotFound(t *testing.T) {
	pool := setupViewsTestDB(t)
	repo := NewViewsRepo(pool)

	err := repo.Delete(context.Background(), 999999, 51, 42)
	require.Error(t, err)
	require.Truef(t, errors.Is(err, domain.ErrViewNotFound),
		"expected ErrViewNotFound, got: %v", err)
}

func TestViewsRepo_Delete_OtherPartnerReturnsErrViewForbidden(t *testing.T) {
	pool := setupViewsTestDB(t)
	repo := NewViewsRepo(pool)
	ctx := context.Background()

	otherUser := int64(1)
	view := &domain.SavedView{
		PartnerID: 99,
		UserID:    &otherUser,
		Name:      "test-foreign-view-task1",
		Mode:      "stats",
		Filters:   "{}",
		Columns:   []string{"slice", "total"},
	}
	saved, err := repo.Save(ctx, view)
	require.NoError(t, err, "seed Save failed")
	require.NotNil(t, saved)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM saved_views WHERE id = $1`, saved.ID)
	})

	// Caller is partner 51, user 42 — does not own the seeded view.
	err = repo.Delete(ctx, saved.ID, 51, 42)
	require.Error(t, err)
	require.Truef(t, errors.Is(err, domain.ErrViewForbidden),
		"expected ErrViewForbidden, got: %v", err)
}

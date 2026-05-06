package maintenance_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/maintenance"
	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestEnsureFuturePartitions_CreatesMissingPartitions(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Используем audit_log как safe-known partitioned table.
	// Берём базу далёкое будущее (2030), чтобы не пересекать существующие партиции.
	base := time.Date(2030, 1, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", base, 3))

	// Должны существовать audit_log_y2030m01..audit_log_y2030m04 (4 = 0..3 inclusive).
	expectedCount := 4
	for i := 0; i < expectedCount; i++ {
		mt := base.AddDate(0, i, 0)
		name := fmt.Sprintf("audit_log_y%dm%02d", mt.Year(), int(mt.Month()))
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_class WHERE relname=$1)`, name).Scan(&exists))
		require.True(t, exists, "partition %s должна существовать", name)
	}

	// Cleanup test-партиций (чтобы не загромождать sandbox БД).
	for i := 0; i < expectedCount; i++ {
		mt := base.AddDate(0, i, 0)
		name := fmt.Sprintf("audit_log_y%dm%02d", mt.Year(), int(mt.Month()))
		_, _ = pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", name))
	}
}

func TestEnsureFuturePartitions_Idempotent(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	base := time.Date(2031, 6, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", base, 2))
	// Повтор не должен падать.
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", base, 2))

	// Cleanup.
	for i := 0; i < 3; i++ {
		mt := base.AddDate(0, i, 0)
		name := fmt.Sprintf("audit_log_y%dm%02d", mt.Year(), int(mt.Month()))
		_, _ = pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", name))
	}
}

func TestEnsureFuturePartitions_RejectsInvalidForward(t *testing.T) {
	// Не нужен real DB — тестируем validation.
	err := maintenance.EnsureFuturePartitions(context.Background(), (*pgxpool.Pool)(nil), "audit_log", time.Now(), 0)
	require.Error(t, err)
}

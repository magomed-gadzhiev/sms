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

func TestEnsureFuturePartitions_CreatesMissingPartitions_NamingYMM(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// audit_log использует NamingYMM (`<table>_yYYYYmMM`).
	// База — далёкое будущее (2030), чтобы не пересекать существующие партиции.
	base := time.Date(2030, 1, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", maintenance.NamingYMM, base, 3))

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

func TestEnsureFuturePartitions_CreatesMissingPartitions_NamingYYYYMM(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// lookup_log использует NamingYYYYMM (`<table>_YYYY_MM`).
	base := time.Date(2030, 1, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "lookup_log", maintenance.NamingYYYYMM, base, 2))

	// Должны существовать lookup_log_2030_01..lookup_log_2030_03 (3 = 0..2 inclusive).
	expectedCount := 3
	for i := 0; i < expectedCount; i++ {
		mt := base.AddDate(0, i, 0)
		name := fmt.Sprintf("lookup_log_%d_%02d", mt.Year(), int(mt.Month()))
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_class WHERE relname=$1)`, name).Scan(&exists))
		require.True(t, exists, "partition %s должна существовать", name)
	}

	for i := 0; i < expectedCount; i++ {
		mt := base.AddDate(0, i, 0)
		name := fmt.Sprintf("lookup_log_%d_%02d", mt.Year(), int(mt.Month()))
		_, _ = pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", name))
	}
}

func TestEnsureFuturePartitions_Idempotent(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	base := time.Date(2031, 6, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", maintenance.NamingYMM, base, 2))
	// Повтор не должен падать.
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", maintenance.NamingYMM, base, 2))

	// Cleanup.
	for i := 0; i < 3; i++ {
		mt := base.AddDate(0, i, 0)
		name := fmt.Sprintf("audit_log_y%dm%02d", mt.Year(), int(mt.Month()))
		_, _ = pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", name))
	}
}

func TestEnsureFuturePartitions_RejectsInvalidForward(t *testing.T) {
	// Не нужен real DB — тестируем validation.
	err := maintenance.EnsureFuturePartitions(context.Background(), (*pgxpool.Pool)(nil), "audit_log", maintenance.NamingYMM, time.Now(), 0)
	require.Error(t, err)
}

func TestEnsureFuturePartitions_RejectsInjectionAttempt(t *testing.T) {
	cases := []string{
		"audit_log; DROP TABLE users; --",
		"audit_log\"",
		"AUDIT_LOG", // uppercase нарушает identifier regex
		"audit-log", // дефис недопустим
		"",
		"1audit_log", // не может начинаться с цифры
	}
	for _, name := range cases {
		err := maintenance.EnsureFuturePartitions(context.Background(), (*pgxpool.Pool)(nil), name, maintenance.NamingYMM, time.Now(), 1)
		require.Error(t, err, "tableName=%q должен быть отвергнут", name)
		require.Contains(t, err.Error(), "invalid tableName", "tableName=%q error must mention invalid tableName, got %v", name, err)
	}
}

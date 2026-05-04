package storage

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// setupTestDB открывает пул к тестовой БД из TEST_DATABASE_URL.
// Тест пропускается, если переменная не задана.
func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — пропускаем storage-интеграционный тест")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err, "pgxpool.New failed")
	cleanup := func() { pool.Close() }
	return pool, cleanup
}

// seedTestReseller вставляет минимального клиента-агрегатора (is_reseller=true)
// и регистрирует его удаление через t.Cleanup. Возвращает UUID нового клиента.
func seedTestReseller(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	// clients.plan_id NOT NULL — берём любой существующий план.
	var planID uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`,
	).Scan(&planID)
	require.NoError(t, err, "нет ни одного subscription_plan — seed-данные не накатаны?")

	id := uuid.New()
	suffix := id.String()[:8]
	_, err = pool.Exec(ctx,
		`INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id)
		 VALUES ($1, $2, $3, 'secret', $4, true, true, $5)`,
		id,
		fmt.Sprintf("test-reseller-%s", suffix),
		fmt.Sprintf("apikey-%s", suffix),
		fmt.Sprintf("reseller-%s@test.local", suffix),
		planID,
	)
	require.NoError(t, err, "seedTestReseller INSERT failed")

	t.Cleanup(func() {
		// удаляем provider-set'ы агрегатора (CASCADE должен убрать items),
		// затем самого клиента.
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM reseller_provider_sets WHERE reseller_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM clients WHERE id = $1`, id)
	})

	return id
}

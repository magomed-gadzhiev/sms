package storage

// Cross-package test fixtures.
//
// Эти функции — экспортированные обёртки над приватными помощниками из
// `test_helpers_test.go`. Файл НЕ имеет суффикса `_test.go`, чтобы быть
// видимым из тестов других пакетов (handlers, services и т.п.).
//
// ВАЖНО: код выполняет реальные INSERT'ы в TEST_DATABASE_URL и должен
// использоваться ТОЛЬКО в тестах. Каждая функция помечена `t.Helper()` и
// регистрирует cleanup.

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// SetupTestDBExposed — экспортированная версия setupTestDB.
// Возвращает пул к TEST_DATABASE_URL и cleanup. Если переменная не задана —
// тест пропускается через t.Skip.
func SetupTestDBExposed(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — пропускаем интеграционный тест")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err, "pgxpool.New failed")
	cleanup := func() { pool.Close() }
	return pool, cleanup
}

// SeedTestReseller вставляет минимального клиента-агрегатора (is_reseller=true).
// Удаление регистрируется через t.Cleanup. Возвращает UUID нового клиента.
func SeedTestReseller(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

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
	require.NoError(t, err, "SeedTestReseller INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM reseller_provider_sets WHERE reseller_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM providers WHERE source_client_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM clients WHERE id = $1`, id)
	})

	return id
}

// SeedTestSubAccount вставляет суб-аккаунт (parent_client_id = resellerID).
// Удаление регистрируется через t.Cleanup. Возвращает UUID нового клиента.
func SeedTestSubAccount(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var planID uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM subscription_plans ORDER BY monthly_price_rub LIMIT 1`,
	).Scan(&planID)
	require.NoError(t, err, "нет ни одного subscription_plan")

	id := uuid.New()
	suffix := id.String()[:8]
	_, err = pool.Exec(ctx,
		`INSERT INTO clients (id, name, api_key, secret, email, active, is_reseller, plan_id, parent_client_id, account_type)
		 VALUES ($1, $2, $3, 'secret', $4, true, false, $5, $6, 'sub_account')`,
		id,
		fmt.Sprintf("test-subaccount-%s", suffix),
		fmt.Sprintf("apikey-sub-%s", suffix),
		fmt.Sprintf("sub-%s@test.local", suffix),
		planID,
		resellerID,
	)
	require.NoError(t, err, "SeedTestSubAccount INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM subaccount_routing_assignment WHERE client_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM client_providers WHERE client_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM clients WHERE id = $1`, id)
	})

	return id
}

// SeedTestProvider вставляет минимального платформенного провайдера
// (ownership='platform' по дефолту миграции 000132).
// name используется как суффикс, чтобы избежать конфликтов UNIQUE-индекса providers.name.
func SeedTestProvider(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	id := uuid.New()
	suffix := id.String()[:8]
	uniqueName := fmt.Sprintf("test-provider-%s-%s", name, suffix)

	_, err := pool.Exec(ctx,
		`INSERT INTO providers (id, name, host, port, system_id, password, bind_type, ownership)
		 VALUES ($1, $2, 'localhost', 2775, 'test', 'test', 'transceiver', 'platform')`,
		id, uniqueName,
	)
	require.NoError(t, err, "SeedTestProvider INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM providers WHERE id = $1`, id)
	})

	return id
}

// SeedTestProviderPrivate вставляет private-провайдера, принадлежащего агрегатору.
// ownership='private', source_client_id=resellerID.
func SeedTestProviderPrivate(t *testing.T, pool *pgxpool.Pool, name string, resellerID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	id := uuid.New()
	suffix := id.String()[:8]
	uniqueName := fmt.Sprintf("test-provider-priv-%s-%s", name, suffix)

	_, err := pool.Exec(ctx,
		`INSERT INTO providers (id, name, host, port, system_id, password, bind_type, ownership, source_client_id)
		 VALUES ($1, $2, 'localhost', 2775, 'test', 'test', 'transceiver', 'private', $3)`,
		id, uniqueName, resellerID,
	)
	require.NoError(t, err, "SeedTestProviderPrivate INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM providers WHERE id = $1`, id)
	})

	return id
}

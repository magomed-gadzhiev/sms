// Package storagetest содержит экспортированные хелперы для интеграционных
// тестов, использующих реальную БД из TEST_DATABASE_URL.
//
// Пакет вынесен из `internal/storage/`, чтобы зависимости от `testing` и
// `testify` не утекали в production-бинарники (cmd/api, cmd/portal-gateway
// и др.), которые импортируют internal/storage.
//
// Все функции выполняют реальные INSERT'ы в TEST_DATABASE_URL и должны
// использоваться ТОЛЬКО в тестах. Каждая функция помечена `t.Helper()` и
// регистрирует cleanup через t.Cleanup.
package storagetest

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// SetupTestDB открывает пул к TEST_DATABASE_URL и возвращает cleanup.
// Если переменная не задана — тест пропускается через t.Skip.
func SetupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
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

// SeedReseller вставляет минимального клиента-агрегатора (is_reseller=true).
// Удаление регистрируется через t.Cleanup. Возвращает UUID нового клиента.
func SeedReseller(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
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
	require.NoError(t, err, "SeedReseller INSERT failed")

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

// SeedSubAccount вставляет суб-аккаунт (parent_client_id = resellerID).
// Удаление регистрируется через t.Cleanup. Возвращает UUID нового клиента.
func SeedSubAccount(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID) uuid.UUID {
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
	require.NoError(t, err, "SeedSubAccount INSERT failed")

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

// SeedProvider вставляет минимального платформенного провайдера
// (ownership='platform' по дефолту миграции 000132).
// name используется как суффикс, чтобы избежать конфликтов UNIQUE-индекса providers.name.
func SeedProvider(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
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
	require.NoError(t, err, "SeedProvider INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM providers WHERE id = $1`, id)
	})

	return id
}

// SeedProviderPrivate вставляет private-провайдера, принадлежащего агрегатору.
// ownership='private', source_client_id=resellerID.
func SeedProviderPrivate(t *testing.T, pool *pgxpool.Pool, name string, resellerID uuid.UUID) uuid.UUID {
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
	require.NoError(t, err, "SeedProviderPrivate INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM providers WHERE id = $1`, id)
	})

	return id
}

// SeedSRAErrorState инициализирует subaccount_routing_assignment row для уже
// существующего sub-account'а: provider_set_id/route_set_id = переданные ID
// (могут быть nil), last_materialize_error_at = now(), error_text = errText.
// Используется тестами retry-логики.
//
// ВАЖНО: при providerSetID=nil И routeSetID=nil триггер trg_sra_orphan_cleanup
// (миграция 000140) удалит row при следующем UPDATE. Тесты, проверяющие
// retry-state после UPDATE, должны передавать хотя бы один не-nil set ID.
//
// Cleanup идёт через SeedSubAccount — он удаляет SRA row при teardown,
// поэтому собственного Cleanup эта функция не регистрирует.
func SeedSRAErrorState(t *testing.T, pool *pgxpool.Pool, clientID uuid.UUID, providerSetID, routeSetID *uuid.UUID, errText string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO subaccount_routing_assignment
		   (client_id, provider_set_id, route_set_id, assigned_at,
		    last_materialize_error_at, last_materialize_error_text, materialize_retry_count)
		 VALUES ($1, $2, $3, now(), now(), $4, 1)
		 ON CONFLICT (client_id) DO UPDATE SET
		    provider_set_id = EXCLUDED.provider_set_id,
		    route_set_id = EXCLUDED.route_set_id,
		    last_materialize_error_at = EXCLUDED.last_materialize_error_at,
		    last_materialize_error_text = EXCLUDED.last_materialize_error_text,
		    materialize_retry_count = subaccount_routing_assignment.materialize_retry_count + 1`,
		clientID, providerSetID, routeSetID, errText,
	)
	require.NoError(t, err, "SeedSRAErrorState INSERT failed")
}

// SeedSRAErrorStateWithCount — то же что SeedSRAErrorState, но позволяет задать
// произвольный materialize_retry_count. Нужен для тестов retry-cap логики (Plan 6
// Task 2), где требуется row с count=99 (below cap) и row с count=100 (at cap).
func SeedSRAErrorStateWithCount(t *testing.T, pool *pgxpool.Pool, clientID uuid.UUID, providerSetID, routeSetID *uuid.UUID, errText string, retryCount int) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO subaccount_routing_assignment
		   (client_id, provider_set_id, route_set_id, assigned_at,
		    last_materialize_error_at, last_materialize_error_text, materialize_retry_count)
		 VALUES ($1, $2, $3, now(), now(), $4, $5)
		 ON CONFLICT (client_id) DO UPDATE SET
		    provider_set_id = EXCLUDED.provider_set_id,
		    route_set_id = EXCLUDED.route_set_id,
		    last_materialize_error_at = EXCLUDED.last_materialize_error_at,
		    last_materialize_error_text = EXCLUDED.last_materialize_error_text,
		    materialize_retry_count = EXCLUDED.materialize_retry_count`,
		clientID, providerSetID, routeSetID, errText, retryCount,
	)
	require.NoError(t, err, "SeedSRAErrorStateWithCount INSERT failed")
}

// SeedProviderSet вставляет минимальный provider-set агрегатора и регистрирует cleanup.
func SeedProviderSet(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO reseller_provider_sets (id, reseller_id, name, is_default)
		 VALUES ($1, $2, $3, false)`,
		id, resellerID, name,
	)
	require.NoError(t, err, "SeedProviderSet INSERT failed")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM reseller_provider_sets WHERE id = $1`, id)
	})
	return id
}

// SeedRouteSet вставляет минимальный route-set агрегатора и регистрирует cleanup.
func SeedRouteSet(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO reseller_route_sets (id, reseller_id, name, is_default)
		 VALUES ($1, $2, $3, false)`,
		id, resellerID, name,
	)
	require.NoError(t, err, "SeedRouteSet INSERT failed")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM reseller_route_sets WHERE id = $1`, id)
	})
	return id
}

// SeedRouteSetItem вставляет item с одной группой условий (logic_op='IF').
// conditions: список (type, value); если пуст — item без условий (always-match).
func SeedRouteSetItem(t *testing.T, pool *pgxpool.Pool, setID, providerID uuid.UUID, priority int, conditions [][2]string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck

	itemID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO reseller_route_set_items (id, set_id, name, provider_id, priority, share, route_type, status)
		 VALUES ($1, $2, 'test-item', $3, $4, 100, 'sms', 'active')`,
		itemID, setID, providerID, priority,
	)
	require.NoError(t, err)
	if len(conditions) > 0 {
		var groupID int64
		err = tx.QueryRow(ctx,
			`INSERT INTO route_set_condition_groups (item_id, group_index, logic_op)
			 VALUES ($1, 0, 'IF') RETURNING id`,
			itemID,
		).Scan(&groupID)
		require.NoError(t, err)
		for _, c := range conditions {
			_, err = tx.Exec(ctx,
				`INSERT INTO route_set_conditions (group_id, condition_type, condition_value)
				 VALUES ($1, $2, $3)`, groupID, c[0], c[1])
			require.NoError(t, err)
		}
	}
	require.NoError(t, tx.Commit(ctx))
	// cleanup идёт через CASCADE FK при удалении reseller_route_sets в SeedRouteSet
	return itemID
}

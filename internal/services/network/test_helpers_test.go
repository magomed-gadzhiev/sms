package network

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// setupNetworkTestDB opens a pgxpool to the test DB from TEST_DATABASE_URL.
// The test is skipped if the variable is not set.
func setupNetworkTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — пропускаем network service integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err, "pgxpool.New failed")
	return pool, func() { pool.Close() }
}

// seedNetworkReseller inserts a minimal reseller client (is_reseller=true).
// Registers cleanup via t.Cleanup. Returns the new client UUID.
func seedNetworkReseller(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
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
	require.NoError(t, err, "seedNetworkReseller INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM reseller_provider_sets WHERE reseller_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM clients WHERE id = $1`, id)
	})

	return id
}

// seedNetworkSubAccount inserts a sub-account (parent_client_id = resellerID).
// Registers cleanup via t.Cleanup. Returns the new client UUID.
func seedNetworkSubAccount(t *testing.T, pool *pgxpool.Pool, resellerID uuid.UUID) uuid.UUID {
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
	require.NoError(t, err, "seedNetworkSubAccount INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM client_providers WHERE client_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM subaccount_routing_assignment WHERE client_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM clients WHERE id = $1`, id)
	})

	return id
}

// seedNetworkProvider inserts a minimal provider and registers its deletion via t.Cleanup.
func seedNetworkProvider(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	id := uuid.New()
	suffix := id.String()[:8]
	uniqueName := fmt.Sprintf("test-provider-%s-%s", name, suffix)

	_, err := pool.Exec(ctx,
		`INSERT INTO providers (id, name, host, port, system_id, password, bind_type)
		 VALUES ($1, $2, 'localhost', 2775, 'test', 'test', 'transceiver')`,
		id, uniqueName,
	)
	require.NoError(t, err, "seedNetworkProvider INSERT failed")

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM providers WHERE id = $1`, id)
	})

	return id
}

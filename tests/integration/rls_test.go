//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getAdminDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func getAppDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_APP_DATABASE_URL not set (needs sms_app role connection)")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestRLSIsolation_MessagesTable(t *testing.T) {
	ctx := context.Background()
	adminDB := getAdminDB(t)
	appDB := getAppDB(t)

	clientA := uuid.New()
	clientB := uuid.New()

	// Insert test clients as superuser (need plan_id, so get starter plan first)
	var starterPlanID uuid.UUID
	err := adminDB.QueryRow(ctx, "SELECT id FROM subscription_plans WHERE name = 'starter'").Scan(&starterPlanID)
	require.NoError(t, err, "starter plan must exist")

	_, err = adminDB.Exec(ctx, `
		INSERT INTO clients (id, name, api_key, secret, active, plan_id)
		VALUES ($1, 'Test Client A', $3, 'secret', true, $5),
		       ($2, 'Test Client B', $4, 'secret', true, $5)`,
		clientA, clientB,
		fmt.Sprintf("api-key-a-%s", clientA),
		fmt.Sprintf("api-key-b-%s", clientB),
		starterPlanID,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		adminDB.Exec(ctx, "DELETE FROM messages WHERE client_id IN ($1, $2)", clientA, clientB)
		adminDB.Exec(ctx, "DELETE FROM clients WHERE id IN ($1, $2)", clientA, clientB)
	})

	// Insert one message for each client as superuser
	_, err = adminDB.Exec(ctx, `
		INSERT INTO messages (id, client_id, source, destination, body, status)
		VALUES ($1, $2, 'sender', '+79001234567', 'hello from A', 'pending'),
		       ($3, $4, 'sender', '+79001234568', 'hello from B', 'pending')`,
		uuid.New(), clientA,
		uuid.New(), clientB,
	)
	require.NoError(t, err)

	// Set RLS context to client A
	_, err = appDB.Exec(ctx, "SET app.current_client_id = $1", clientA.String())
	require.NoError(t, err)

	var count int
	err = appDB.QueryRow(ctx, "SELECT COUNT(*) FROM messages WHERE client_id IN ($1, $2)", clientA, clientB).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Client A context should see only client A messages")

	// Set RLS context to client B
	_, err = appDB.Exec(ctx, "SET app.current_client_id = $1", clientB.String())
	require.NoError(t, err)

	err = appDB.QueryRow(ctx, "SELECT COUNT(*) FROM messages WHERE client_id IN ($1, $2)", clientA, clientB).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Client B context should see only client B messages")
}

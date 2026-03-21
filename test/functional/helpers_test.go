//go:build functional

package functional_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// skipIfNoDB skips the test when TEST_DB_HOST is not set.
func skipIfNoDB(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("TEST_DB_HOST not set, skipping functional test")
	}
}

// testDSN builds a PostgreSQL DSN from TEST_DB_* env vars.
func testDSN() string {
	host := envOr("TEST_DB_HOST", "localhost")
	port := envOr("TEST_DB_PORT", "5432")
	user := envOr("TEST_DB_USER", "smpp_test")
	password := envOr("TEST_DB_PASSWORD", "smpp_test")
	dbname := envOr("TEST_DB_NAME", "smpp_test")

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// setupTestDB opens a *sqlx.DB connection and registers cleanup.
func setupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	db, err := sqlx.Open("pgx", testDSN())
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("failed to ping test DB: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })
	return db
}

// beginTx starts a transaction and registers t.Cleanup to roll it back,
// so every test runs in isolation without side effects.
func beginTx(t *testing.T, db *sqlx.DB) *sqlx.Tx {
	t.Helper()

	tx, err := db.BeginTxx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatalf("failed to begin test transaction: %v", err)
	}

	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

// ensurePartition creates a monthly partition for the messages table covering
// the current month if it doesn't already exist.
func ensurePartition(t *testing.T, db *sqlx.DB) {
	t.Helper()

	now := time.Now()
	partName := fmt.Sprintf("messages_y%dm%02d", now.Year(), now.Month())
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	query := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF messages FOR VALUES FROM ('%s') TO ('%s')`,
		partName,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, query); err != nil {
		t.Fatalf("failed to ensure messages partition %s: %v", partName, err)
	}
}

// insertTestClient inserts a minimal client row and returns its UUID string.
// The client is needed because messages.client_id references clients(id).
func insertTestClient(t *testing.T, tx *sqlx.Tx, clientID string) {
	t.Helper()

	_, err := tx.ExecContext(context.Background(),
		`INSERT INTO clients (id, name, api_key, secret)
		 VALUES ($1, 'test-client', $1, 'secret')
		 ON CONFLICT DO NOTHING`,
		clientID,
	)
	if err != nil {
		t.Fatalf("failed to insert test client: %v", err)
	}
}

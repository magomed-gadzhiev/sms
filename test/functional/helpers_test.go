//go:build functional

package functional_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// skipIfNoDB skips the test when neither TEST_DATABASE_URL (canonical,
// architecture review candidate 5) nor TEST_DB_HOST is set.
func skipIfNoDB(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" && os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("TEST_DATABASE_URL/TEST_DB_HOST not set, skipping functional test")
	}
}

// testDSN builds a PostgreSQL DSN: TEST_DATABASE_URL (one DSN convention,
// architecture review candidate 5) wins over TEST_DB_* parts.
func testDSN() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
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

// ensureMonthlyPartition creates the current-month partition of a monthly
// RANGE-partitioned table when it doesn't already exist. Migrations only
// pre-create a handful of months; in deployed environments the pipeline-worker
// partition maintainer keeps them coming (internal/services/maintenance/
// partition_maintainer.go), but test databases get migrations alone.
func ensureMonthlyPartition(t *testing.T, db *sqlx.DB, table string, partitionName func(month time.Time) string) {
	t.Helper()

	now := time.Now()
	partName := partitionName(now)
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	query := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
		partName, table,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, query); err != nil {
		t.Fatalf("failed to ensure %s partition %s: %v", table, partName, err)
	}
}

// ensurePartition creates the current-month messages partition
// (naming per migrations: messages_y2026m09).
func ensurePartition(t *testing.T, db *sqlx.DB) {
	t.Helper()
	ensureMonthlyPartition(t, db, "messages", func(month time.Time) string {
		return fmt.Sprintf("messages_y%dm%02d", month.Year(), month.Month())
	})
}

// ensureClickEventsPartition creates the current-month click_events partition
// (naming per migration 000047: click_events_2026_09). Without it RecordClick
// fails with "no partition of relation click_events found" (SQLSTATE 23514)
// on databases provisioned from migrations only.
func ensureClickEventsPartition(t *testing.T, db *sqlx.DB) {
	t.Helper()
	ensureMonthlyPartition(t, db, "click_events", func(month time.Time) string {
		return fmt.Sprintf("click_events_%d_%02d", month.Year(), month.Month())
	})
}

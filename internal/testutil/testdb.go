package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// SetupTestDB создает тестовую базу данных
// Используется для integration тестов
func SetupTestDB(t *testing.T, dsn string) (*storage.DB, func()) {
	t.Helper()

	db, err := storage.NewDB(dsn)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping test database: %v", err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close test database: %v", err)
		}
	}

	return db, cleanup
}

// CleanupTestDB очищает тестовую базу данных
func CleanupTestDB(t *testing.T, db *storage.DB) {
	t.Helper()

	ctx := context.Background()
	tables := []string{
		"dlr_receipts",
		"messages",
		"routes",
		"providers",
		"clients",
		"audit_log",
	}

	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Logf("Failed to truncate table %s: %v", table, err)
		}
	}
}

// GetTestDSN возвращает DSN для тестовой базы данных
func GetTestDSN() string {
	host := getEnv("TEST_DB_HOST", "localhost")
	port := getEnv("TEST_DB_PORT", "5432")
	user := getEnv("TEST_DB_USER", "smpp_test")
	password := getEnv("TEST_DB_PASSWORD", "smpp_test")
	database := getEnv("TEST_DB_NAME", "smpp_test")

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, database)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// MustExec выполняет SQL запрос и паникует при ошибке
func MustExec(t *testing.T, db *storage.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}
}

// MustQuery выполняет SQL запрос и возвращает результат
func MustQuery(t *testing.T, db *storage.DB, query string, args ...interface{}) *sql.Rows {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	return rows
}

// BeginTestTx starts a transaction and registers t.Cleanup to rollback.
// Returns a pgx.Tx that can be used as a database connection for the test.
func BeginTestTx(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin test transaction: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
	})
	return tx
}

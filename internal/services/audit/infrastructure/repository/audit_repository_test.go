package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/services/audit/domain"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	return db
}

func TestQueryAuditLog_FilterByResourceType_AndTenantScope(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)

	// Random tenant/user UUIDs per run — avoids cross-run row accumulation
	// when t.Cleanup fails to fire (process killed, partition rotated, etc.).
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()

	// Non-NULL user_id required: repo scans into string, can't handle NULL.
	userA := uuid.NewString()
	userB := uuid.NewString()

	// Seed: tenant A — 2 route_set + 1 provider_set; tenant B — 3 route_set.
	_, err := db.Exec(
		`INSERT INTO audit_log (tenant_id, user_id, action, resource_type, resource_id) VALUES
		 ($1, $3, 'create', 'route_set', 'rs-a-1'),
		 ($1, $3, 'update', 'route_set', 'rs-a-2'),
		 ($1, $3, 'create', 'provider_set', 'ps-a-1'),
		 ($2, $4, 'create', 'route_set', 'rs-b-1'),
		 ($2, $4, 'create', 'route_set', 'rs-b-2'),
		 ($2, $4, 'create', 'route_set', 'rs-b-3')`,
		tenantA, tenantB, userA, userB,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM audit_log WHERE tenant_id IN ($1, $2)`, tenantA, tenantB)
	})

	ctx := context.Background()

	// Tenant A, no resource_type filter — sees only its 3 rows.
	entries, total, err := repo.QueryAuditLog(ctx, &domain.AuditLogFilters{TenantID: tenantA})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, entries, 3)
	for _, e := range entries {
		assert.Equal(t, tenantA, e.TenantID, "tenant scope leak — tenant A query returned tenant B row")
	}

	// Tenant A + resource_type=route_set — 2 rows.
	entries, total, err = repo.QueryAuditLog(ctx, &domain.AuditLogFilters{TenantID: tenantA, ResourceType: "route_set"})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	for _, e := range entries {
		assert.Equal(t, "route_set", e.ResourceType)
		assert.Equal(t, tenantA, e.TenantID)
	}

	// Tenant B sees its 3 rows, not A's.
	entries, total, err = repo.QueryAuditLog(ctx, &domain.AuditLogFilters{TenantID: tenantB})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	for _, e := range entries {
		assert.Equal(t, tenantB, e.TenantID)
	}
}

// Regression A4: domain.AuditLogEntry.UserID is plain string. NULL user_id rows
// (system/background writers without auth-context) crashed Scan with
// "converting NULL to string is unsupported" → 500 on /audit/network.
// COALESCE in SELECT must produce empty string for NULL.
func TestQueryAuditLog_NullUserID_Scans(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db)

	tenantID := uuid.NewString()

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO audit_log (tenant_id, user_id, action, resource_type, resource_id, details, ip_address, created_at)
		VALUES ($1, NULL, 'system', 'route_set', 'rs-null-1', '{}', NULL, now())`,
		tenantID,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM audit_log WHERE tenant_id = $1`, tenantID)
	})

	entries, total, err := repo.QueryAuditLog(context.Background(), &domain.AuditLogFilters{
		TenantID: tenantID,
		Page:     1,
		PerPage:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, entries, 1)
	assert.Equal(t, "", entries[0].UserID, "NULL user_id must scan as empty string")
}

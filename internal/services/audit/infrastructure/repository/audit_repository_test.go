package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

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

	tenantA := "11111111-1111-1111-1111-111111111111"
	tenantB := "22222222-2222-2222-2222-222222222222"

	// Fixed user UUIDs so user_id is non-NULL (repo scans into string, can't handle NULL).
	userA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	userB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

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

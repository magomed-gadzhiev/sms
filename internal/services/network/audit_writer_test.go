package network

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRecordAuditEvent_InsertsRow(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	tenantID := uuid.New()
	userID := uuid.New()

	err := RecordAuditEvent(context.Background(), pool, AuditEvent{
		TenantID:     tenantID,
		UserID:       &userID,
		Action:       "create",
		ResourceType: "route_set",
		ResourceID:   "rs-123",
		Details:      map[string]interface{}{"name": "Default"},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM audit_log WHERE tenant_id=$1`, tenantID)
	})

	var (
		action       string
		resourceType string
		resourceID   string
		details      []byte
	)
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT action, resource_type, resource_id, details
		   FROM audit_log
		  WHERE tenant_id=$1 AND user_id=$2
		  ORDER BY created_at DESC LIMIT 1`,
		tenantID, userID,
	).Scan(&action, &resourceType, &resourceID, &details))
	assert.Equal(t, "create", action)
	assert.Equal(t, "route_set", resourceType)
	assert.Equal(t, "rs-123", resourceID)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(details, &parsed))
	assert.Equal(t, "Default", parsed["name"])
}

func TestRecordAuditEvent_NilUserID_StillInserts(t *testing.T) {
	pool, cleanup := storagetest.SetupTestDB(t)
	defer cleanup()
	tenantID := uuid.New()

	err := RecordAuditEvent(context.Background(), pool, AuditEvent{
		TenantID:     tenantID,
		Action:       "delete",
		ResourceType: "provider_set",
		ResourceID:   "ps-1",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM audit_log WHERE tenant_id=$1`, tenantID)
	})

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE tenant_id=$1 AND user_id IS NULL`, tenantID,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

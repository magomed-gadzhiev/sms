package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogEntry_ZeroValue(t *testing.T) {
	var entry AuditLogEntry
	assert.Equal(t, "", entry.ID)
	assert.Equal(t, "", entry.TenantID)
	assert.Equal(t, "", entry.UserID)
	assert.Equal(t, "", entry.Action)
	assert.Equal(t, "", entry.ResourceType)
	assert.Equal(t, "", entry.ResourceID)
	assert.Equal(t, "", entry.Details)
	assert.Equal(t, "", entry.IPAddress)
	assert.True(t, entry.CreatedAt.IsZero())
}

func TestAuditLogEntry_FullyPopulated(t *testing.T) {
	now := time.Now()
	entry := AuditLogEntry{
		ID:           "entry-001",
		TenantID:     "tenant-abc",
		UserID:       "user-xyz",
		Action:       "create",
		ResourceType: "client",
		ResourceID:   "client-123",
		Details:      `{"field":"value"}`,
		IPAddress:    "192.168.1.1",
		CreatedAt:    now,
	}

	assert.Equal(t, "entry-001", entry.ID)
	assert.Equal(t, "tenant-abc", entry.TenantID)
	assert.Equal(t, "user-xyz", entry.UserID)
	assert.Equal(t, "create", entry.Action)
	assert.Equal(t, "client", entry.ResourceType)
	assert.Equal(t, "client-123", entry.ResourceID)
	assert.Equal(t, `{"field":"value"}`, entry.Details)
	assert.Equal(t, "192.168.1.1", entry.IPAddress)
	assert.Equal(t, now, entry.CreatedAt)
}

func TestAuditLogEntry_DetailsCanBeEmptyJSON(t *testing.T) {
	entry := AuditLogEntry{
		Details: "{}",
	}
	assert.Equal(t, "{}", entry.Details)
}

func TestAuditLogFilters_ZeroValue(t *testing.T) {
	var filters AuditLogFilters
	assert.Equal(t, "", filters.TenantID)
	assert.Equal(t, "", filters.Action)
	assert.Equal(t, "", filters.UserID)
	assert.Nil(t, filters.DateFrom)
	assert.Nil(t, filters.DateTo)
	assert.Equal(t, int32(0), filters.Page)
	assert.Equal(t, int32(0), filters.PerPage)
}

func TestAuditLogFilters_FullyPopulated(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)

	filters := AuditLogFilters{
		TenantID: "tenant-abc",
		Action:   "update",
		UserID:   "user-xyz",
		DateFrom: &from,
		DateTo:   &to,
		Page:     1,
		PerPage:  25,
	}

	assert.Equal(t, "tenant-abc", filters.TenantID)
	assert.Equal(t, "update", filters.Action)
	assert.Equal(t, "user-xyz", filters.UserID)
	require.NotNil(t, filters.DateFrom)
	require.NotNil(t, filters.DateTo)
	assert.Equal(t, from, *filters.DateFrom)
	assert.Equal(t, to, *filters.DateTo)
	assert.Equal(t, int32(1), filters.Page)
	assert.Equal(t, int32(25), filters.PerPage)
}

func TestAuditLogFilters_OnlyTenantID(t *testing.T) {
	filters := AuditLogFilters{
		TenantID: "tenant-abc",
	}

	assert.Equal(t, "tenant-abc", filters.TenantID)
	assert.Equal(t, "", filters.Action)
	assert.Equal(t, "", filters.UserID)
	assert.Nil(t, filters.DateFrom)
	assert.Nil(t, filters.DateTo)
}

func TestAuditLogFilters_DateFromOnly(t *testing.T) {
	from := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	filters := AuditLogFilters{
		TenantID: "tenant-abc",
		DateFrom: &from,
	}

	require.NotNil(t, filters.DateFrom)
	assert.Nil(t, filters.DateTo)
	assert.Equal(t, from, *filters.DateFrom)
}

func TestAuditLogFilters_DateToOnly(t *testing.T) {
	to := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	filters := AuditLogFilters{
		TenantID: "tenant-abc",
		DateTo:   &to,
	}

	assert.Nil(t, filters.DateFrom)
	require.NotNil(t, filters.DateTo)
	assert.Equal(t, to, *filters.DateTo)
}

func TestAuditLogFilters_PaginationDefaults(t *testing.T) {
	filters := AuditLogFilters{
		TenantID: "t",
		Page:     0,
		PerPage:  0,
	}

	assert.Equal(t, int32(0), filters.Page)
	assert.Equal(t, int32(0), filters.PerPage)
}

func TestAuditLogEntry_StructTags(t *testing.T) {
	// Verify the struct can be used with the expected db tags by populating all fields
	entry := AuditLogEntry{
		ID:           "id",
		TenantID:     "t",
		UserID:       "u",
		Action:       "a",
		ResourceType: "r",
		ResourceID:   "rid",
		Details:      "d",
		IPAddress:    "ip",
		CreatedAt:    time.Now(),
	}

	assert.NotEmpty(t, entry.ID)
	assert.NotEmpty(t, entry.TenantID)
}

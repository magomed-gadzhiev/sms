package domain

import (
	"context"
	"time"
)

// AuditLogEntry represents a single audit log record from the database.
type AuditLogEntry struct {
	ID           string    `db:"id"`
	TenantID     string    `db:"tenant_id"`
	UserID       string    `db:"user_id"`
	Action       string    `db:"action"`
	ResourceType string    `db:"resource_type"`
	ResourceID   string    `db:"resource_id"`
	Details      string    `db:"details"` // JSON string
	IPAddress    string    `db:"ip_address"`
	CreatedAt    time.Time `db:"created_at"`
}

// AuditLogFilters contains filters for querying audit logs.
type AuditLogFilters struct {
	TenantID     string // required
	Action       string
	UserID       string
	ResourceType string
	DateFrom     *time.Time
	DateTo       *time.Time
	Page         int32
	PerPage      int32
}

// AuditLogRepository defines the interface for querying audit logs.
type AuditLogRepository interface {
	QueryAuditLog(ctx context.Context, filters *AuditLogFilters) ([]*AuditLogEntry, int, error)
}

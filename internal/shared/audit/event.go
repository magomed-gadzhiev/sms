package audit

import (
	"time"

	"github.com/google/uuid"
)

// Audit action constants.
const (
	ActionLogin         = "auth.login"
	ActionLogout        = "auth.logout"
	ActionRegister      = "auth.register"
	ActionPasswordReset = "auth.password_reset"
	ActionTOTPEnabled   = "auth.totp_enabled"
	ActionTOTPDisabled  = "auth.totp_disabled"

	ActionAPIKeyCreated = "api_key.created"
	ActionAPIKeyRevoked = "api_key.revoked"
	ActionAPIKeyUpdated = "api_key.updated"

	ActionWebhookCreated  = "webhook.created"
	ActionWebhookUpdated  = "webhook.updated"
	ActionWebhookDeleted  = "webhook.deleted"
	ActionWebhookTestSent = "webhook.test_sent"

	ActionSubAccountCreated      = "sub_account.created"
	ActionSubAccountDeleted      = "sub_account.deleted"
	ActionSubAccountLimitUpdated = "sub_account.limit_updated"

	ActionBalanceTransferOut = "balance.transfer_out"
	ActionBalanceTransferIn  = "balance.transfer_in"

	ActionProfileUpdated = "profile.updated"
)

// Resource type constants.
const (
	ResourceAuth       = "auth"
	ResourceAPIKey     = "api_key"
	ResourceWebhook    = "webhook"
	ResourceSubAccount = "sub_account"
	ResourceBalance    = "balance"
	ResourceProfile    = "profile"
)

// AuditEvent represents an auditable action in the system.
type AuditEvent struct {
	EventID      string         `json:"event_id"`
	TenantID     string         `json:"tenant_id"`
	UserID       string         `json:"user_id,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
	IPAddress    string         `json:"ip_address,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
}

// NewAuditEvent creates a new AuditEvent with a generated event ID and current timestamp.
func NewAuditEvent(tenantID, userID, action, resourceType, resourceID string) *AuditEvent {
	return &AuditEvent{
		EventID:      uuid.New().String(),
		TenantID:     tenantID,
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Timestamp:    time.Now().UTC(),
	}
}

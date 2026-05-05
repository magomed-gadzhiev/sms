package network

import (
	"context"
	"encoding/json"
	"net"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// AuditEvent — запись в audit_log из network handler'ов.
// ResourceType ∈ {"route_set","route_set_item","provider_set","provider_set_item","assignment","route_override"}.
// Action ∈ {"create","update","delete","reorder","apply"}.
// Details — произвольный JSON, обычно {old: {...}, new: {...}} или {summary: "..."}.
type AuditEvent struct {
	TenantID     uuid.UUID              // обычно reseller_id
	UserID       *uuid.UUID             // nil для system-инициированных
	Action       string
	ResourceType string
	ResourceID   string
	Details      map[string]interface{}
	IPAddress    string // optional
}

// RecordAuditEvent — direct INSERT в audit_log. Не использует gRPC, потому что
// audit-service сейчас read-only (см. spec §6.7 commentary). Фейл — non-fatal:
// логируем и возвращаем error, но caller обычно игнорирует (mutation уже
// committed; audit-loss не должен блокировать UX).
func RecordAuditEvent(ctx context.Context, pool *pgxpool.Pool, e AuditEvent) error {
	var detailsJSON []byte
	if e.Details != nil {
		b, err := json.Marshal(e.Details)
		if err != nil {
			log.Warn().Err(err).Msg("audit details marshal failed")
			detailsJSON = []byte("{}")
		} else {
			detailsJSON = b
		}
	}
	// audit_log.ip_address is INET — r.RemoteAddr is "host:port", strip port.
	var ip interface{}
	if e.IPAddress != "" {
		host, _, splitErr := net.SplitHostPort(e.IPAddress)
		if splitErr == nil {
			ip = host
		} else {
			ip = e.IPAddress
		}
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO audit_log (tenant_id, user_id, action, resource_type, resource_id, details, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.TenantID, e.UserID, e.Action, e.ResourceType, e.ResourceID, detailsJSON, ip,
	)
	if err != nil {
		log.Error().Err(err).
			Str("resource_type", e.ResourceType).
			Str("resource_id", e.ResourceID).
			Msg("audit event insert failed")
	}
	return err
}

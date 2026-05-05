package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// stuckThreshold is the materialize_retry_count at or above which a row is
// considered "stuck" (give-up state, no further automatic retries). Matches
// the cap enforced in Task 1 migration and Task 2 pipeline worker.
const stuckThreshold = 100

// SRAStuckHandlers serves admin endpoints for inspecting and resetting
// subaccount_routing_assignment rows that have hit the retry cap.
type SRAStuckHandlers struct {
	pool *pgxpool.Pool
}

// NewSRAStuckHandlers constructs the handler.
func NewSRAStuckHandlers(pool *pgxpool.Pool) *SRAStuckHandlers {
	return &SRAStuckHandlers{pool: pool}
}

// sraStuckRow is the per-row JSON payload for the list endpoint.
type sraStuckRow struct {
	ClientID                 string     `json:"client_id"`
	ClientEmail              string     `json:"client_email"`
	ProviderSetID            *string    `json:"provider_set_id"`
	RouteSetID               *string    `json:"route_set_id"`
	MaterializeRetryCount    int        `json:"materialize_retry_count"`
	LastMaterializeErrorAt   *time.Time `json:"last_materialize_error_at"`
	LastMaterializeErrorText *string    `json:"last_materialize_error_text"`
}

// List handles GET /portal/v1/admin/network/sra-stuck.
// Returns all subaccount_routing_assignment rows where materialize_retry_count >= 100,
// ordered by last_materialize_error_at ASC (oldest failure first).
// LIMIT 500: ops dashboards never need more; protects against runaway result sets.
func (h *SRAStuckHandlers) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT
			sra.client_id,
			c.email,
			sra.provider_set_id,
			sra.route_set_id,
			sra.materialize_retry_count,
			sra.last_materialize_error_at,
			sra.last_materialize_error_text
		FROM subaccount_routing_assignment sra
		JOIN clients c ON c.id = sra.client_id
		WHERE sra.materialize_retry_count >= $1
		ORDER BY sra.last_materialize_error_at ASC
		LIMIT 500
	`, stuckThreshold)
	if err != nil {
		log.Error().Err(err).Msg("sra_stuck: query failed")
		respondError(w, shared.ErrInternalServer("не удалось получить список"))
		return
	}
	defer rows.Close()

	result := make([]sraStuckRow, 0)
	for rows.Next() {
		var row sraStuckRow
		var clientID uuid.UUID
		var providerSetID, routeSetID *uuid.UUID
		if err := rows.Scan(
			&clientID,
			&row.ClientEmail,
			&providerSetID,
			&routeSetID,
			&row.MaterializeRetryCount,
			&row.LastMaterializeErrorAt,
			&row.LastMaterializeErrorText,
		); err != nil {
			log.Error().Err(err).Msg("sra_stuck: scan failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		row.ClientID = clientID.String()
		if providerSetID != nil {
			s := providerSetID.String()
			row.ProviderSetID = &s
		}
		if routeSetID != nil {
			s := routeSetID.String()
			row.RouteSetID = &s
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("sra_stuck: rows iteration error")
		respondError(w, shared.ErrInternalServer("ошибка итерации результата"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"rows": result})
}

// Reset handles POST /portal/v1/admin/network/sra-stuck/{client_id}/reset.
// Clears the error state for a stuck row, allowing the pipeline to retry.
// Returns 404 if the row does not exist, 409 if the row is not in stuck state
// (retry_count < 100) — ops must not blindly reset active retry state and lose diagnostics.
func (h *SRAStuckHandlers) Reset(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientIDStr := vars["client_id"]
	clientID, err := uuid.Parse(clientIDStr)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат client_id"))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("sra_stuck reset: begin tx")
		respondError(w, shared.ErrInternalServer("не удалось открыть транзакцию"))
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Fetch current state for the 409-guard and audit details.
	// parent_client_id is the owning reseller — used as tenant_id in audit_log.
	// COALESCE: if the client is itself a top-level account (no parent), use client_id.
	var retryCount int
	var providerSetID, routeSetID *uuid.UUID
	var errorText *string
	var tenantID uuid.UUID
	err = tx.QueryRow(r.Context(), `
		SELECT sra.materialize_retry_count,
		       sra.provider_set_id,
		       sra.route_set_id,
		       sra.last_materialize_error_text,
		       COALESCE(c.parent_client_id, sra.client_id) AS tenant_id
		FROM subaccount_routing_assignment sra
		JOIN clients c ON c.id = sra.client_id
		WHERE sra.client_id = $1
		FOR UPDATE OF sra
	`, clientID).Scan(&retryCount, &providerSetID, &routeSetID, &errorText, &tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			respondError(w, shared.ErrNotFound("row не найден"))
			return
		}
		log.Error().Err(err).Msg("sra_stuck reset: select for update")
		respondError(w, shared.ErrInternalServer("не удалось получить данные"))
		return
	}

	if retryCount < stuckThreshold {
		respondError(w, shared.ErrConflict("row is not in stuck state"))
		return
	}

	_, err = tx.Exec(r.Context(), `
		UPDATE subaccount_routing_assignment
		SET materialize_retry_count    = 0,
		    last_materialize_error_at  = NULL,
		    last_materialize_error_text = NULL
		WHERE client_id = $1
	`, clientID)
	if err != nil {
		log.Error().Err(err).Msg("sra_stuck reset: update")
		respondError(w, shared.ErrInternalServer("не удалось сбросить состояние"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Error().Err(err).Msg("sra_stuck reset: commit")
		respondError(w, shared.ErrInternalServer("не удалось зафиксировать транзакцию"))
		return
	}

	// Audit log — non-fatal; mutation is already committed.
	auditDetails := map[string]interface{}{
		"retry_count_before": retryCount,
	}
	if providerSetID != nil {
		auditDetails["provider_set_id"] = providerSetID.String()
	}
	if routeSetID != nil {
		auditDetails["route_set_id"] = routeSetID.String()
	}
	if errorText != nil {
		auditDetails["error_text_before"] = *errorText
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     tenantID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "sra_stuck_reset",
		ResourceType: "subaccount_routing_assignment",
		ResourceID:   clientID.String(),
		Details:      auditDetails,
		IPAddress:    r.RemoteAddr,
	})

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/admin/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	bindingAppl "github.com/smpp-server/smpp-server/internal/services/template/application"
	bindingDomain "github.com/smpp-server/smpp-server/internal/services/template/domain"
)

// ModerationHandlers serves the /admin/v1/moderation/* endpoints.
type ModerationHandlers struct {
	db             *storage.DB
	bindingService *bindingAppl.OperatorBindingService
}

// NewModerationHandlers returns an uninitialised ModerationHandlers.
// Call SetDB and SetBindingService before registering routes.
func NewModerationHandlers() *ModerationHandlers {
	return &ModerationHandlers{}
}

// SetDB injects the direct DB pool.
func (h *ModerationHandlers) SetDB(db *storage.DB) {
	h.db = db
}

// SetBindingService injects the OperatorBindingService.
func (h *ModerationHandlers) SetBindingService(svc *bindingAppl.OperatorBindingService) {
	h.bindingService = svc
}

// authorizeBinding verifies that the binding identified by bindingID belongs to
// the caller's scope:
//   - global admin → binding's client must have parent_client_id IS NULL
//   - aggregator   → binding's client must have parent_client_id = <aggregator client>
func (h *ModerationHandlers) authorizeBinding(ctx context.Context, bindingID string) error {
	scope := middleware.ScopeFromContext(ctx)
	if !scope.IsGlobal && scope.ResellerID == nil {
		return errors.New("forbidden: no scope")
	}
	const q = `
SELECT c.parent_client_id
FROM operator_template_bindings b
JOIN sender_names sn ON sn.id = b.sender_name_id
JOIN clients c ON c.id = sn.client_id
WHERE b.id = $1`
	var parentClientID *uuid.UUID
	if err := h.db.QueryRowContext(ctx, q, bindingID).Scan(&parentClientID); err != nil {
		return err
	}
	if scope.IsGlobal {
		if parentClientID != nil {
			return errors.New("forbidden: binding belongs to aggregator-owned client")
		}
		return nil
	}
	aggClientID, err := resolveAggregatorClientID(ctx, h.db, *scope.ResellerID)
	if err != nil {
		return err
	}
	if parentClientID == nil || *parentClientID != aggClientID {
		return errors.New("forbidden: binding is outside your scope")
	}
	return nil
}

// Counts returns pending counts for sender-name moderation and binding moderation,
// scoped by the caller's ModerationScope.
//
// GET /admin/v1/moderation/counts
func (h *ModerationHandlers) Counts(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		respondError(w, shared.ErrInternalServer("db not configured"))
		return
	}
	scope := middleware.ScopeFromContext(r.Context())
	if !scope.IsGlobal && scope.ResellerID == nil {
		respondError(w, shared.ErrForbidden("no moderation scope"))
		return
	}

	ctx := r.Context()

	var resellerClientID uuid.UUID
	if !scope.IsGlobal {
		var err error
		resellerClientID, err = resolveAggregatorClientID(ctx, h.db, *scope.ResellerID)
		if err != nil {
			log.Err(err).Msg("moderation counts: resolve aggregator client")
			respondError(w, shared.ErrInternalServer("database error"))
			return
		}
	}

	snSQL, snArgs := buildPendingSenderNameCountQuery(scope, resellerClientID)
	var snCount int
	if err := h.db.QueryRowContext(ctx, snSQL, snArgs...).Scan(&snCount); err != nil {
		log.Err(err).Msg("moderation counts: sender_names")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}

	bSQL, bArgs := buildPendingBindingCountQuery(scope, resellerClientID)
	var bCount int
	if err := h.db.QueryRowContext(ctx, bSQL, bArgs...).Scan(&bCount); err != nil {
		log.Err(err).Msg("moderation counts: bindings")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_names_pending": snCount,
		"bindings_pending":     bCount,
	})
}

func buildPendingSenderNameCountQuery(scope middleware.Scope, resellerClientID uuid.UUID) (string, []interface{}) {
	const base = `
SELECT COUNT(*)
FROM sender_names sn
JOIN clients c ON c.id = sn.client_id
WHERE sn.status = 'pending'`
	if scope.IsGlobal {
		return base + ` AND c.parent_client_id IS NULL`, nil
	}
	return base + ` AND c.parent_client_id = $1`, []interface{}{resellerClientID}
}

func buildPendingBindingCountQuery(scope middleware.Scope, resellerClientID uuid.UUID) (string, []interface{}) {
	const base = `
SELECT COUNT(*)
FROM operator_template_bindings b
JOIN sender_names sn ON sn.id = b.sender_name_id
JOIN clients c ON c.id = sn.client_id
WHERE b.status = 'pending'`
	if scope.IsGlobal {
		return base + ` AND c.parent_client_id IS NULL`, nil
	}
	return base + ` AND c.parent_client_id = $1`, []interface{}{resellerClientID}
}

// ListPendingBindings returns pending operator_template_bindings enriched with labels.
// Supports optional ?operator_id= filter and standard ?limit= / ?offset= pagination.
//
// GET /admin/v1/moderation/bindings
func (h *ModerationHandlers) ListPendingBindings(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		respondError(w, shared.ErrInternalServer("db not configured"))
		return
	}
	scope := middleware.ScopeFromContext(r.Context())
	if !scope.IsGlobal && scope.ResellerID == nil {
		respondError(w, shared.ErrForbidden("no moderation scope"))
		return
	}

	var resellerClientID uuid.UUID
	if !scope.IsGlobal {
		var err error
		resellerClientID, err = resolveAggregatorClientID(r.Context(), h.db, *scope.ResellerID)
		if err != nil {
			log.Err(err).Msg("bindings inbox: resolve aggregator client")
			respondError(w, shared.ErrInternalServer("database error"))
			return
		}
	}

	operatorIDParam := r.URL.Query().Get("operator_id")
	limit := parseIntParam(r, "limit", 20)
	offset := parseIntParam(r, "offset", 0)

	conds := []string{"b.status = 'pending'"}
	args := []interface{}{}
	i := 1

	if operatorIDParam != "" {
		conds = append(conds, fmt.Sprintf("b.operator_id = $%d", i))
		args = append(args, operatorIDParam)
		i++
	}
	if scope.IsGlobal {
		conds = append(conds, "c.parent_client_id IS NULL")
	} else {
		conds = append(conds, fmt.Sprintf("c.parent_client_id = $%d", i))
		args = append(args, resellerClientID)
		i++
	}
	where := strings.Join(conds, " AND ")

	countSQL := fmt.Sprintf(`
SELECT COUNT(*) FROM operator_template_bindings b
JOIN sender_names sn ON sn.id = b.sender_name_id
JOIN clients c ON c.id = sn.client_id
WHERE %s`, where)

	var total int
	if err := h.db.QueryRowContext(r.Context(), countSQL, args...).Scan(&total); err != nil {
		log.Err(err).Msg("bindings inbox count")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}

	// Three-index slice to prevent backing-array aliasing with args.
	listArgs := append(args[:len(args):len(args)], limit, offset)
	listSQL := fmt.Sprintf(`
SELECT b.id, b.template_id, b.sender_name_id, b.operator_id, b.status, b.created_at,
       t.name, t.body, sn.name, sn.channel, COALESCE(c.email, ''), COALESCE(o.name, '')
FROM operator_template_bindings b
JOIN sender_names sn ON sn.id = b.sender_name_id
JOIN clients c ON c.id = sn.client_id
JOIN templates t ON t.id = b.template_id
JOIN operators o ON o.id = b.operator_id
WHERE %s
ORDER BY b.created_at ASC
LIMIT $%d OFFSET $%d`, where, i, i+1)

	rows, err := h.db.QueryContext(r.Context(), listSQL, listArgs...)
	if err != nil {
		log.Err(err).Msg("bindings inbox list")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}
	defer rows.Close()

	out := make([]map[string]interface{}, 0, limit)
	for rows.Next() {
		var (
			id, templateID, senderNameID, operatorID, status string
			createdAt                                         time.Time
			tplName, tplBody, snName, snChannel               string
			clientEmail, opName                               string
		)
		if err := rows.Scan(
			&id, &templateID, &senderNameID, &operatorID, &status, &createdAt,
			&tplName, &tplBody, &snName, &snChannel, &clientEmail, &opName,
		); err != nil {
			log.Err(err).Msg("bindings inbox scan")
			respondError(w, shared.ErrInternalServer("database error"))
			return
		}
		out = append(out, map[string]interface{}{
			"id":             id,
			"template_id":    templateID,
			"sender_name_id": senderNameID,
			"operator_id":    operatorID,
			"status":         status,
			"created_at":     createdAt.Format(time.RFC3339),
			"template_name":  tplName,
			"template_body":  tplBody,
			"sender_name":    snName,
			"channel":        snChannel,
			"client_email":   clientEmail,
			"operator_name":  opName,
		})
	}
	if err := rows.Err(); err != nil {
		log.Err(err).Msg("bindings inbox iterate")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"bindings": out,
		"total":    total,
	})
}

// ApproveBinding approves a pending operator_template_binding.
//
// POST /admin/v1/moderation/bindings/:id/approve
func (h *ModerationHandlers) ApproveBinding(w http.ResponseWriter, r *http.Request) {
	if h.bindingService == nil || h.db == nil {
		respondError(w, shared.ErrInternalServer("not configured"))
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]
	bindingID, err := uuid.Parse(id)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("bad binding id"))
		return
	}
	if err := h.authorizeBinding(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, shared.ErrNotFound("binding not found"))
			return
		}
		respondError(w, shared.ErrForbidden("access denied"))
		return
	}
	reviewerID, _ := middleware.GetUserID(r.Context())
	if err := h.bindingService.Approve(r.Context(), bindingID, reviewerID); err != nil {
		if errors.Is(err, bindingDomain.ErrOperatorBindingNotFound) {
			respondError(w, shared.ErrNotFound("binding not found"))
			return
		}
		if errors.Is(err, bindingDomain.ErrInvalidOperatorBindingTransition) {
			respondError(w, shared.ErrInvalidInput("invalid status transition"))
			return
		}
		log.Err(err).Msg("approve binding")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "approved"})
}

// RejectBinding rejects a pending operator_template_binding with a mandatory reason.
//
// POST /admin/v1/moderation/bindings/:id/reject
func (h *ModerationHandlers) RejectBinding(w http.ResponseWriter, r *http.Request) {
	if h.bindingService == nil || h.db == nil {
		respondError(w, shared.ErrInternalServer("not configured"))
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]
	bindingID, err := uuid.Parse(id)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("bad binding id"))
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("bad body"))
		return
	}

	if err := h.authorizeBinding(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, shared.ErrNotFound("binding not found"))
			return
		}
		respondError(w, shared.ErrForbidden("access denied"))
		return
	}
	reviewerID, _ := middleware.GetUserID(r.Context())
	if err := h.bindingService.Reject(r.Context(), bindingID, reviewerID, body.Reason); err != nil {
		if errors.Is(err, bindingDomain.ErrOperatorBindingNotFound) {
			respondError(w, shared.ErrNotFound("binding not found"))
			return
		}
		if errors.Is(err, bindingDomain.ErrInvalidOperatorBindingTransition) {
			respondError(w, shared.ErrInvalidInput("invalid status transition"))
			return
		}
		if strings.Contains(err.Error(), "reason") {
			respondError(w, shared.ErrInvalidInput("rejection reason is required"))
			return
		}
		log.Err(err).Msg("reject binding")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "rejected"})
}

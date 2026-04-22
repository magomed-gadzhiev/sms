package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/admin/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// ModerationHandlers serves the /admin/v1/moderation/* endpoints.
type ModerationHandlers struct {
	db *storage.DB
}

// NewModerationHandlers returns an uninitialised ModerationHandlers.
// Call SetDB before registering routes.
func NewModerationHandlers() *ModerationHandlers {
	return &ModerationHandlers{}
}

// SetDB injects the direct DB pool.
func (h *ModerationHandlers) SetDB(db *storage.DB) {
	h.db = db
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

	snSQL, snArgs := buildPendingSenderNameCountQuery(scope)
	var snCount int
	if err := h.db.QueryRowContext(ctx, snSQL, snArgs...).Scan(&snCount); err != nil {
		log.Err(err).Msg("moderation counts: sender_names")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}

	bSQL, bArgs := buildPendingBindingCountQuery(scope)
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

func buildPendingSenderNameCountQuery(scope middleware.Scope) (string, []interface{}) {
	const base = `
SELECT COUNT(*)
FROM sender_names sn
JOIN clients c ON c.id = sn.client_id
WHERE sn.status = 'pending'`
	if scope.IsGlobal {
		return base + ` AND c.reseller_id IS NULL`, nil
	}
	return base + ` AND c.reseller_id = $1`, []interface{}{*scope.ResellerID}
}

func buildPendingBindingCountQuery(scope middleware.Scope) (string, []interface{}) {
	const base = `
SELECT COUNT(*)
FROM operator_template_bindings b
JOIN sender_names sn ON sn.id = b.sender_name_id
JOIN clients c ON c.id = sn.client_id
WHERE b.status = 'pending'`
	if scope.IsGlobal {
		return base + ` AND c.reseller_id IS NULL`, nil
	}
	return base + ` AND c.reseller_id = $1`, []interface{}{*scope.ResellerID}
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
		conds = append(conds, "c.reseller_id IS NULL")
	} else {
		conds = append(conds, fmt.Sprintf("c.reseller_id = $%d", i))
		args = append(args, *scope.ResellerID)
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

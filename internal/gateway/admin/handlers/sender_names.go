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

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	tarificationv1 "github.com/smpp-server/smpp-server/api/proto/tarificationv1"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type AdminSenderNameHandlers struct {
	client        sendernamev1.SenderNameServiceClient
	routingClient routingv1.RoutingServiceClient
	tariffClient  tarificationv1.TarificationServiceClient
	db            *storage.DB
}

func NewAdminSenderNameHandlers(client sendernamev1.SenderNameServiceClient) *AdminSenderNameHandlers {
	return &AdminSenderNameHandlers{client: client}
}

// SetClients устанавливает дополнительные gRPC-клиенты для обогащения данных.
func (h *AdminSenderNameHandlers) SetClients(
	routingClient routingv1.RoutingServiceClient,
	tariffClient tarificationv1.TarificationServiceClient,
) {
	h.routingClient = routingClient
	h.tariffClient = tariffClient
}

// SetDB injects the direct DB pool used by ListAllSenderNames for scoped queries.
func (h *AdminSenderNameHandlers) SetDB(db *storage.DB) {
	h.db = db
}

// adminSenderNameListRow holds the columns scanned from the direct SQL query.
type adminSenderNameListRow struct {
	ID              string
	ClientID        string
	ClientEmail     string
	Name            string
	Channel         string
	Status          string
	RejectionReason *string
	ReviewedAt      *time.Time
	CreatedAt       time.Time
}

// ListAllSenderNames returns sender names visible to the authenticated moderator.
//
// Scope is injected by middleware.ModerationScope (must be wired in the router):
//   - admin / superadmin  → rows where clients.reseller_id IS NULL (direct clients)
//   - aggregator_moderator → rows where clients.reseller_id = caller's user ID
//   - empty scope         → 403
//
// The response preserves the original shape (sender_names / total / limit / offset)
// and adds client_email per item.
func (h *AdminSenderNameHandlers) ListAllSenderNames(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		respondError(w, shared.ErrInternalServer("db not configured — check SetDB wiring"))
		return
	}

	scope := middleware.ScopeFromContext(r.Context())
	if !scope.IsGlobal && scope.ResellerID == nil {
		respondError(w, shared.ErrForbidden("insufficient role for sender-name moderation"))
		return
	}

	status := r.URL.Query().Get("status")
	nameQuery := r.URL.Query().Get("name_query")
	channel := r.URL.Query().Get("channel")
	clientID := r.URL.Query().Get("client_id")
	limit := parseIntParam(r, "limit", 20)
	offset := parseIntParam(r, "offset", 0)

	var (
		conds []string
		args  []interface{}
		i     = 1
	)
	conds = append(conds, "1=1")
	if status != "" {
		conds = append(conds, fmt.Sprintf("sn.status = $%d", i))
		args = append(args, status)
		i++
	}
	if channel != "" {
		conds = append(conds, fmt.Sprintf("sn.channel = $%d", i))
		args = append(args, channel)
		i++
	}
	if nameQuery != "" {
		conds = append(conds, fmt.Sprintf("sn.name ILIKE $%d", i))
		args = append(args, "%"+nameQuery+"%")
		i++
	}
	if clientID != "" {
		conds = append(conds, fmt.Sprintf("sn.client_id = $%d", i))
		args = append(args, clientID)
		i++
	}
	if scope.IsGlobal {
		conds = append(conds, "c.reseller_id IS NULL")
	} else {
		conds = append(conds, fmt.Sprintf("c.reseller_id = $%d", i))
		args = append(args, scope.ResellerID.String())
		i++
	}
	where := strings.Join(conds, " AND ")

	countSQL := fmt.Sprintf(`
SELECT COUNT(*) FROM sender_names sn
JOIN clients c ON c.id = sn.client_id
WHERE %s`, where)

	var total int32
	if err := h.db.QueryRowContext(r.Context(), countSQL, args...).Scan(&total); err != nil {
		log.Err(err).Msg("sender-name list count db error")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}

	// Three-index slice to prevent backing-array aliasing with args.
	listArgs := append(args[:len(args):len(args)], limit, offset)
	listSQL := fmt.Sprintf(`
SELECT sn.id, sn.client_id, COALESCE(c.email, ''), sn.name, sn.channel, sn.status,
       sn.rejection_reason, sn.reviewed_at, sn.created_at
FROM sender_names sn
JOIN clients c ON c.id = sn.client_id
WHERE %s
ORDER BY sn.created_at DESC
LIMIT $%d OFFSET $%d`, where, i, i+1)

	rows, err := h.db.QueryContext(r.Context(), listSQL, listArgs...)
	if err != nil {
		log.Err(err).Msg("sender-name list query db error")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}
	defer rows.Close()

	items := make([]map[string]interface{}, 0, limit)
	for rows.Next() {
		var row adminSenderNameListRow
		if err := rows.Scan(
			&row.ID, &row.ClientID, &row.ClientEmail, &row.Name, &row.Channel, &row.Status,
			&row.RejectionReason, &row.ReviewedAt, &row.CreatedAt,
		); err != nil {
			log.Err(err).Msg("sender-name list scan db error")
			respondError(w, shared.ErrInternalServer("database error"))
			return
		}
		item := map[string]interface{}{
			"id":           row.ID,
			"client_id":    row.ClientID,
			"client_email": row.ClientEmail,
			"name":         row.Name,
			"channel":      row.Channel,
			"status":       row.Status,
			"created_at":   row.CreatedAt.Format(time.RFC3339),
		}
		if row.RejectionReason != nil {
			item["rejection_reason"] = *row.RejectionReason
		} else {
			item["rejection_reason"] = ""
		}
		if row.ReviewedAt != nil {
			item["reviewed_at"] = row.ReviewedAt.Format(time.RFC3339)
		} else {
			item["reviewed_at"] = nil
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		log.Err(err).Msg("sender-name list rows iteration db error")
		respondError(w, shared.ErrInternalServer("database error"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_names": items,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

// authorizeSenderName enforces moderation scope for operations on a single
// sender name by ID. Admin/superadmin (IsGlobal) are allowed only for
// direct-client rows (reseller_id IS NULL). Aggregator moderators are allowed
// only when the sender name's client.reseller_id matches their ResellerID.
// Returns nil if access is granted, sql.ErrNoRows if the row does not exist,
// or a sentinel forbidden error otherwise.
func (h *AdminSenderNameHandlers) authorizeSenderName(ctx context.Context, id string) error {
	scope := middleware.ScopeFromContext(ctx)
	if !scope.IsGlobal && scope.ResellerID == nil {
		return fmt.Errorf("forbidden: no moderation scope")
	}

	const q = `
SELECT c.reseller_id FROM sender_names sn
JOIN clients c ON c.id = sn.client_id
WHERE sn.id = $1`

	var resellerID *string
	if err := h.db.QueryRowContext(ctx, q, id).Scan(&resellerID); err != nil {
		return err // sql.ErrNoRows or a real DB error; caller maps accordingly
	}

	if scope.IsGlobal {
		// Admin/superadmin: allow direct-client rows only (reseller_id IS NULL).
		if resellerID != nil {
			return fmt.Errorf("forbidden: sender name belongs to aggregator-owned client")
		}
		return nil
	}

	// Aggregator scope: reseller_id must match.
	if resellerID == nil || *resellerID != scope.ResellerID.String() {
		return fmt.Errorf("forbidden: sender name is outside your scope")
	}
	return nil
}

func (h *AdminSenderNameHandlers) ApproveSenderName(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	if err := h.authorizeSenderName(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, shared.ErrNotFound("sender name"))
			return
		}
		respondError(w, shared.ErrForbidden("access denied"))
		return
	}
	actorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}
	resp, err := h.client.ApproveSenderName(r.Context(), &sendernamev1.ApproveSenderNameRequest{
		Id:      id,
		ActorId: actorID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminSenderNameToJSON(resp.SenderName))
}

type rejectSenderNameRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminSenderNameHandlers) RejectSenderName(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	if err := h.authorizeSenderName(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, shared.ErrNotFound("sender name"))
			return
		}
		respondError(w, shared.ErrForbidden("access denied"))
		return
	}
	actorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}
	var req rejectSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.client.RejectSenderName(r.Context(), &sendernamev1.RejectSenderNameRequest{
		Id:      id,
		ActorId: actorID.String(),
		Reason:  req.Reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminSenderNameToJSON(resp.SenderName))
}

type deactivateSenderNameRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminSenderNameHandlers) DeactivateSenderName(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	if err := h.authorizeSenderName(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, shared.ErrNotFound("sender name"))
			return
		}
		respondError(w, shared.ErrForbidden("access denied"))
		return
	}
	actorID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Пользователь не найден"))
		return
	}
	var req deactivateSenderNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	resp, err := h.client.DeactivateSenderName(r.Context(), &sendernamev1.DeactivateSenderNameRequest{
		Id:      id,
		ActorId: actorID.String(),
		Reason:  req.Reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, adminSenderNameToJSON(resp.SenderName))
}

// GetSenderNameAdmin возвращает имя отправителя по ID с проверкой области видимости.
// GET /admin/v1/sender-names/{id}
func (h *AdminSenderNameHandlers) GetSenderNameAdmin(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}
	if err := h.authorizeSenderName(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, shared.ErrNotFound("sender name"))
			return
		}
		respondError(w, shared.ErrForbidden("access denied"))
		return
	}
	resp, err := h.client.GetSenderName(r.Context(), &sendernamev1.GetSenderNameRequest{Id: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_name": adminSenderNameToJSON(resp.SenderName),
	})
}

// GetSenderNameOperatorRegistrations возвращает список регистраций у операторов.
// GET /admin/v1/sender-names/{id}/operator-registrations
func (h *AdminSenderNameHandlers) GetSenderNameOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	// Получаем имя отправителя для client_id и name
	snResp, err := h.client.GetSenderName(r.Context(), &sendernamev1.GetSenderNameRequest{Id: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	empty := map[string]interface{}{"registrations": []interface{}{}}

	if h.tariffClient == nil {
		respondJSON(w, http.StatusOK, empty)
		return
	}

	regsResp, err := h.tariffClient.ListSenderRegistrations(r.Context(), &tarificationv1.ListSenderRegistrationsRequest{
		ClientId: snResp.SenderName.ClientId,
		Limit:    100,
	})
	if err != nil {
		log.Error().Err(err).Str("sender_name_id", id).Msg("ошибка получения регистраций ��тправителей")
		respondJSON(w, http.StatusOK, empty)
		return
	}

	registrations := make([]map[string]interface{}, 0)
	for _, reg := range regsResp.Registrations {
		if reg.SenderName != snResp.SenderName.Name {
			continue
		}
		item := map[string]interface{}{
			"operator_id":   reg.OperatorId,
			"operator_name": reg.OperatorId,
			"mcc":           "",
			"mnc":           "",
			"status":        reg.Status,
			"registered_at": nil,
		}
		if reg.CreatedAt != nil {
			item["registered_at"] = reg.CreatedAt.AsTime()
		}
		// Обогащаем данными оператора
		if h.routingClient != nil {
			op, opErr := h.routingClient.GetOperator(r.Context(), &routingv1.GetOperatorRequest{Id: reg.OperatorId})
			if opErr == nil {
				item["operator_name"] = op.Name
				item["mcc"] = op.Code
				item["mnc"] = op.CountryId
			}
		}
		registrations = append(registrations, item)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"registrations": registrations})
}

func adminSenderNameToJSON(sn *sendernamev1.SenderNameInfo) map[string]interface{} {
	m := map[string]interface{}{
		"id":               sn.Id,
		"client_id":        sn.ClientId,
		"name":             sn.Name,
		"status":           sn.Status,
		"rejection_reason": sn.RejectionReason,
		"reviewer_id":      sn.ReviewerId,
	}
	if sn.ReviewedAt != nil {
		m["reviewed_at"] = sn.ReviewedAt.AsTime()
	} else {
		m["reviewed_at"] = nil
	}
	if sn.CreatedAt != nil {
		m["created_at"] = sn.CreatedAt.AsTime()
	}
	if sn.UpdatedAt != nil {
		m["updated_at"] = sn.UpdatedAt.AsTime()
	}
	return m
}

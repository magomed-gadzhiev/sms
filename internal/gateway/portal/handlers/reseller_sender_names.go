package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerSenderNameHandlers struct {
	pool             *pgxpool.Pool
	senderNameClient sendernamev1.SenderNameServiceClient
}

func NewResellerSenderNameHandlers(pool *pgxpool.Pool, senderNameClient sendernamev1.SenderNameServiceClient) *ResellerSenderNameHandlers {
	return &ResellerSenderNameHandlers{pool: pool, senderNameClient: senderNameClient}
}

func (h *ResellerSenderNameHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return "", false
	}
	var isReseller bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
	).Scan(&isReseller); err != nil || !isReseller {
		respondError(w, shared.ErrUnauthorized("доступ только для агрегаторов"))
		return "", false
	}
	return clientID.String(), true
}

// ListResellerSenderNames GET /portal/v1/reseller/sender-names
func (h *ResellerSenderNameHandlers) ListResellerSenderNames(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	statusFilter := r.URL.Query().Get("status")
	query := `SELECT sn.id, sn.client_id, c.email AS sub_account_email,
	                 sn.name, sn.status, sn.rejection_reason, sn.created_at
	          FROM sender_names sn
	          JOIN clients c ON c.id = sn.client_id
	          WHERE c.parent_client_id = $1`
	args := []interface{}{clientID}

	if statusFilter != "" {
		query += " AND sn.status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY sn.created_at DESC LIMIT 50"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения sender names субаккаунтов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type senderNameJSON struct {
		ID              string    `json:"id"`
		ClientID        string    `json:"client_id"`
		SubAccountEmail string    `json:"sub_account_email"`
		Name            string    `json:"name"`
		Status          string    `json:"status"`
		RejectionReason *string   `json:"rejection_reason"`
		CreatedAt       time.Time `json:"created_at"`
	}
	items := make([]senderNameJSON, 0)
	for rows.Next() {
		var sn senderNameJSON
		if err := rows.Scan(&sn.ID, &sn.ClientID, &sn.SubAccountEmail,
			&sn.Name, &sn.Status, &sn.RejectionReason, &sn.CreatedAt); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, sn)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"sender_names": items, "total": len(items)})
}

// ApproveResellerSenderName POST /portal/v1/reseller/sender-names/{id}/approve
func (h *ResellerSenderNameHandlers) ApproveResellerSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT sn.status FROM sender_names sn
		 JOIN clients c ON c.id = sn.client_id
		 WHERE sn.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("имя отправителя не найдено"))
		return
	}
	if currentStatus != "pending" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса pending"))
		return
	}

	resp, err := h.senderNameClient.ApproveSenderName(r.Context(), &sendernamev1.ApproveSenderNameRequest{
		Id:      id,
		ActorId: userID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerSenderNameToJSON(resp.SenderName))
}

// RejectResellerSenderName POST /portal/v1/reseller/sender-names/{id}/reject
func (h *ResellerSenderNameHandlers) RejectResellerSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Reason == "" {
		respondError(w, shared.ErrInvalidInput("reason обязателен"))
		return
	}

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT sn.status FROM sender_names sn
		 JOIN clients c ON c.id = sn.client_id
		 WHERE sn.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("имя отправителя не найдено"))
		return
	}
	if currentStatus != "pending" {
		respondError(w, shared.ErrInvalidInput("reject возможен только из статуса pending"))
		return
	}

	resp, err := h.senderNameClient.RejectSenderName(r.Context(), &sendernamev1.RejectSenderNameRequest{
		Id:      id,
		ActorId: userID.String(),
		Reason:  req.Reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerSenderNameToJSON(resp.SenderName))
}

func resellerSenderNameToJSON(sn *sendernamev1.SenderNameInfo) map[string]interface{} {
	// Canonical shape matches ListResellerSenderNames: rejection_reason is
	// nullable. Proto emits empty string for non-rejected items; normalize
	// to null here so reseller UI and list/detail endpoints agree.
	var rejectionReason interface{}
	if sn.RejectionReason != "" {
		rejectionReason = sn.RejectionReason
	}
	m := map[string]interface{}{
		"id":               sn.Id,
		"client_id":        sn.ClientId,
		"name":             sn.Name,
		"status":           sn.Status,
		"rejection_reason": rejectionReason,
	}
	if sn.CreatedAt != nil {
		m["created_at"] = sn.CreatedAt.AsTime()
	}
	if sn.UpdatedAt != nil {
		m["updated_at"] = sn.UpdatedAt.AsTime()
	}
	return m
}

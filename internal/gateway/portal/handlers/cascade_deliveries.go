package handlers

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CascadeDeliveryHandlers обрабатывает запросы истории каскадных доставок (клиентский портал)
type CascadeDeliveryHandlers struct {
	client cascadev1.CascadeServiceClient
}

// NewCascadeDeliveryHandlers создаёт новый CascadeDeliveryHandlers
func NewCascadeDeliveryHandlers(client cascadev1.CascadeServiceClient) *CascadeDeliveryHandlers {
	return &CascadeDeliveryHandlers{client: client}
}

// ListDeliveries обрабатывает GET /cascade/deliveries
func (h *CascadeDeliveryHandlers) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	q := r.URL.Query()
	page, _ := strconv.ParseInt(q.Get("page"), 10, 32)
	pageSize, _ := strconv.ParseInt(q.Get("page_size"), 10, 32)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	resp, err := h.client.ListDeliveries(r.Context(), &cascadev1.ListDeliveriesRequest{
		ClientId:   clientID.String(),
		StrategyId: q.Get("strategy_id"),
		Status:     q.Get("status"),
		DateFrom:   q.Get("date_from"),
		DateTo:     q.Get("date_to"),
		Page:       int32(page),
		PageSize:   int32(pageSize),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// GetDelivery обрабатывает GET /cascade/deliveries/{id}
func (h *CascadeDeliveryHandlers) GetDelivery(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	deliveryID := mux.Vars(r)["id"]
	if _, err := uuid.Parse(deliveryID); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid delivery_id format"))
		return
	}
	resp, err := h.client.GetDelivery(r.Context(), &cascadev1.GetDeliveryRequest{
		DeliveryId: deliveryID,
		ClientId:   clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// GetStats обрабатывает GET /cascade/stats
func (h *CascadeDeliveryHandlers) GetStats(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	q := r.URL.Query()
	resp, err := h.client.GetDeliveryStats(r.Context(), &cascadev1.GetDeliveryStatsRequest{
		ClientId:   clientID.String(),
		StrategyId: q.Get("strategy_id"),
		DateFrom:   q.Get("date_from"),
		DateTo:     q.Get("date_to"),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

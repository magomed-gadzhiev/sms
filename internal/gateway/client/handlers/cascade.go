package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	"github.com/smpp-server/smpp-server/internal/gateway/client/middleware"
)

// CascadeHandlers содержит handlers для каскадной доставки
type CascadeHandlers struct {
	cascadeClient cascadev1.CascadeServiceClient
}

// NewCascadeHandlers создает новый CascadeHandlers
func NewCascadeHandlers(cascadeClient cascadev1.CascadeServiceClient) *CascadeHandlers {
	return &CascadeHandlers{cascadeClient: cascadeClient}
}

// CreateDeliveryRequest — тело запроса для создания каскадной доставки
type CreateDeliveryRequest struct {
	StrategyID string `json:"strategy_id"`
	Recipient  string `json:"recipient"`
	Text       string `json:"text"`
	SenderName string `json:"sender_name,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
}

// CreateDelivery обрабатывает POST /cascade/deliveries
func (h *CascadeHandlers) CreateDelivery(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateDeliveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.cascadeClient.CreateDelivery(r.Context(), &cascadev1.CreateDeliveryRequest{
		ClientId:   clientID.String(),
		StrategyId: req.StrategyID,
		Recipient:  req.Recipient,
		Text:       req.Text,
		SenderName: req.SenderName,
		RequestId:  req.RequestID,
		MessageId:  req.MessageID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// GetDelivery обрабатывает GET /cascade/deliveries/{id}
func (h *CascadeHandlers) GetDelivery(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	deliveryID := mux.Vars(r)["id"]
	resp, err := h.cascadeClient.GetDelivery(r.Context(), &cascadev1.GetDeliveryRequest{
		DeliveryId: deliveryID,
		ClientId:   clientID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// ListDeliveries обрабатывает GET /cascade/deliveries
func (h *CascadeHandlers) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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

	resp, err := h.cascadeClient.ListDeliveries(r.Context(), &cascadev1.ListDeliveriesRequest{
		ClientId:   clientID.String(),
		StrategyId: q.Get("strategy_id"),
		Status:     q.Get("status"),
		Page:       int32(page),
		PageSize:   int32(pageSize),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetStats обрабатывает GET /cascade/stats
func (h *CascadeHandlers) GetStats(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	resp, err := h.cascadeClient.GetDeliveryStats(r.Context(), &cascadev1.GetDeliveryStatsRequest{
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

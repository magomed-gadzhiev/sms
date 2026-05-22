package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	cascadev1 "github.com/smpp-server/smpp-server/api/proto/cascadev1"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// CascadeStrategyHandlers обрабатывает admin-запросы для управления стратегиями и OCS
type CascadeStrategyHandlers struct {
	client cascadev1.StrategyAdminServiceClient
}

// NewCascadeStrategyHandlers создаёт новый CascadeStrategyHandlers
func NewCascadeStrategyHandlers(client cascadev1.StrategyAdminServiceClient) *CascadeStrategyHandlers {
	return &CascadeStrategyHandlers{client: client}
}

// ListStrategies обрабатывает GET /admin/delivery-strategies
func (h *CascadeStrategyHandlers) ListStrategies(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active_only") == "true"
	resp, err := h.client.ListStrategies(r.Context(), &cascadev1.ListStrategiesRequest{
		ActiveOnly: activeOnly,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	// BUG-65: nil-slice → JSON `null` ломает frontend.
	strategies := resp.Strategies
	if strategies == nil {
		strategies = []*cascadev1.StrategyResponse{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"strategies": strategies,
	})
}

// ListStrategiesClient обрабатывает GET /cascade/strategies — клиентский read-only список активных стратегий
func (h *CascadeStrategyHandlers) ListStrategiesClient(w http.ResponseWriter, r *http.Request) {
	resp, err := h.client.ListStrategies(r.Context(), &cascadev1.ListStrategiesRequest{
		ActiveOnly: true,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	strategies := resp.Strategies
	if strategies == nil {
		strategies = []*cascadev1.StrategyResponse{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"strategies": strategies,
	})
}

// GetStrategy обрабатывает GET /admin/delivery-strategies/{id}
func (h *CascadeStrategyHandlers) GetStrategy(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := uuid.Parse(id); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid strategy_id format"))
		return
	}
	resp, err := h.client.GetStrategy(r.Context(), &cascadev1.GetStrategyRequest{StrategyId: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

type strategyStepInput struct {
	ChannelID string `json:"channel_id"`
	StepOrder int32  `json:"step_order"`
	TimeoutS  int32  `json:"timeout_s"`
	Billable  bool   `json:"billable"`
}

type createStrategyRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Mode        string              `json:"mode"` // sequential | parallel
	Steps       []strategyStepInput `json:"steps"`
}

// CreateStrategy обрабатывает POST /admin/delivery-strategies
func (h *CascadeStrategyHandlers) CreateStrategy(w http.ResponseWriter, r *http.Request) {
	var req createStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if req.Name == "" || req.Mode == "" {
		respondError(w, shared.ErrInvalidInput("name and mode are required"))
		return
	}

	grpcReq := &cascadev1.CreateStrategyRequest{
		Name:        req.Name,
		Description: req.Description,
		Mode:        req.Mode,
	}
	for _, s := range req.Steps {
		grpcReq.Steps = append(grpcReq.Steps, &cascadev1.CreateStrategyStepInput{
			ChannelId: s.ChannelID,
			StepOrder: s.StepOrder,
			TimeoutS:  s.TimeoutS,
			Billable:  s.Billable,
		})
	}

	resp, err := h.client.CreateStrategy(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, resp)
}

// UpdateStrategy обрабатывает PUT /admin/delivery-strategies/{id}
func (h *CascadeStrategyHandlers) UpdateStrategy(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := uuid.Parse(id); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid strategy_id format"))
		return
	}
	var req createStrategyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}

	grpcReq := &cascadev1.UpdateStrategyRequest{
		StrategyId:  id,
		Name:        req.Name,
		Description: req.Description,
		Mode:        req.Mode,
	}
	for _, s := range req.Steps {
		grpcReq.Steps = append(grpcReq.Steps, &cascadev1.CreateStrategyStepInput{
			ChannelId: s.ChannelID,
			StepOrder: s.StepOrder,
			TimeoutS:  s.TimeoutS,
			Billable:  s.Billable,
		})
	}

	resp, err := h.client.UpdateStrategy(r.Context(), grpcReq)
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// DeleteStrategy обрабатывает DELETE /admin/delivery-strategies/{id}
func (h *CascadeStrategyHandlers) DeleteStrategy(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if _, err := uuid.Parse(id); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid strategy_id format"))
		return
	}
	resp, err := h.client.DeleteStrategy(r.Context(), &cascadev1.DeleteStrategyRequest{StrategyId: id})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": resp.Success})
}

// GetOperatorChannelSupport обрабатывает GET /admin/operator-channel-support
func (h *CascadeStrategyHandlers) GetOperatorChannelSupport(w http.ResponseWriter, r *http.Request) {
	operatorID := r.URL.Query().Get("operator_id")
	resp, err := h.client.GetOperatorChannelSupport(r.Context(), &cascadev1.GetOCSRequest{
		OperatorId: operatorID,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": resp.Entries,
	})
}

type updateOCSRequest struct {
	OperatorID  string `json:"operator_id"`
	ChannelType string `json:"channel_type"`
	Supported   bool   `json:"supported"`
	Notes       string `json:"notes,omitempty"`
}

// UpdateOperatorChannelSupport обрабатывает PUT /admin/operator-channel-support
func (h *CascadeStrategyHandlers) UpdateOperatorChannelSupport(w http.ResponseWriter, r *http.Request) {
	var req updateOCSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if req.OperatorID == "" || req.ChannelType == "" {
		respondError(w, shared.ErrInvalidInput("operator_id and channel_type are required"))
		return
	}

	resp, err := h.client.UpdateOperatorChannelSupport(r.Context(), &cascadev1.UpdateOCSRequest{
		OperatorId:  req.OperatorID,
		ChannelType: req.ChannelType,
		Supported:   req.Supported,
		Notes:       req.Notes,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp.Entry)
}

package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/smpp-server/smpp-server/internal/gateway/admin/services"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// HierarchicalPeriodsHandler handles the dimension-based periods API.
type HierarchicalPeriodsHandler struct {
	svc *services.PeriodService
}

// NewHierarchicalPeriodsHandler creates the handler.
func NewHierarchicalPeriodsHandler(db *storage.DB) *HierarchicalPeriodsHandler {
	return &HierarchicalPeriodsHandler{svc: services.NewPeriodService(db)}
}

// periodResponse is the JSON shape returned to the frontend.
type periodResponse struct {
	ID             string  `json:"id"`
	CountryID      *string `json:"country_id"`
	OperatorID     *string `json:"operator_id"`
	SenderCategory *string `json:"sender_category"`
	TrafficType    *string `json:"traffic_type"`
	ClientID       *string `json:"client_id"`
	ScopeKey       string  `json:"scope_key"`
	ScopePriority  int     `json:"scope_priority"`
	Strategy       string  `json:"strategy"`
	StartDate      string  `json:"start_date"`
	EndDate        *string `json:"end_date"`
	CreatedAt      string  `json:"created_at"`
}

func periodToResponse(p *services.HierarchicalPeriod) periodResponse {
	r := periodResponse{
		ID:            p.ID.String(),
		ScopeKey:      p.ScopeKey,
		ScopePriority: p.ScopePriority,
		Strategy:      p.Strategy,
		StartDate:     p.StartDate.Format("2006-01-02"),
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
	}
	if p.CountryID != nil {
		s := p.CountryID.String()
		r.CountryID = &s
	}
	if p.OperatorID != nil {
		s := p.OperatorID.String()
		r.OperatorID = &s
	}
	if p.ClientID != nil {
		s := p.ClientID.String()
		r.ClientID = &s
	}
	r.SenderCategory = p.SenderCategory
	r.TrafficType = p.TrafficType
	if p.EndDate != nil {
		s := p.EndDate.Format("2006-01-02")
		r.EndDate = &s
	}
	return r
}

// parseOptionalUUID returns nil for empty string, or parses the UUID.
func parseOptionalUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// parseOptionalString returns nil for empty string.
func parseOptionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ListPeriods handles GET /admin/v1/tarification/periods
func (h *HierarchicalPeriodsHandler) ListPeriods(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	countryID, err := parseOptionalUUID(q.Get("country_id"))
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid country_id"))
		return
	}
	operatorID, err := parseOptionalUUID(q.Get("operator_id"))
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid operator_id"))
		return
	}
	clientID, err := parseOptionalUUID(q.Get("client_id"))
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid client_id"))
		return
	}

	filter := services.PeriodDimensions{
		CountryID:      countryID,
		OperatorID:     operatorID,
		SenderCategory: parseOptionalString(q.Get("sender_category")),
		TrafficType:    parseOptionalString(q.Get("traffic_type")),
		ClientID:       clientID,
	}

	periods, err := h.svc.ListPeriods(r.Context(), filter)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}

	resp := make([]periodResponse, 0, len(periods))
	for _, p := range periods {
		resp = append(resp, periodToResponse(p))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"periods": resp,
		"total":   len(resp),
	})
}

// createPeriodRequest is the POST body.
type createPeriodRequest struct {
	CountryID      *string `json:"country_id"`
	OperatorID     *string `json:"operator_id"`
	SenderCategory *string `json:"sender_category"`
	TrafficType    *string `json:"traffic_type"`
	ClientID       *string `json:"client_id"`
	Strategy       string  `json:"strategy"`
	StartDate      string  `json:"start_date"` // YYYY-MM-DD
	EndDate        *string `json:"end_date"`   // YYYY-MM-DD or null
}

// CreatePeriod handles POST /admin/v1/tarification/periods
func (h *HierarchicalPeriodsHandler) CreatePeriod(w http.ResponseWriter, r *http.Request) {
	var req createPeriodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if req.Strategy == "" || req.StartDate == "" {
		respondError(w, shared.ErrInvalidInput("strategy and start_date are required"))
		return
	}

	dims, err := parseDimensions(req.CountryID, req.OperatorID, req.SenderCategory, req.TrafficType, req.ClientID)
	if err != nil {
		respondError(w, shared.ErrInvalidInput(err.Error()))
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid start_date format, use YYYY-MM-DD"))
		return
	}

	var endDate *time.Time
	if req.EndDate != nil {
		d, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("invalid end_date format, use YYYY-MM-DD"))
			return
		}
		endDate = &d
	}

	period, warning, err := h.svc.CreatePeriod(r.Context(), services.CreatePeriodInput{
		PeriodDimensions: dims,
		Strategy:         req.Strategy,
		StartDate:        startDate,
		EndDate:          endDate,
	})
	if err != nil {
		respondPeriodError(w, err)
		return
	}

	body := map[string]interface{}{
		"period": periodToResponse(period),
	}
	if warning != nil {
		body["auto_close_warning"] = map[string]interface{}{
			"period_id":    warning.PeriodID.String(),
			"new_end_date": warning.NewEndDate.Format("2006-01-02"),
		}
	}
	respondJSON(w, http.StatusCreated, body)
}

// updatePeriodRequest is the PUT body.
type updatePeriodRequest struct {
	EndDate  *string `json:"end_date"` // YYYY-MM-DD or null
	Strategy *string `json:"strategy"`
}

// UpdatePeriod handles PUT /admin/v1/tarification/periods/{id}
func (h *HierarchicalPeriodsHandler) UpdatePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period id"))
		return
	}

	var req updatePeriodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}

	in := services.UpdatePeriodInput{Strategy: req.Strategy}
	if req.EndDate != nil {
		d, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("invalid end_date format"))
			return
		}
		in.EndDate = &d
	}

	period, err := h.svc.UpdatePeriod(r.Context(), id, in)
	if err != nil {
		respondPeriodError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"period": periodToResponse(period)})
}

// DeletePeriod handles DELETE /admin/v1/tarification/periods/{id}
func (h *HierarchicalPeriodsHandler) DeletePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period id"))
		return
	}
	if err := h.svc.DeletePeriod(r.Context(), id); err != nil {
		respondPeriodError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseDimensions converts nullable string pointers to uuid.UUID pointers.
func parseDimensions(countryStr, operatorStr, senderCat, trafficType, clientStr *string) (services.PeriodDimensions, error) {
	d := services.PeriodDimensions{
		SenderCategory: senderCat,
		TrafficType:    trafficType,
	}
	if countryStr != nil {
		id, err := parseOptionalUUID(*countryStr)
		if err != nil {
			return d, fmt.Errorf("invalid country_id: %w", err)
		}
		d.CountryID = id
	}
	if operatorStr != nil {
		id, err := parseOptionalUUID(*operatorStr)
		if err != nil {
			return d, fmt.Errorf("invalid operator_id: %w", err)
		}
		d.OperatorID = id
	}
	if clientStr != nil {
		id, err := parseOptionalUUID(*clientStr)
		if err != nil {
			return d, fmt.Errorf("invalid client_id: %w", err)
		}
		d.ClientID = id
	}
	return d, nil
}

// respondPeriodError maps service errors to HTTP status codes.
func respondPeriodError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrPeriodNotFound):
		respondError(w, shared.ErrNotFound(err.Error()))
	case errors.Is(err, services.ErrMissingCountry),
		errors.Is(err, services.ErrMissingOperator),
		errors.Is(err, services.ErrMissingSenderCategory),
		errors.Is(err, services.ErrRetroactiveStart),
		errors.Is(err, services.ErrInvalidStrategy),
		errors.Is(err, services.ErrInvalidSenderCategory),
		errors.Is(err, services.ErrInvalidTrafficType):
		respondError(w, shared.ErrInvalidInput(err.Error()))
	case errors.Is(err, services.ErrNoParentPeriod),
		errors.Is(err, services.ErrChildPeriodExceedsParent),
		errors.Is(err, services.ErrChildPeriodsBlock),
		errors.Is(err, services.ErrHasChildPeriods),
		errors.Is(err, services.ErrPeriodOverlap):
		respondError(w, shared.ErrConflict(err.Error()))
	default:
		respondError(w, shared.ErrInternalServer(err.Error()))
	}
}

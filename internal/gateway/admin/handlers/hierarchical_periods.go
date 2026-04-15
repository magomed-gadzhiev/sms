package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
	db  *storage.DB
}

// NewHierarchicalPeriodsHandler creates the handler.
func NewHierarchicalPeriodsHandler(db *storage.DB) *HierarchicalPeriodsHandler {
	return &HierarchicalPeriodsHandler{svc: services.NewPeriodService(db), db: db}
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

// tierResponse is the JSON shape for a tariff_tiers_new row.
type tierResponse struct {
	ID             string `json:"id"`
	TariffPeriodID string `json:"tariff_period_id"`
	FromCount      int    `json:"from_count"`
	PricePerSegment string `json:"price_per_segment"`
	CreatedAt      string `json:"created_at"`
}

// ListPeriodTiers handles GET /admin/v1/tarification/periods/{id}/tiers
func (h *HierarchicalPeriodsHandler) ListPeriodTiers(w http.ResponseWriter, r *http.Request) {
	periodID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period id"))
		return
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id::text, tariff_period_id::text, from_count, price_per_segment::text, created_at
		 FROM tariff_tiers_new WHERE tariff_period_id = $1 ORDER BY from_count ASC`, periodID)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	defer rows.Close()
	tiers := make([]tierResponse, 0)
	for rows.Next() {
		var t tierResponse
		var createdAt time.Time
		if err := rows.Scan(&t.ID, &t.TariffPeriodID, &t.FromCount, &t.PricePerSegment, &createdAt); err != nil {
			respondError(w, shared.ErrInternalServer(err.Error()))
			return
		}
		t.CreatedAt = createdAt.Format(time.RFC3339)
		tiers = append(tiers, t)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"tiers": tiers, "total": len(tiers)})
}

// createPeriodTierRequest is the POST body for tier creation.
type createPeriodTierRequest struct {
	FromCount       int    `json:"from_count"`
	PricePerSegment string `json:"price_per_segment"`
}

// CreatePeriodTier handles POST /admin/v1/tarification/periods/{id}/tiers
func (h *HierarchicalPeriodsHandler) CreatePeriodTier(w http.ResponseWriter, r *http.Request) {
	periodID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period id"))
		return
	}
	var req createPeriodTierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if req.PricePerSegment == "" {
		respondError(w, shared.ErrInvalidInput("price_per_segment is required"))
		return
	}
	if req.FromCount < 0 {
		respondError(w, shared.ErrInvalidInput("from_count must be >= 0"))
		return
	}
	var t tierResponse
	var createdAt time.Time
	err = h.db.QueryRowContext(r.Context(),
		`INSERT INTO tariff_tiers_new (tariff_period_id, from_count, price_per_segment)
		 VALUES ($1, $2, $3)
		 RETURNING id::text, tariff_period_id::text, from_count, price_per_segment::text, created_at`,
		periodID, req.FromCount, req.PricePerSegment,
	).Scan(&t.ID, &t.TariffPeriodID, &t.FromCount, &t.PricePerSegment, &createdAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			respondError(w, shared.ErrConflict(fmt.Sprintf("tier with from_count=%d already exists in this period", req.FromCount)))
			return
		}
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "violates") {
			respondError(w, shared.ErrNotFound("period not found"))
			return
		}
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"tier": t})
}

// updatePeriodTierRequest is the PUT body.
type updatePeriodTierRequest struct {
	FromCount       int    `json:"from_count"`
	PricePerSegment string `json:"price_per_segment"`
}

// UpdatePeriodTier handles PUT /admin/v1/tarification/periods/{id}/tiers/{tier_id}
func (h *HierarchicalPeriodsHandler) UpdatePeriodTier(w http.ResponseWriter, r *http.Request) {
	periodID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period id"))
		return
	}
	tierID, err := uuid.Parse(mux.Vars(r)["tier_id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid tier id"))
		return
	}
	var req updatePeriodTierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("invalid request body"))
		return
	}
	if req.PricePerSegment == "" {
		respondError(w, shared.ErrInvalidInput("price_per_segment is required"))
		return
	}
	var t tierResponse
	var createdAt time.Time
	err = h.db.QueryRowContext(r.Context(),
		`UPDATE tariff_tiers_new SET from_count=$1, price_per_segment=$2
		 WHERE id=$3 AND tariff_period_id=$4
		 RETURNING id::text, tariff_period_id::text, from_count, price_per_segment::text, created_at`,
		req.FromCount, req.PricePerSegment, tierID, periodID,
	).Scan(&t.ID, &t.TariffPeriodID, &t.FromCount, &t.PricePerSegment, &createdAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			respondError(w, shared.ErrConflict(fmt.Sprintf("tier with from_count=%d already exists in this period", req.FromCount)))
			return
		}
		respondError(w, shared.ErrNotFound("tier not found"))
		return
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	respondJSON(w, http.StatusOK, map[string]interface{}{"tier": t})
}

// DeletePeriodTier handles DELETE /admin/v1/tarification/periods/{id}/tiers/{tier_id}
func (h *HierarchicalPeriodsHandler) DeletePeriodTier(w http.ResponseWriter, r *http.Request) {
	periodID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid period id"))
		return
	}
	tierID, err := uuid.Parse(mux.Vars(r)["tier_id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("invalid tier id"))
		return
	}
	res, err := h.db.ExecContext(r.Context(),
		`DELETE FROM tariff_tiers_new WHERE id=$1 AND tariff_period_id=$2`, tierID, periodID)
	if err != nil {
		respondError(w, shared.ErrInternalServer(err.Error()))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("tier not found"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

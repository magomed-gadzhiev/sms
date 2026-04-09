package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/services/routing/domain"
	"github.com/smpp-server/smpp-server/internal/services/routing/infrastructure"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// RouteHandlers provides HTTP handlers for admin route CRUD.
type RouteHandlers struct {
	repo *infrastructure.RouteRepo
	pool *pgxpool.Pool
}

// NewRouteHandlers creates a new RouteHandlers instance.
func NewRouteHandlers(repo *infrastructure.RouteRepo, pool *pgxpool.Pool) *RouteHandlers {
	return &RouteHandlers{repo: repo, pool: pool}
}

// ---- JSON request / response types ----

type routeRequest struct {
	ClientID        *string              `json:"client_id"`
	Name            string               `json:"name"`
	Comment         string               `json:"comment"`
	Status          string               `json:"status"`
	RouteType       string               `json:"route_type"`
	OperatorID      *string              `json:"operator_id"`
	ProviderID      string               `json:"provider_id"`
	Priority        int                  `json:"priority"`
	Share           int                  `json:"share"`
	ConditionGroups []conditionGroupReq  `json:"condition_groups"`
	Schedules       []scheduleReq        `json:"schedules"`
}

type conditionGroupReq struct {
	LogicOp    string         `json:"logic_op"`
	Conditions []conditionReq `json:"conditions"`
}

type conditionReq struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type scheduleReq struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	TimeFrom string `json:"time_from"`
	TimeTo   string `json:"time_to"`
	Weekdays int    `json:"weekdays"`
	Timezone string `json:"timezone"`
}

type routeResponse struct {
	ID              uuid.UUID              `json:"id"`
	ClientID        *uuid.UUID             `json:"client_id"`
	Name            string                 `json:"name"`
	Comment         string                 `json:"comment"`
	Status          string                 `json:"status"`
	RouteType       string                 `json:"route_type"`
	OperatorID      *uuid.UUID             `json:"operator_id"`
	ProviderID      uuid.UUID              `json:"provider_id"`
	Priority        int                    `json:"priority"`
	Share           int                    `json:"share"`
	ConditionGroups []conditionGroupResp   `json:"condition_groups"`
	Schedules       []scheduleResp         `json:"schedules"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type conditionGroupResp struct {
	LogicOp    string          `json:"logic_op"`
	Conditions []conditionResp `json:"conditions"`
}

type conditionResp struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type scheduleResp struct {
	DateFrom *string `json:"date_from"`
	DateTo   *string `json:"date_to"`
	TimeFrom *string `json:"time_from"`
	TimeTo   *string `json:"time_to"`
	Weekdays int     `json:"weekdays"`
	Timezone string  `json:"timezone"`
}

type routeListItem struct {
	ID              uuid.UUID  `json:"id"`
	ClientID        *uuid.UUID `json:"client_id"`
	Name            string     `json:"name"`
	Comment         string     `json:"comment"`
	Status          string     `json:"status"`
	RouteType       string     `json:"route_type"`
	ProviderID      uuid.UUID  `json:"provider_id"`
	ProviderName    string     `json:"provider_name"`
	Priority        int        `json:"priority"`
	Share           int        `json:"share"`
	ConditionTags   []string   `json:"condition_tags"`
	ScheduleSummary string     `json:"schedule_summary"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ---- Handlers ----

// CreateRoute POST /portal/v1/routes
func (h *RouteHandlers) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var req routeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if appErr := validateRouteRequest(&req); appErr != nil {
		respondError(w, appErr)
		return
	}

	route, appErr := requestToRoute(&req)
	if appErr != nil {
		respondError(w, appErr)
		return
	}

	if err := h.repo.Create(r.Context(), route); err != nil {
		log.Error().Err(err).Msg("failed to create route")
		respondError(w, shared.ErrInternalServer("Ошибка создания маршрута"))
		return
	}

	// Re-read to get DB-generated timestamps
	created, err := h.repo.GetByID(r.Context(), route.ID)
	if err != nil {
		log.Error().Err(err).Msg("failed to re-read created route")
		respondJSON(w, http.StatusCreated, routeToResponse(route))
		return
	}
	respondJSON(w, http.StatusCreated, routeToResponse(created))
}

// ListRoutes GET /portal/v1/routes
func (h *RouteHandlers) ListRoutes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filters := infrastructure.RouteFilters{
		Status:    q.Get("status"),
		RouteType: q.Get("route_type"),
	}

	if cidStr := q.Get("client_id"); cidStr != "" {
		cid, err := uuid.Parse(cidStr)
		if err != nil {
			respondError(w, shared.ErrInvalidInput("Неверный client_id"))
			return
		}
		filters.ClientID = &cid
	}

	if q.Get("default") == "true" {
		filters.DefaultOnly = true
	}

	routes, total, err := h.repo.List(r.Context(), filters)
	if err != nil {
		log.Error().Err(err).Msg("failed to list routes")
		respondError(w, shared.ErrInternalServer("Ошибка получения маршрутов"))
		return
	}

	// Load children for each route to build condition_tags and schedule_summary
	fullRoutes := make([]*domain.ClientRoute, 0, len(routes))
	for _, route := range routes {
		full, err := h.repo.GetByID(r.Context(), route.ID)
		if err != nil {
			log.Error().Err(err).Str("route_id", route.ID.String()).Msg("failed to load route children")
			fullRoutes = append(fullRoutes, route)
			continue
		}
		fullRoutes = append(fullRoutes, full)
	}

	// Collect unique provider IDs and operator condition IDs for batch name lookup
	providerIDSet := map[uuid.UUID]struct{}{}
	operatorIDSet := map[uuid.UUID]struct{}{}
	for _, route := range fullRoutes {
		providerIDSet[route.ProviderID] = struct{}{}
		for _, g := range route.Groups {
			for _, c := range g.Conditions {
				if c.Type == domain.ConditionOperator {
					if id, err := uuid.Parse(c.Value); err == nil {
						operatorIDSet[id] = struct{}{}
					}
				}
			}
		}
	}
	providerIDs := make([]uuid.UUID, 0, len(providerIDSet))
	for id := range providerIDSet {
		providerIDs = append(providerIDs, id)
	}
	operatorIDs := make([]uuid.UUID, 0, len(operatorIDSet))
	for id := range operatorIDSet {
		operatorIDs = append(operatorIDs, id)
	}
	providerNames := h.fetchProviderNames(r.Context(), providerIDs)
	operatorNames := h.fetchOperatorNames(r.Context(), operatorIDs)

	items := make([]routeListItem, 0, len(fullRoutes))
	for _, route := range fullRoutes {
		items = append(items, routeToListItem(route, providerNames, operatorNames))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"routes": items,
		"total":  total,
	})
}

// GetRoute GET /portal/v1/routes/{id}
func (h *RouteHandlers) GetRoute(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат ID"))
		return
	}

	route, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrClientRouteNotFound) {
			respondError(w, shared.ErrNotFound("Маршрут"))
			return
		}
		log.Error().Err(err).Msg("failed to get route")
		respondError(w, shared.ErrInternalServer("Ошибка получения маршрута"))
		return
	}

	respondJSON(w, http.StatusOK, routeToResponse(route))
}

// UpdateRoute PUT /portal/v1/routes/{id}
func (h *RouteHandlers) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат ID"))
		return
	}

	var req routeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if appErr := validateRouteRequest(&req); appErr != nil {
		respondError(w, appErr)
		return
	}

	route, appErr := requestToRoute(&req)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	route.ID = id

	if err := h.repo.Update(r.Context(), route); err != nil {
		if errors.Is(err, domain.ErrClientRouteNotFound) {
			respondError(w, shared.ErrNotFound("Маршрут"))
			return
		}
		log.Error().Err(err).Msg("failed to update route")
		respondError(w, shared.ErrInternalServer("Ошибка обновления маршрута"))
		return
	}

	updated, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Msg("failed to re-read updated route")
		respondJSON(w, http.StatusOK, routeToResponse(route))
		return
	}
	respondJSON(w, http.StatusOK, routeToResponse(updated))
}

// DeleteRoute DELETE /portal/v1/routes/{id}
func (h *RouteHandlers) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат ID"))
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrClientRouteNotFound) {
			respondError(w, shared.ErrNotFound("Маршрут"))
			return
		}
		log.Error().Err(err).Msg("failed to delete route")
		respondError(w, shared.ErrInternalServer("Ошибка удаления маршрута"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetReferences GET /portal/v1/routes/references
func (h *RouteHandlers) GetReferences(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"route_types":     []string{"sms", "hlr", "max"},
		"statuses":        []string{"active", "draft"},
		"condition_types": []string{"operator", "country", "traffic_type", "paid_name", "regex"},
		"logic_ops":       []string{"IF", "AND", "AND_NOT", "OR", "OR_NOT"},
		"traffic_types":   []string{"authorization", "transactional", "service"},
	})
}

// ---- Validation ----

func validateRouteRequest(req *routeRequest) *shared.AppError {
	if req.ProviderID == "" {
		return shared.ErrInvalidInput("provider_id обязателен")
	}
	if _, err := uuid.Parse(req.ProviderID); err != nil {
		return shared.ErrInvalidInput("Неверный формат provider_id")
	}

	if req.RouteType == "" {
		req.RouteType = "sms"
	}
	if req.RouteType != "sms" && req.RouteType != "hlr" && req.RouteType != "max" {
		return shared.ErrInvalidInput("route_type должен быть sms, hlr или max")
	}

	if req.Status == "" {
		req.Status = "active"
	}
	if !domain.ValidRouteStatus(req.Status) {
		return shared.ErrInvalidInput("status должен быть active или draft")
	}

	if req.Share == 0 {
		req.Share = 100
	}

	if req.ClientID != nil && *req.ClientID != "" {
		if _, err := uuid.Parse(*req.ClientID); err != nil {
			return shared.ErrInvalidInput("Неверный формат client_id")
		}
	}

	if req.OperatorID != nil && *req.OperatorID != "" {
		if _, err := uuid.Parse(*req.OperatorID); err != nil {
			return shared.ErrInvalidInput("Неверный формат operator_id")
		}
	}

	for i, g := range req.ConditionGroups {
		if !domain.ValidLogicOp(g.LogicOp) {
			return shared.ErrInvalidInput(fmt.Sprintf("condition_groups[%d]: неверный logic_op %q", i, g.LogicOp))
		}
		for j, c := range g.Conditions {
			if !domain.ValidConditionType(c.Type) {
				return shared.ErrInvalidInput(fmt.Sprintf("condition_groups[%d].conditions[%d]: неверный type %q", i, j, c.Type))
			}
		}
	}

	return nil
}

// ---- Conversion helpers ----

func requestToRoute(req *routeRequest) (*domain.ClientRoute, *shared.AppError) {
	providerID, _ := uuid.Parse(req.ProviderID)

	route := &domain.ClientRoute{
		ID:         uuid.New(),
		ProviderID: providerID,
		Name:       req.Name,
		Comment:    req.Comment,
		Status:     domain.RouteStatus(req.Status),
		RouteType:  req.RouteType,
		Priority:   req.Priority,
		Share:      req.Share,
		Weight:     req.Share,
		Active:     domain.RouteStatus(req.Status) == domain.RouteStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if req.ClientID != nil && *req.ClientID != "" {
		cid, _ := uuid.Parse(*req.ClientID)
		route.ClientID = &cid
	}

	if req.OperatorID != nil && *req.OperatorID != "" {
		oid, _ := uuid.Parse(*req.OperatorID)
		route.OperatorID = &oid
	}

	for i, g := range req.ConditionGroups {
		group := domain.ConditionGroup{
			GroupIndex: i,
			LogicOp:    domain.LogicOp(g.LogicOp),
		}
		for _, c := range g.Conditions {
			group.Conditions = append(group.Conditions, domain.Condition{
				Type:  domain.ConditionType(c.Type),
				Value: c.Value,
			})
		}
		route.Groups = append(route.Groups, group)
	}

	for _, s := range req.Schedules {
		sched := domain.Schedule{
			Weekdays: s.Weekdays,
			Timezone: s.Timezone,
		}
		if s.DateFrom != "" {
			t, err := time.Parse("2006-01-02", s.DateFrom)
			if err != nil {
				return nil, shared.ErrInvalidInput("Неверный формат date_from, ожидается YYYY-MM-DD")
			}
			sched.DateFrom = &t
		}
		if s.DateTo != "" {
			t, err := time.Parse("2006-01-02", s.DateTo)
			if err != nil {
				return nil, shared.ErrInvalidInput("Неверный формат date_to, ожидается YYYY-MM-DD")
			}
			sched.DateTo = &t
		}
		if s.TimeFrom != "" {
			sched.TimeFrom = &s.TimeFrom
		}
		if s.TimeTo != "" {
			sched.TimeTo = &s.TimeTo
		}
		route.Schedules = append(route.Schedules, sched)
	}

	return route, nil
}

func routeToResponse(route *domain.ClientRoute) routeResponse {
	resp := routeResponse{
		ID:         route.ID,
		ClientID:   route.ClientID,
		Name:       route.Name,
		Comment:    route.Comment,
		Status:     string(route.Status),
		RouteType:  route.RouteType,
		OperatorID: route.OperatorID,
		ProviderID: route.ProviderID,
		Priority:   route.Priority,
		Share:      route.Share,
		CreatedAt:  route.CreatedAt,
		UpdatedAt:  route.UpdatedAt,
	}

	resp.ConditionGroups = make([]conditionGroupResp, 0, len(route.Groups))
	for _, g := range route.Groups {
		gr := conditionGroupResp{
			LogicOp: string(g.LogicOp),
		}
		gr.Conditions = make([]conditionResp, 0, len(g.Conditions))
		for _, c := range g.Conditions {
			gr.Conditions = append(gr.Conditions, conditionResp{
				Type:  string(c.Type),
				Value: c.Value,
			})
		}
		resp.ConditionGroups = append(resp.ConditionGroups, gr)
	}

	resp.Schedules = make([]scheduleResp, 0, len(route.Schedules))
	for _, s := range route.Schedules {
		sr := scheduleResp{
			Weekdays: s.Weekdays,
			Timezone: s.Timezone,
		}
		if s.DateFrom != nil {
			v := s.DateFrom.Format("2006-01-02")
			sr.DateFrom = &v
		}
		if s.DateTo != nil {
			v := s.DateTo.Format("2006-01-02")
			sr.DateTo = &v
		}
		sr.TimeFrom = s.TimeFrom
		sr.TimeTo = s.TimeTo
		resp.Schedules = append(resp.Schedules, sr)
	}

	return resp
}

func routeToListItem(route *domain.ClientRoute, providerNames, operatorNames map[uuid.UUID]string) routeListItem {
	item := routeListItem{
		ID:           route.ID,
		ClientID:     route.ClientID,
		Name:         route.Name,
		Comment:      route.Comment,
		Status:       string(route.Status),
		RouteType:    route.RouteType,
		ProviderID:   route.ProviderID,
		ProviderName: providerNames[route.ProviderID],
		Priority:     route.Priority,
		Share:        route.Share,
		CreatedAt:    route.CreatedAt,
		UpdatedAt:    route.UpdatedAt,
	}

	// Build condition_tags from groups, resolving operator UUIDs to names
	tags := make([]string, 0)
	for _, g := range route.Groups {
		for _, c := range g.Conditions {
			label := conditionTypeLabel(c.Type)
			value := c.Value
			if c.Type == domain.ConditionOperator {
				if id, err := uuid.Parse(c.Value); err == nil {
					if name, ok := operatorNames[id]; ok {
						value = name
					}
				}
			}
			tags = append(tags, fmt.Sprintf("%s: %s", label, value))
		}
	}
	item.ConditionTags = tags

	// Build schedule_summary
	if len(route.Schedules) > 0 {
		item.ScheduleSummary = formatScheduleSummary(route.Schedules[0])
	}

	return item
}

func conditionTypeLabel(t domain.ConditionType) string {
	switch t {
	case domain.ConditionOperator:
		return "Оператор"
	case domain.ConditionCountry:
		return "Страна"
	case domain.ConditionTrafficType:
		return "Тип трафика"
	case domain.ConditionPaidName:
		return "Платное имя"
	case domain.ConditionRegex:
		return "Regex"
	default:
		return string(t)
	}
}

func (h *RouteHandlers) fetchProviderNames(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	result := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return result
	}
	rows, err := h.pool.Query(ctx, `SELECT id, name FROM providers WHERE id = ANY($1)`, ids)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch provider names")
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			result[id] = name
		}
	}
	return result
}

func (h *RouteHandlers) fetchOperatorNames(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	result := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return result
	}
	rows, err := h.pool.Query(ctx, `SELECT id, name FROM operators WHERE id = ANY($1)`, ids)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch operator names")
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			result[id] = name
		}
	}
	return result
}

// weekdayNames maps bit positions to short Russian weekday names.
var weekdayNames = []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}

func formatScheduleSummary(s domain.Schedule) string {
	var parts []string

	// Weekdays bitmask: bit 0 = Monday, bit 6 = Sunday
	if s.Weekdays > 0 {
		days := make([]string, 0)
		for i := 0; i < 7; i++ {
			if s.Weekdays&(1<<i) != 0 {
				days = append(days, weekdayNames[i])
			}
		}
		// Try to form a range like "Пн-Пт"
		if len(days) > 2 {
			// Check if consecutive
			first := -1
			last := -1
			consecutive := true
			for i := 0; i < 7; i++ {
				if s.Weekdays&(1<<i) != 0 {
					if first == -1 {
						first = i
					}
					if last != -1 && i != last+1 {
						consecutive = false
					}
					last = i
				}
			}
			if consecutive && first != last {
				parts = append(parts, fmt.Sprintf("%s-%s", weekdayNames[first], weekdayNames[last]))
			} else {
				parts = append(parts, strings.Join(days, ", "))
			}
		} else {
			parts = append(parts, strings.Join(days, ", "))
		}
	}

	// Time range
	if s.TimeFrom != nil && s.TimeTo != nil {
		parts = append(parts, fmt.Sprintf("%s-%s", *s.TimeFrom, *s.TimeTo))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

package handlers

// /reseller/network/route-sets/{id}/items — items route-set'а агрегатора.
//
// CRUD + Duplicate + Reorder. После любой mutating-операции вызывается
// RouteSetMaterializer.ApplyToAllSubscribers(setID), чтобы пересобрать
// client_routes (source='template') у всех подписанных суб-аккаунтов.
//
// Provider-инвариант (spec §6.1): provider_id, упомянутый в правиле, должен
// присутствовать в provider-set'е каждого подписанного суб-аккаунта. Нарушение —
// 409 с details {"kind":"route_uses_unavailable_provider","items":[...]}.
// Materialize шаги (3) и (4) не атомарны — partial-failure compromise тот же,
// что в network_provider_set_items.go.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/services/network"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// NetworkRouteSetItemsHandlers обрабатывает /portal/v1/reseller/network/route-sets/{id}/items.
type NetworkRouteSetItemsHandlers struct {
	pool         *pgxpool.Pool
	setRepo      *storage.ResellerRouteSetRepository
	itemsRepo    *storage.ResellerRouteSetItemsRepository
	materializer *network.RouteSetMaterializer
	validator    *network.ConflictValidator
}

// NewNetworkRouteSetItemsHandlers конструирует handler.
func NewNetworkRouteSetItemsHandlers(pool *pgxpool.Pool, mat *network.RouteSetMaterializer, validator *network.ConflictValidator) *NetworkRouteSetItemsHandlers {
	return &NetworkRouteSetItemsHandlers{
		pool:         pool,
		setRepo:      storage.NewResellerRouteSetRepository(pool),
		itemsRepo:    storage.NewResellerRouteSetItemsRepository(pool),
		materializer: mat,
		validator:    validator,
	}
}

// reseller извлекает client_id из контекста.
func (h *NetworkRouteSetItemsHandlers) reseller(r *http.Request) (uuid.UUID, *shared.AppError) {
	cid, ok := middleware.GetClientID(r.Context())
	if !ok || cid == uuid.Nil {
		return uuid.Nil, shared.ErrUnauthorized("Клиент не найден")
	}
	return cid, nil
}

// verifyOwnership: чужой set → 404 (не светим существование).
func (h *NetworkRouteSetItemsHandlers) verifyOwnership(ctx context.Context, resellerID, setID uuid.UUID) *shared.AppError {
	set, err := h.setRepo.GetByID(ctx, setID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return shared.ErrNotFound("route-set")
		}
		log.Error().Err(err).Msg("route-set-items ownership lookup")
		return shared.ErrInternalServer("ошибка чтения route-set")
	}
	if set.ResellerID != resellerID {
		return shared.ErrNotFound("route-set")
	}
	return nil
}

// JSON DTO ----------------------------------------------------------------

type conditionIn struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type groupIn struct {
	LogicOp    string        `json:"logic_op"`
	Conditions []conditionIn `json:"conditions"`
}

type scheduleIn struct {
	DateFrom *string `json:"date_from"` // YYYY-MM-DD
	DateTo   *string `json:"date_to"`
	TimeFrom *string `json:"time_from"` // HH:MM или HH:MM:SS
	TimeTo   *string `json:"time_to"`
	Weekdays int16   `json:"weekdays"`
	Timezone string  `json:"timezone"`
}

type itemIn struct {
	Name            string       `json:"name"`
	Comment         string       `json:"comment"`
	ProviderID      string       `json:"provider_id"`
	Priority        int          `json:"priority"`
	Share           int          `json:"share"`
	RouteType       string       `json:"route_type"`
	Status          string       `json:"status"`
	ConditionGroups []groupIn    `json:"condition_groups"`
	Schedules       []scheduleIn `json:"schedules"`
}

type itemOut struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	Comment         string                   `json:"comment"`
	ProviderID      string                   `json:"provider_id"`
	ProviderName    string                   `json:"provider_name"`
	Priority        int                      `json:"priority"`
	Share           int                      `json:"share"`
	RouteType       string                   `json:"route_type"`
	Status          string                   `json:"status"`
	ConditionGroups []map[string]interface{} `json:"condition_groups"`
	Schedules       []map[string]interface{} `json:"schedules"`
}

func groupsOut(groups []storage.RouteSetConditionGroup) []map[string]interface{} {
	out := []map[string]interface{}{}
	for _, g := range groups {
		conds := []map[string]string{}
		for _, c := range g.Conditions {
			conds = append(conds, map[string]string{"type": c.Type, "value": c.Value})
		}
		out = append(out, map[string]interface{}{"logic_op": g.LogicOp, "conditions": conds})
	}
	return out
}

func schedulesOut(scheds []storage.RouteSetSchedule) []map[string]interface{} {
	out := []map[string]interface{}{}
	for _, s := range scheds {
		entry := map[string]interface{}{"weekdays": s.Weekdays, "timezone": s.Timezone}
		if s.DateFrom != nil {
			entry["date_from"] = s.DateFrom.Format("2006-01-02")
		}
		if s.DateTo != nil {
			entry["date_to"] = s.DateTo.Format("2006-01-02")
		}
		if s.TimeFrom != nil {
			entry["time_from"] = *s.TimeFrom
		}
		if s.TimeTo != nil {
			entry["time_to"] = *s.TimeTo
		}
		out = append(out, entry)
	}
	return out
}

// parseItemIn — нормализует JSON-вход в storage.RouteSetItemFull.
func parseItemIn(in itemIn) (storage.RouteSetItemFull, error) {
	provID, err := uuid.Parse(in.ProviderID)
	if err != nil {
		return storage.RouteSetItemFull{}, fmt.Errorf("provider_id invalid")
	}
	out := storage.RouteSetItemFull{
		Name:       in.Name,
		Comment:    in.Comment,
		ProviderID: provID,
		Priority:   in.Priority,
		Share:      in.Share,
		RouteType:  in.RouteType,
		Status:     in.Status,
	}
	if out.RouteType == "" {
		out.RouteType = "sms"
	}
	if out.Status == "" {
		out.Status = "active"
	}
	if out.Share == 0 {
		out.Share = 100
	}
	for idx, g := range in.ConditionGroups {
		grp := storage.RouteSetConditionGroup{GroupIndex: int16(idx), LogicOp: g.LogicOp}
		for _, c := range g.Conditions {
			grp.Conditions = append(grp.Conditions, storage.RouteSetCondition{Type: c.Type, Value: c.Value})
		}
		out.ConditionGroups = append(out.ConditionGroups, grp)
	}
	for _, s := range in.Schedules {
		sched := storage.RouteSetSchedule{Weekdays: s.Weekdays, Timezone: s.Timezone}
		if sched.Weekdays == 0 {
			sched.Weekdays = 127
		}
		if sched.Timezone == "" {
			sched.Timezone = "Europe/Moscow"
		}
		if s.DateFrom != nil {
			d, err := time.Parse("2006-01-02", *s.DateFrom)
			if err != nil {
				return storage.RouteSetItemFull{}, fmt.Errorf("date_from invalid")
			}
			sched.DateFrom = &d
		}
		if s.DateTo != nil {
			d, err := time.Parse("2006-01-02", *s.DateTo)
			if err != nil {
				return storage.RouteSetItemFull{}, fmt.Errorf("date_to invalid")
			}
			sched.DateTo = &d
		}
		if s.TimeFrom != nil {
			tf := *s.TimeFrom
			sched.TimeFrom = &tf
		}
		if s.TimeTo != nil {
			tt := *s.TimeTo
			sched.TimeTo = &tt
		}
		out.Schedules = append(out.Schedules, sched)
	}
	return out, nil
}

// validateProviderForSubscribers — для каждого подписанного на route-set client'а
// проверяет, что providerID присутствует в его provider-set. Возвращает details
// для 409 (или nil, если конфликтов нет).
func (h *NetworkRouteSetItemsHandlers) validateProviderForSubscribers(ctx context.Context, routeSetID, providerID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT sra.client_id, COALESCE(c.name, c.email), sra.provider_set_id, ps.name
		FROM subaccount_routing_assignment sra
		JOIN clients c ON c.id = sra.client_id
		LEFT JOIN reseller_provider_sets ps ON ps.id = sra.provider_set_id
		WHERE sra.route_set_id = $1`, routeSetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var conflicts []map[string]interface{}
	for rows.Next() {
		var clientID uuid.UUID
		var name string
		var psID *uuid.UUID
		var psName *string
		if err := rows.Scan(&clientID, &name, &psID, &psName); err != nil {
			return nil, err
		}
		if psID == nil {
			conflicts = append(conflicts, map[string]interface{}{
				"client_id":            clientID.String(),
				"client_name":          name,
				"current_provider_set": nil,
				"missing":              providerID.String(),
			})
			continue
		}
		var exists bool
		if err := h.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM reseller_provider_set_items WHERE set_id=$1 AND provider_id=$2)`,
			*psID, providerID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			cps := ""
			if psName != nil {
				cps = *psName
			}
			conflicts = append(conflicts, map[string]interface{}{
				"client_id":            clientID.String(),
				"client_name":          name,
				"current_provider_set": cps,
				"missing":              providerID.String(),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return conflicts, nil
}

// Handlers ---------------------------------------------------------------

// List GET /portal/v1/reseller/network/route-sets/{id}/items
func (h *NetworkRouteSetItemsHandlers) List(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if ownErr := h.verifyOwnership(r.Context(), resellerID, setID); ownErr != nil {
		respondError(w, ownErr)
		return
	}

	items, err := h.itemsRepo.ListBySet(r.Context(), setID)
	if err != nil {
		log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items list")
		respondError(w, shared.ErrInternalServer("ошибка чтения items"))
		return
	}

	// Подтянуть имена провайдеров одним запросом.
	providerNames := map[uuid.UUID]string{}
	if len(items) > 0 {
		ids := make([]uuid.UUID, 0, len(items))
		seen := map[uuid.UUID]struct{}{}
		for _, it := range items {
			if _, ok := seen[it.ProviderID]; ok {
				continue
			}
			seen[it.ProviderID] = struct{}{}
			ids = append(ids, it.ProviderID)
		}
		rows, err := h.pool.Query(r.Context(), `SELECT id, name FROM providers WHERE id = ANY($1)`, ids)
		if err == nil {
			for rows.Next() {
				var id uuid.UUID
				var name string
				if err := rows.Scan(&id, &name); err == nil {
					providerNames[id] = name
				}
			}
			rows.Close()
		} else {
			log.Warn().Err(err).Msg("route-set-items provider names lookup")
		}
	}

	out := make([]itemOut, 0, len(items))
	for _, it := range items {
		out = append(out, itemOut{
			ID:              it.ID.String(),
			Name:            it.Name,
			Comment:         it.Comment,
			ProviderID:      it.ProviderID.String(),
			ProviderName:    providerNames[it.ProviderID],
			Priority:        it.Priority,
			Share:           it.Share,
			RouteType:       it.RouteType,
			Status:          it.Status,
			ConditionGroups: groupsOut(it.ConditionGroups),
			Schedules:       schedulesOut(it.Schedules),
		})
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"items": out})
}

// Create POST /portal/v1/reseller/network/route-sets/{id}/items
func (h *NetworkRouteSetItemsHandlers) Create(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if ownErr := h.verifyOwnership(r.Context(), resellerID, setID); ownErr != nil {
		respondError(w, ownErr)
		return
	}

	var in itemIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	full, err := parseItemIn(in)
	if err != nil {
		respondError(w, shared.ErrInvalidInput(err.Error()))
		return
	}

	conflicts, err := h.validateProviderForSubscribers(r.Context(), setID, full.ProviderID)
	if err != nil {
		log.Error().Err(err).Msg("route-set-items create validate")
		respondError(w, shared.ErrInternalServer("ошибка валидации провайдера"))
		return
	}
	if len(conflicts) > 0 {
		details, _ := json.Marshal(map[string]interface{}{
			"kind":  "route_uses_unavailable_provider",
			"items": conflicts,
		})
		respondError(w, shared.ErrConflict("provider правила отсутствует в provider-set подписанного суб-аккаунта").
			WithDetails(string(details)))
		return
	}

	created, err := h.itemsRepo.CreateFull(r.Context(), setID, full)
	if err != nil {
		log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items create")
		respondError(w, shared.ErrInternalServer("ошибка создания item"))
		return
	}
	if h.materializer != nil {
		if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
			log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items create rematerialize")
			respondError(w, shared.ErrInternalServer("rematerialize"))
			return
		}
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "create",
		ResourceType: "route_set_item",
		ResourceID:   created.ID.String(),
		Details: map[string]interface{}{
			"set_id":      setID.String(),
			"name":        full.Name,
			"provider_id": full.ProviderID.String(),
		},
		IPAddress: r.RemoteAddr,
	})
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": created.ID.String()})
}

// Update PUT /portal/v1/reseller/network/route-sets/{id}/items/{item_id}
func (h *NetworkRouteSetItemsHandlers) Update(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if ownErr := h.verifyOwnership(r.Context(), resellerID, setID); ownErr != nil {
		respondError(w, ownErr)
		return
	}
	itemID, perr := uuid.Parse(mux.Vars(r)["item_id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("item_id invalid"))
		return
	}

	var in itemIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	full, err := parseItemIn(in)
	if err != nil {
		respondError(w, shared.ErrInvalidInput(err.Error()))
		return
	}

	conflicts, err := h.validateProviderForSubscribers(r.Context(), setID, full.ProviderID)
	if err != nil {
		log.Error().Err(err).Msg("route-set-items update validate")
		respondError(w, shared.ErrInternalServer("ошибка валидации провайдера"))
		return
	}
	if len(conflicts) > 0 {
		details, _ := json.Marshal(map[string]interface{}{
			"kind":  "route_uses_unavailable_provider",
			"items": conflicts,
		})
		respondError(w, shared.ErrConflict("provider правила отсутствует в provider-set подписанного суб-аккаунта").
			WithDetails(string(details)))
		return
	}

	if err := h.itemsRepo.UpdateFull(r.Context(), setID, itemID, full); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			respondError(w, shared.ErrNotFound("item"))
			return
		}
		log.Error().Err(err).Str("item_id", itemID.String()).Msg("route-set-items update")
		respondError(w, shared.ErrInternalServer("ошибка обновления item"))
		return
	}
	if h.materializer != nil {
		if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
			log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items update rematerialize")
			respondError(w, shared.ErrInternalServer("rematerialize"))
			return
		}
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "update",
		ResourceType: "route_set_item",
		ResourceID:   itemID.String(),
		Details: map[string]interface{}{
			"set_id":      setID.String(),
			"name":        full.Name,
			"provider_id": full.ProviderID.String(),
		},
		IPAddress: r.RemoteAddr,
	})
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": itemID.String()})
}

// Delete DELETE /portal/v1/reseller/network/route-sets/{id}/items/{item_id}
func (h *NetworkRouteSetItemsHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if ownErr := h.verifyOwnership(r.Context(), resellerID, setID); ownErr != nil {
		respondError(w, ownErr)
		return
	}
	itemID, perr := uuid.Parse(mux.Vars(r)["item_id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("item_id invalid"))
		return
	}

	if err := h.itemsRepo.Delete(r.Context(), setID, itemID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			respondError(w, shared.ErrNotFound("item"))
			return
		}
		log.Error().Err(err).Str("item_id", itemID.String()).Msg("route-set-items delete")
		respondError(w, shared.ErrInternalServer("ошибка удаления item"))
		return
	}
	if h.materializer != nil {
		if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
			log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items delete rematerialize")
			respondError(w, shared.ErrInternalServer("rematerialize"))
			return
		}
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "delete",
		ResourceType: "route_set_item",
		ResourceID:   itemID.String(),
		Details:      map[string]interface{}{"set_id": setID.String()},
		IPAddress:    r.RemoteAddr,
	})
	w.WriteHeader(http.StatusNoContent)
}

// Duplicate POST /portal/v1/reseller/network/route-sets/{id}/items/{item_id}/duplicate
//
// Копирует item целиком (с условиями + расписаниями) с инкрементом priority
// и суффиксом " (копия)" к имени. ProviderID сохраняется — повторная валидация
// против подписчиков не нужна (исходный item уже прошёл её при создании).
func (h *NetworkRouteSetItemsHandlers) Duplicate(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if ownErr := h.verifyOwnership(r.Context(), resellerID, setID); ownErr != nil {
		respondError(w, ownErr)
		return
	}
	srcID, perr := uuid.Parse(mux.Vars(r)["item_id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("item_id invalid"))
		return
	}

	src, err := h.itemsRepo.LoadFullByItem(r.Context(), srcID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			respondError(w, shared.ErrNotFound("item"))
			return
		}
		log.Error().Err(err).Str("item_id", srcID.String()).Msg("route-set-items duplicate load")
		respondError(w, shared.ErrInternalServer("ошибка чтения item"))
		return
	}
	if src.SetID != setID {
		// item принадлежит другому set'у — не светим его существование.
		respondError(w, shared.ErrNotFound("item"))
		return
	}
	src.Name = src.Name + " (копия)"
	src.Priority = src.Priority + 1

	created, err := h.itemsRepo.CreateFull(r.Context(), setID, *src)
	if err != nil {
		log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items duplicate")
		respondError(w, shared.ErrInternalServer("ошибка дублирования item"))
		return
	}
	if h.materializer != nil {
		if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
			log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items duplicate rematerialize")
			respondError(w, shared.ErrInternalServer("rematerialize"))
			return
		}
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "create",
		ResourceType: "route_set_item",
		ResourceID:   created.ID.String(),
		Details: map[string]interface{}{
			"set_id":         setID.String(),
			"source_item_id": srcID.String(),
		},
		IPAddress: r.RemoteAddr,
	})
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": created.ID.String()})
}

// reorderEntry — элемент тела PUT /reorder.
type reorderEntry struct {
	ItemID   string `json:"item_id"`
	Priority int    `json:"priority"`
}

// Reorder PUT /portal/v1/reseller/network/route-sets/{id}/items/reorder
//
// Bulk-обновление priority. Запросы вне set'а игнорируются (Reorder в repo
// фильтрует по set_id). После — rematerialize.
func (h *NetworkRouteSetItemsHandlers) Reorder(w http.ResponseWriter, r *http.Request) {
	resellerID, appErr := h.reseller(r)
	if appErr != nil {
		respondError(w, appErr)
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	if ownErr := h.verifyOwnership(r.Context(), resellerID, setID); ownErr != nil {
		respondError(w, ownErr)
		return
	}

	var body struct {
		Items []reorderEntry `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}
	entries := make([]storage.RouteSetReorderEntry, 0, len(body.Items))
	for idx, e := range body.Items {
		id, err := uuid.Parse(e.ItemID)
		if err != nil {
			respondError(w, shared.ErrInvalidInput(fmt.Sprintf("items[%d].item_id invalid", idx)))
			return
		}
		entries = append(entries, storage.RouteSetReorderEntry{ItemID: id, Priority: e.Priority})
	}
	if err := h.itemsRepo.Reorder(r.Context(), setID, entries); err != nil {
		log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items reorder")
		respondError(w, shared.ErrInternalServer("ошибка сохранения порядка"))
		return
	}
	if h.materializer != nil {
		if err := h.materializer.ApplyToAllSubscribers(r.Context(), setID); err != nil {
			log.Error().Err(err).Str("set_id", setID.String()).Msg("route-set-items reorder rematerialize")
			respondError(w, shared.ErrInternalServer("rematerialize"))
			return
		}
	}
	_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
		TenantID:     resellerID,
		UserID:       userIDFromCtx(r.Context()),
		Action:       "reorder",
		ResourceType: "route_set",
		ResourceID:   setID.String(),
		Details:      map[string]interface{}{"item_count": len(entries)},
		IPAddress:    r.RemoteAddr,
	})
	respondJSON(w, http.StatusOK, map[string]interface{}{"set_id": setID.String()})
}

package handlers

// /reseller/network/route-sets/{id}/preview — симулятор matching'а.
//
// POST {phone, sender_id, traffic_type} → []{matched_item_id, item_name,
// provider_id, provider_name, priority} в порядке приоритета items.
//
// Жертва: country определяется простым prefix-mapping'ом по телефону —
// для preview достаточно, реальный pipeline берёт из таблицы operators.

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// NetworkRoutePreviewHandlers обрабатывает POST /preview.
type NetworkRoutePreviewHandlers struct {
	pool      *pgxpool.Pool
	setRepo   *storage.ResellerRouteSetRepository
	itemsRepo *storage.ResellerRouteSetItemsRepository
}

// NewNetworkRoutePreviewHandlers конструирует handler.
func NewNetworkRoutePreviewHandlers(pool *pgxpool.Pool) *NetworkRoutePreviewHandlers {
	return &NetworkRoutePreviewHandlers{
		pool:      pool,
		setRepo:   storage.NewResellerRouteSetRepository(pool),
		itemsRepo: storage.NewResellerRouteSetItemsRepository(pool),
	}
}

// phonePrefixes — упрощённое определение страны по началу номера.
// Реальный matching в pipeline идёт из operators-таблицы по prefix.
var phonePrefixes = map[string]string{
	"7": "RU", "375": "BY", "380": "UA", "996": "KG",
	"998": "UZ", "992": "TJ", "993": "TM", "994": "AZ",
	"995": "GE", "374": "AM", "1": "US", "44": "GB", "49": "DE",
	"33": "FR", "39": "IT", "34": "ES", "86": "CN", "91": "IN",
}

// countryByPhone определяет ISO-код страны по телефону. "" — если не угадали.
func countryByPhone(phone string) string {
	clean := strings.TrimPrefix(strings.TrimSpace(phone), "+")
	// Сначала проверяем 3-значные префиксы (более специфичные), потом 1-2.
	// Простая стратегия: пройти все, выбрать самый длинный матчующий.
	bestPrefix, bestCode := "", ""
	for prefix, code := range phonePrefixes {
		if strings.HasPrefix(clean, prefix) && len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
			bestCode = code
		}
	}
	if bestCode == "" {
		return ""
	}
	// Уточнение для KZ vs RU: оба +7, но KZ начинается с 76/77.
	if bestCode == "RU" && len(clean) >= 2 {
		if clean[1] == '6' || clean[1] == '7' {
			return "KZ"
		}
	}
	return bestCode
}

type previewReq struct {
	Phone       string `json:"phone"`
	SenderID    string `json:"sender_id"`
	TrafficType string `json:"traffic_type"`
}

type previewMatch struct {
	ItemID       string `json:"matched_item_id"`
	ItemName     string `json:"item_name"`
	ProviderID   string `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	Priority     int    `json:"priority"`
}

// Preview POST /portal/v1/reseller/network/route-sets/{id}/preview.
func (h *NetworkRoutePreviewHandlers) Preview(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := middleware.GetClientID(r.Context())
	if !ok || resellerID == uuid.Nil {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	setID, perr := uuid.Parse(mux.Vars(r)["id"])
	if perr != nil {
		respondError(w, shared.ErrInvalidInput("id invalid"))
		return
	}
	set, err := h.setRepo.GetByID(r.Context(), setID)
	if err != nil || set.ResellerID != resellerID {
		respondError(w, shared.ErrNotFound("route-set"))
		return
	}

	var req previewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Невалидный JSON"))
		return
	}

	items, err := h.itemsRepo.ListBySet(r.Context(), setID)
	if err != nil {
		log.Error().Err(err).Str("set_id", setID.String()).Msg("preview list items")
		respondError(w, shared.ErrInternalServer("ошибка чтения items"))
		return
	}

	now := time.Now()
	country := countryByPhone(req.Phone)

	// Сначала отфильтровать matched items, чтобы потом одним запросом
	// подтянуть имена провайдеров.
	type matchTmp struct {
		item storage.RouteSetItemFull
	}
	tmp := make([]matchTmp, 0, len(items))
	for _, it := range items {
		if it.Status != "active" {
			continue
		}
		if !matchesConditions(it.ConditionGroups, country, req.TrafficType, req.SenderID, req.Phone) {
			continue
		}
		if !matchesSchedules(it.Schedules, now) {
			continue
		}
		tmp = append(tmp, matchTmp{item: it})
	}

	// Batched provider name lookup.
	providerNames := map[uuid.UUID]string{}
	if len(tmp) > 0 {
		ids := make([]uuid.UUID, 0, len(tmp))
		seen := map[uuid.UUID]struct{}{}
		for _, m := range tmp {
			if _, ok := seen[m.item.ProviderID]; ok {
				continue
			}
			seen[m.item.ProviderID] = struct{}{}
			ids = append(ids, m.item.ProviderID)
		}
		rows, qerr := h.pool.Query(r.Context(), `SELECT id, name FROM providers WHERE id = ANY($1)`, ids)
		if qerr == nil {
			for rows.Next() {
				var id uuid.UUID
				var name string
				if err := rows.Scan(&id, &name); err == nil {
					providerNames[id] = name
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				log.Warn().Err(err).Msg("preview provider names partial read")
			}
		} else {
			log.Warn().Err(qerr).Msg("preview provider names lookup")
		}
	}

	matches := make([]previewMatch, 0, len(tmp))
	for _, m := range tmp {
		matches = append(matches, previewMatch{
			ItemID:       m.item.ID.String(),
			ItemName:     m.item.Name,
			ProviderID:   m.item.ProviderID.String(),
			ProviderName: providerNames[m.item.ProviderID],
			Priority:     m.item.Priority,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"matches": matches})
}

// matchesConditions оценивает группы условий. Группы комбинируются по
// logic_op первой группы; в практике используется одна группа с logic_op=IF.
// Пустой список — match всегда (item без ограничений).
func matchesConditions(groups []storage.RouteSetConditionGroup, country, trafficType, senderID, phone string) bool {
	if len(groups) == 0 {
		return true
	}
	for _, g := range groups {
		groupResult := true
		for _, c := range g.Conditions {
			if !evalCondition(c, country, trafficType, senderID, phone) {
				groupResult = false
				break
			}
		}
		switch g.LogicOp {
		case "IF", "AND":
			if !groupResult {
				return false
			}
		case "AND_NOT":
			if groupResult {
				return false
			}
		case "OR":
			if groupResult {
				return true
			}
		case "OR_NOT":
			if !groupResult {
				return true
			}
		}
	}
	return true
}

// evalCondition оценивает одно условие. operator-matching отложен (требует
// таблицы operators) — в preview всегда true.
func evalCondition(c storage.RouteSetCondition, country, trafficType, senderID, phone string) bool {
	switch c.Type {
	case "country":
		return c.Value == country
	case "traffic_type":
		return c.Value == trafficType
	case "paid_name":
		return c.Value == senderID
	case "regex":
		re, err := regexp.Compile(c.Value)
		if err != nil {
			return false
		}
		return re.MatchString(phone)
	case "operator":
		return true
	}
	return false
}

// matchesSchedules возвращает true, если хотя бы одно расписание содержит now.
// Пустой список — match всегда (24/7).
func matchesSchedules(scheds []storage.RouteSetSchedule, now time.Time) bool {
	if len(scheds) == 0 {
		return true
	}
	for _, s := range scheds {
		loc, err := time.LoadLocation(s.Timezone)
		if err != nil {
			loc = time.UTC
		}
		t := now.In(loc)
		if s.DateFrom != nil && t.Before(*s.DateFrom) {
			continue
		}
		if s.DateTo != nil && t.After(s.DateTo.AddDate(0, 0, 1)) {
			continue
		}
		// weekdays bitmask: бит 0 = Mon, ..., бит 6 = Sun.
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7 // Sun
		}
		mask := int16(1) << (wd - 1)
		if s.Weekdays&mask == 0 {
			continue
		}
		if s.TimeFrom != nil && s.TimeTo != nil {
			cur := t.Format("15:04:05")
			if cur < *s.TimeFrom || cur > *s.TimeTo {
				continue
			}
		}
		return true
	}
	return false
}

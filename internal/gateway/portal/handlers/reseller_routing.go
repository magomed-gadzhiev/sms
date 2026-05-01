package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	routingv1 "github.com/smpp-server/smpp-server/api/proto/routingv1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerRoutingHandlers struct {
	pool          *pgxpool.Pool
	routingClient routingv1.RoutingServiceClient
}

func NewResellerRoutingHandlers(pool *pgxpool.Pool, routingClient routingv1.RoutingServiceClient) *ResellerRoutingHandlers {
	return &ResellerRoutingHandlers{pool: pool, routingClient: routingClient}
}

func (h *ResellerRoutingHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
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

// ListNetworkProviders GET /portal/v1/reseller/routing/providers
func (h *ResellerRoutingHandlers) ListNetworkProviders(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	subAccountFilter := r.URL.Query().Get("sub_account_id")

	query := `SELECT cp.id, cp.client_id, c.name AS sub_account_name,
	                 cp.provider_id, p.name AS provider_name,
	                 cp.active, cp.shared_priority
	          FROM client_providers cp
	          JOIN clients c ON c.id = cp.client_id
	          JOIN providers p ON p.id = cp.provider_id
	          WHERE c.parent_client_id = $1`
	args := []interface{}{resellerID}
	i := 2

	if subAccountFilter != "" {
		query += fmt.Sprintf(" AND cp.client_id = $%d", i)
		args = append(args, subAccountFilter)
		i++
	}
	query += " ORDER BY c.name, cp.shared_priority"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения провайдеров сети")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type providerAssignment struct {
		ID             string `json:"id"`
		ClientID       string `json:"client_id"`
		SubAccountName string `json:"sub_account_name"`
		ProviderID     string `json:"provider_id"`
		ProviderName   string `json:"provider_name"`
		Active         bool   `json:"active"`
		Priority       int    `json:"priority"`
	}
	items := make([]providerAssignment, 0)
	for rows.Next() {
		var p providerAssignment
		if err := rows.Scan(&p.ID, &p.ClientID, &p.SubAccountName,
			&p.ProviderID, &p.ProviderName, &p.Active, &p.Priority); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, p)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"providers": items, "total": len(items)})
}

// ListNetworkRoutes GET /portal/v1/reseller/routing/routes
func (h *ResellerRoutingHandlers) ListNetworkRoutes(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	subAccountFilter := r.URL.Query().Get("sub_account_id")

	// client_routes больше не содержит country_id напрямую — после миграции на
	// route_condition_groups признак страны переехал туда. Возвращаем NULL для
	// обратной совместимости с фронтом (NetworkRoutingPage отображает '—' при
	// null), не блокируя страницу 500-кой как было раньше.
	query := `SELECT cr.id, cr.client_id, c.name AS sub_account_name,
	                 cr.provider_id, p.name AS provider_name,
	                 NULL::uuid AS country_id, cr.operator_id, cr.priority, cr.active
	          FROM client_routes cr
	          JOIN clients c ON c.id = cr.client_id
	          JOIN providers p ON p.id = cr.provider_id
	          WHERE c.parent_client_id = $1`
	args := []interface{}{resellerID}
	i := 2

	if subAccountFilter != "" {
		query += fmt.Sprintf(" AND cr.client_id = $%d", i)
		args = append(args, subAccountFilter)
		i++
	}
	query += " ORDER BY c.name, cr.priority"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения маршрутов сети")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type routeEntry struct {
		ID             string  `json:"id"`
		ClientID       string  `json:"client_id"`
		SubAccountName string  `json:"sub_account_name"`
		ProviderID     string  `json:"provider_id"`
		ProviderName   string  `json:"provider_name"`
		CountryID      *string `json:"country_id"`
		OperatorID     *string `json:"operator_id"`
		Priority       int     `json:"priority"`
		Active         bool    `json:"active"`
	}
	items := make([]routeEntry, 0)
	for rows.Next() {
		var rt routeEntry
		if err := rows.Scan(&rt.ID, &rt.ClientID, &rt.SubAccountName,
			&rt.ProviderID, &rt.ProviderName, &rt.CountryID, &rt.OperatorID,
			&rt.Priority, &rt.Active); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, rt)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"routes": items, "total": len(items)})
}

// BulkAssignProvider POST /portal/v1/reseller/routing/bulk-assign
func (h *ResellerRoutingHandlers) BulkAssignProvider(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var req struct {
		SubAccountIDs []string `json:"sub_account_ids"`
		ProviderID    string   `json:"provider_id"`
		Priority      int32    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.SubAccountIDs) == 0 || req.ProviderID == "" {
		respondError(w, shared.ErrInvalidInput("sub_account_ids и provider_id обязательны"))
		return
	}

	type resultItem struct {
		SubAccountID string `json:"sub_account_id"`
		Status       string `json:"status"`
		Error        string `json:"error,omitempty"`
	}
	results := make([]resultItem, 0, len(req.SubAccountIDs))

	for _, saID := range req.SubAccountIDs {
		// Verify ownership
		var parentID string
		err := h.pool.QueryRow(r.Context(),
			`SELECT parent_client_id::text FROM clients WHERE id = $1`, saID,
		).Scan(&parentID)
		if err != nil || parentID != resellerID {
			results = append(results, resultItem{SubAccountID: saID, Status: "error", Error: "субаккаунт не найден"})
			continue
		}

		_, err = h.routingClient.AssignProviderToClient(r.Context(), &routingv1.AssignProviderRequest{
			ClientId:       saID,
			ProviderId:     req.ProviderID,
			SharedPriority: req.Priority,
		})
		if err != nil {
			results = append(results, resultItem{SubAccountID: saID, Status: "error", Error: err.Error()})
			continue
		}
		results = append(results, resultItem{SubAccountID: saID, Status: "assigned"})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}

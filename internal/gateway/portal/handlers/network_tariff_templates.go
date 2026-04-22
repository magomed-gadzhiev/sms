// Package handlers: network_tariff_templates.go implements the
// GET /portal/v1/network/tariff-templates endpoint.
//
// Task 3: returns the list of tariff templates owned by the authenticated
// reseller, each annotated with two counters:
//   - plans_count — number of active plans bound to the template
//     (reseller_tariff_plans with template_id = tpl.id AND active).
//   - bound_subaccount_count — number of sub-account assignments
//     (sub_account_template_assignments has no `active` flag; each row counts).
//
// Gate: is_reseller = true on the caller. Non-resellers get 401 (same
// convention as network_tariffs_summary). No Redis cache in Task 3.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NetworkTariffTemplatesHandler serves the list of tariff templates for
// a reseller on the /network/tariffs page.
type NetworkTariffTemplatesHandler struct {
	pool *pgxpool.Pool
}

// NewNetworkTariffTemplatesHandler constructs the handler.
func NewNetworkTariffTemplatesHandler(pool *pgxpool.Pool) *NetworkTariffTemplatesHandler {
	return &NetworkTariffTemplatesHandler{pool: pool}
}

// TariffTemplateSummary is the per-row JSON payload.
type TariffTemplateSummary struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	PlansCount           int64     `json:"plans_count"`
	BoundSubaccountCount int64     `json:"bound_subaccount_count"`
	CreatedAt            time.Time `json:"created_at"`
}

// List handles GET /portal/v1/network/tariff-templates.
//
// Returns a JSON array of TariffTemplateSummary, one entry per active template
// owned by the authenticated reseller. Non-reseller callers get 401.
func (h *NetworkTariffTemplatesHandler) List(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var isReseller bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
	).Scan(&isReseller); err != nil || !isReseller {
		respondError(w, shared.ErrUnauthorized("доступ только для агрегаторов"))
		return
	}

	// Correlated subqueries return the two counters. reseller_tariff_plans
	// is scoped to active rows; sub_account_template_assignments has no
	// active flag (migration 000098, lines 94–102).
	const q = `
		SELECT tpl.id::text,
		       tpl.name,
		       COALESCE(tpl.description, ''),
		       (SELECT COUNT(*) FROM reseller_tariff_plans
		          WHERE template_id = tpl.id AND active),
		       (SELECT COUNT(*) FROM sub_account_template_assignments
		          WHERE template_id = tpl.id),
		       tpl.created_at
		FROM reseller_tariff_templates tpl
		WHERE tpl.reseller_id = $1 AND tpl.active
		ORDER BY tpl.name
	`

	rows, err := h.pool.Query(r.Context(), q, clientID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: query failed")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	items := make([]TariffTemplateSummary, 0)
	for rows.Next() {
		var row TariffTemplateSummary
		if err := rows.Scan(
			&row.ID,
			&row.Name,
			&row.Description,
			&row.PlansCount,
			&row.BoundSubaccountCount,
			&row.CreatedAt,
		); err != nil {
			log.Error().Err(err).Msg("network_tariff_templates: scan failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: rows iter failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		log.Error().Err(err).Msg("network_tariff_templates: encode failed")
	}
}

// Package handlers: network_tariffs_summary.go implements the
// GET /portal/v1/network/tariffs/subaccounts-summary endpoint.
//
// Task 1 of the /network/tariffs redesign (see
// docs/superpowers/specs/2026-04-22-network-tariffs-redesign-design.md).
//
// Returns, for every sub-account owned by the calling reseller, the current
// tariff-template binding (if any), the count of direct override plans, and
// placeholders for the average-price / currency columns. The avg price is
// filled in by a subsequent task and is NULL here.
package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// NetworkTariffsSummaryHandler serves the per-reseller sub-accounts summary
// for the /network/tariffs page.
type NetworkTariffsSummaryHandler struct {
	pool *pgxpool.Pool
}

// NewNetworkTariffsSummaryHandler constructs the handler.
func NewNetworkTariffsSummaryHandler(pool *pgxpool.Pool) *NetworkTariffsSummaryHandler {
	return &NetworkTariffsSummaryHandler{pool: pool}
}

// SubAccountSummary is the per-row JSON payload.
type SubAccountSummary struct {
	SubAccountID    string   `json:"sub_account_id"`
	SubAccountName  string   `json:"sub_account_name"`
	SubAccountEmail string   `json:"sub_account_email"`
	TemplateID      *string  `json:"template_id"`
	TemplateName    *string  `json:"template_name"`
	OverrideCount   int      `json:"override_count"`
	AvgPricePerSMS  *float64 `json:"avg_price_per_sms"`
	Currency        string   `json:"currency"`
}

// List handles GET /portal/v1/network/tariffs/subaccounts-summary.
//
// Returns a JSON array of SubAccountSummary, one entry per sub-account
// belonging to the authenticated reseller. Non-reseller callers get 401.
func (h *NetworkTariffsSummaryHandler) List(w http.ResponseWriter, r *http.Request) {
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

	// Single query:
	//   - LEFT JOIN template assignment + template (exactly 0..1 per sub-account
	//     thanks to the unique index on sub_account_template_assignments.sub_account_id)
	//   - LEFT JOIN LATERAL for override_count (active plans with sub_account_id set)
	//   - Currency is hard-coded to 'RUB' — clients table has no currency column.
	//     The plan calls this out as a fallback to be revisited when multi-currency
	//     support lands on the clients table.
	//   - avg_price_per_sms is left NULL; Task 2 computes it.
	const q = `
		SELECT
			c.id::text                                  AS sub_account_id,
			c.name                                      AS sub_account_name,
			COALESCE(c.email, '')                       AS sub_account_email,
			sta.template_id::text                       AS template_id,
			t.name                                      AS template_name,
			COALESCE(oc.cnt, 0)                         AS override_count
		FROM clients c
		LEFT JOIN sub_account_template_assignments sta ON sta.sub_account_id = c.id
		LEFT JOIN reseller_tariff_templates t
		       ON t.id = sta.template_id AND t.active = true
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS cnt
			FROM reseller_tariff_plans p
			WHERE p.sub_account_id = c.id
			  AND p.active = true
		) oc ON true
		WHERE c.parent_client_id = $1 AND c.active = true
		ORDER BY c.name
	`

	rows, err := h.pool.Query(r.Context(), q, clientID)
	if err != nil {
		log.Error().Err(err).Msg("network_tariffs_summary: query failed")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	items := make([]SubAccountSummary, 0)
	for rows.Next() {
		var row SubAccountSummary
		var templateID, templateName *string
		if err := rows.Scan(
			&row.SubAccountID,
			&row.SubAccountName,
			&row.SubAccountEmail,
			&templateID,
			&templateName,
			&row.OverrideCount,
		); err != nil {
			log.Error().Err(err).Msg("network_tariffs_summary: scan failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		row.TemplateID = templateID
		row.TemplateName = templateName
		row.AvgPricePerSMS = nil // computed by Task 2
		row.Currency = "RUB"     // see note above
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("network_tariffs_summary: rows iter failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}

	respondJSON(w, http.StatusOK, items)
}

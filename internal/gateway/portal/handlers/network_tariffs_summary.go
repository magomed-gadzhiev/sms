// Package handlers: network_tariffs_summary.go implements the
// GET /portal/v1/network/tariffs/subaccounts-summary endpoint.
//
// Task 1: returns per-sub-account template binding + override count.
// Task 2: populates avg_price_per_sms (simple AVG of tier-0 price_per_segment
// across effective plans for RU × 'paid_registered'; one row per operator).
// Task 2 also adds Redis caching with key `tariffs:summary:<reseller_id>` and
// a 5-minute TTL. Cache stores the serialized JSON bytes so the response is
// byte-identical on hit/miss. When Redis is unreachable, we log-and-continue —
// the request still succeeds against the DB. Cache invalidation is a later task.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// summaryCacheTTL — срок жизни записи кэша summary на один reseller.
const summaryCacheTTL = 5 * time.Minute

// summaryCacheKeyPrefix — префикс ключа Redis.
const summaryCacheKeyPrefix = "tariffs:summary:"

// NetworkTariffsSummaryHandler serves the per-reseller sub-accounts summary
// for the /network/tariffs page.
type NetworkTariffsSummaryHandler struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewNetworkTariffsSummaryHandler constructs the handler.
// redisClient may be nil — in that case caching is skipped silently.
func NewNetworkTariffsSummaryHandler(pool *pgxpool.Pool, redisClient *redis.Client) *NetworkTariffsSummaryHandler {
	return &NetworkTariffsSummaryHandler{pool: pool, redis: redisClient}
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

	cacheKey := summaryCacheKeyPrefix + clientID.String()

	// Cache lookup — log-and-continue on any error (including miss).
	if h.redis != nil {
		if cached, err := h.redis.Get(r.Context(), cacheKey).Bytes(); err == nil && len(cached) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if _, werr := w.Write(cached); werr != nil {
				log.Error().Err(werr).Msg("network_tariffs_summary: write cached body failed")
			}
			return
		} else if err != nil && err != redis.Nil {
			log.Warn().Err(err).Msg("network_tariffs_summary: redis GET failed, falling back to DB")
		}
	}

	// Effective plan per sub-account:
	//   * override  — plan row with sub_account_id = c.id (wins when present)
	//   * template  — plan owned by template bound via sub_account_template_assignments
	// For each effective plan we take ONE row per operator (the tier with
	// from_count = 0 in the active period) and AVG the price_per_segment
	// into avg_price_per_sms. Scope: RU × sender_category='paid_registered'.
	// NULL when the sub-account has no effective plan/tier matching.
	const q = `
		WITH ru AS (
			SELECT id FROM countries WHERE iso_code = 'RU' LIMIT 1
		),
		effective_plans AS (
			-- override wins
			SELECT c.id AS sub_id, p.id AS plan_id, p.operator_id
			FROM clients c
			JOIN reseller_tariff_plans p
			  ON p.sub_account_id = c.id
			 AND p.active = true
			 AND p.sender_category = 'paid_registered'
			 AND p.country_id = (SELECT id FROM ru)
			WHERE c.parent_client_id = $1 AND c.active = true
			UNION ALL
			-- template-bound (only if no override exists for this sub-account+operator)
			SELECT c.id AS sub_id, p.id AS plan_id, p.operator_id
			FROM clients c
			JOIN sub_account_template_assignments sta ON sta.sub_account_id = c.id
			JOIN reseller_tariff_plans p
			  ON p.template_id = sta.template_id
			 AND p.active = true
			 AND p.sender_category = 'paid_registered'
			 AND p.country_id = (SELECT id FROM ru)
			WHERE c.parent_client_id = $1
			  AND c.active = true
			  AND NOT EXISTS (
			      SELECT 1 FROM reseller_tariff_plans po
			      WHERE po.sub_account_id = c.id
			        AND po.active = true
			        AND po.sender_category = p.sender_category
			        AND po.country_id = p.country_id
			        AND COALESCE(po.operator_id, '00000000-0000-0000-0000-000000000000'::uuid)
			          = COALESCE(p.operator_id,  '00000000-0000-0000-0000-000000000000'::uuid)
			  )
		),
		tier0 AS (
			SELECT ep.sub_id, ep.operator_id, t.price_per_segment
			FROM effective_plans ep
			JOIN reseller_tariff_periods pr
			  ON pr.tariff_plan_id = ep.plan_id
			 AND pr.start_date <= CURRENT_DATE
			 AND (pr.end_date IS NULL OR pr.end_date > CURRENT_DATE)
			JOIN reseller_tariff_tiers t
			  ON t.tariff_period_id = pr.id
			 AND t.from_count = 0
		),
		avg_prices AS (
			-- one row per (sub, operator) → AVG across operators
			SELECT sub_id, AVG(price_per_segment)::float8 AS avg_price
			FROM (
				SELECT sub_id, operator_id, AVG(price_per_segment)::numeric AS price_per_segment
				FROM tier0
				GROUP BY sub_id, operator_id
			) per_op
			GROUP BY sub_id
		)
		SELECT
			c.id::text                                  AS sub_account_id,
			c.name                                      AS sub_account_name,
			COALESCE(c.email, '')                       AS sub_account_email,
			sta.template_id::text                       AS template_id,
			t.name                                      AS template_name,
			COALESCE(oc.cnt, 0)                         AS override_count,
			ap.avg_price                                AS avg_price_per_sms
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
		LEFT JOIN avg_prices ap ON ap.sub_id = c.id
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
		var avgPrice *float64
		if err := rows.Scan(
			&row.SubAccountID,
			&row.SubAccountName,
			&row.SubAccountEmail,
			&templateID,
			&templateName,
			&row.OverrideCount,
			&avgPrice,
		); err != nil {
			log.Error().Err(err).Msg("network_tariffs_summary: scan failed")
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		row.TemplateID = templateID
		row.TemplateName = templateName
		row.AvgPricePerSMS = avgPrice
		row.Currency = "RUB"
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("network_tariffs_summary: rows iter failed")
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}

	// Serialize once — write to cache and response body as identical bytes.
	body, err := json.Marshal(items)
	if err != nil {
		log.Error().Err(err).Msg("network_tariffs_summary: marshal failed")
		respondError(w, shared.ErrInternalServer("ошибка сериализации"))
		return
	}
	// encoding/json.Marshal omits the trailing newline that json.Encoder adds.
	body = append(body, '\n')

	if h.redis != nil {
		// Best-effort cache write — use a short detached timeout so a slow
		// Redis doesn't delay the user-facing response.
		cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := h.redis.Set(cctx, cacheKey, body, summaryCacheTTL).Err(); err != nil {
			log.Warn().Err(err).Msg("network_tariffs_summary: redis SET failed")
		}
		cancel()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		log.Error().Err(err).Msg("network_tariffs_summary: write body failed")
	}
}

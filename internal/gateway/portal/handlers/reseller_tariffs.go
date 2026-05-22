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

type ResellerTariffHandlers struct {
	pool *pgxpool.Pool
}

func NewResellerTariffHandlers(pool *pgxpool.Pool) *ResellerTariffHandlers {
	return &ResellerTariffHandlers{pool: pool}
}

// checkReseller — см. C.4 cleanup в reseller_dashboard.go.
func (h *ResellerTariffHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return "", false
	}
	return clientID.String(), true
}

// ListTariffs GET /portal/v1/reseller/tariffs?sub_account_id=...
func (h *ResellerTariffHandlers) ListTariffs(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	subAccountID := r.URL.Query().Get("sub_account_id")

	args := []interface{}{resellerID}

	var query string
	if subAccountID != "" {
		// Sub-account specific tariff overrides global (NULL) tariff per operator+category
		query = `SELECT t.id, t.sub_account_id, t.operator_id, t.operator_name,
		                t.sender_category, t.price_per_segment, t.active,
		                t.created_at, t.updated_at
		         FROM (
		           SELECT DISTINCT ON (at.operator_id, at.sender_category)
		                  at.id, at.sub_account_id, at.operator_id, o.name AS operator_name,
		                  at.sender_category, at.price_per_segment::text AS price_per_segment, at.active,
		                  at.created_at, at.updated_at
		           FROM aggregator_tariffs at
		           JOIN operators o ON o.id = at.operator_id
		           WHERE at.aggregator_id = $1 AND at.active = true
		             AND (at.sub_account_id = $2 OR at.sub_account_id IS NULL)
		           ORDER BY at.operator_id, at.sender_category, at.sub_account_id NULLS LAST
		         ) t
		         ORDER BY t.operator_name, t.sender_category`
		args = append(args, subAccountID)
	} else {
		query = `SELECT at.id, at.sub_account_id, at.operator_id, o.name AS operator_name,
		                at.sender_category, at.price_per_segment::text, at.active,
		                at.created_at, at.updated_at
		         FROM aggregator_tariffs at
		         JOIN operators o ON o.id = at.operator_id
		         WHERE at.aggregator_id = $1 AND at.active = true
		           AND at.sub_account_id IS NULL
		         ORDER BY o.name, at.sender_category`
	}

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения тарифов агрегатора")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type tariffJSON struct {
		ID             string    `json:"id"`
		SubAccountID   *string   `json:"sub_account_id"`
		OperatorID     string    `json:"operator_id"`
		OperatorName   string    `json:"operator_name"`
		SenderCategory string    `json:"sender_category"`
		PricePerSMS    string    `json:"price_per_sms"`
		Active         bool      `json:"active"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	}
	items := make([]tariffJSON, 0)
	for rows.Next() {
		var t tariffJSON
		if err := rows.Scan(&t.ID, &t.SubAccountID, &t.OperatorID, &t.OperatorName,
			&t.SenderCategory, &t.PricePerSMS, &t.Active,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, t)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"tariffs": items, "total": len(items)})
}

// UpsertTariffs PUT /portal/v1/reseller/tariffs
func (h *ResellerTariffHandlers) UpsertTariffs(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var req struct {
		SubAccountID *string `json:"sub_account_id"`
		Tariffs      []struct {
			OperatorID     string `json:"operator_id"`
			SenderCategory string `json:"sender_category"`
			PricePerSMS    string `json:"price_per_sms"`
		} `json:"tariffs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.Tariffs) == 0 {
		respondError(w, shared.ErrInvalidInput("tariffs не может быть пустым"))
		return
	}

	// Verify sub-account ownership if specified
	if req.SubAccountID != nil && *req.SubAccountID != "" {
		var parentID string
		err := h.pool.QueryRow(r.Context(),
			`SELECT parent_client_id::text FROM clients WHERE id = $1`, *req.SubAccountID,
		).Scan(&parentID)
		if err != nil || parentID != resellerID {
			respondError(w, shared.ErrNotFound("субаккаунт не найден"))
			return
		}
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка начала транзакции"))
		return
	}
	defer tx.Rollback(r.Context())

	for _, t := range req.Tariffs {
		if t.OperatorID == "" || t.PricePerSMS == "" {
			respondError(w, shared.ErrInvalidInput("operator_id и price_per_sms обязательны"))
			return
		}
		category := t.SenderCategory
		if category == "" {
			category = "standard"
		}

		_, err := tx.Exec(r.Context(),
			`INSERT INTO aggregator_tariffs (aggregator_id, sub_account_id, operator_id, sender_category, price_per_segment)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (aggregator_id, COALESCE(sub_account_id, '00000000-0000-0000-0000-000000000000'::uuid), operator_id, sender_category) WHERE active = true
			 DO UPDATE SET price_per_segment = $5, updated_at = NOW()`,
			resellerID, req.SubAccountID, t.OperatorID, category, t.PricePerSMS,
		)
		if err != nil {
			log.Error().Err(err).Str("operator_id", t.OperatorID).Msg("ошибка upsert тарифа")
			respondError(w, shared.ErrInternalServer("ошибка сохранения тарифа"))
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"updated": len(req.Tariffs)})
}

// CopyTariffs POST /portal/v1/reseller/tariffs/copy
func (h *ResellerTariffHandlers) CopyTariffs(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var req struct {
		FromSubAccountID string `json:"from_sub_account_id"`
		ToSubAccountID   string `json:"to_sub_account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.FromSubAccountID == "" || req.ToSubAccountID == "" {
		respondError(w, shared.ErrInvalidInput("from_sub_account_id и to_sub_account_id обязательны"))
		return
	}

	// Verify ownership of both
	for _, saID := range []string{req.FromSubAccountID, req.ToSubAccountID} {
		var parentID string
		err := h.pool.QueryRow(r.Context(),
			`SELECT parent_client_id::text FROM clients WHERE id = $1`, saID,
		).Scan(&parentID)
		if err != nil || parentID != resellerID {
			respondError(w, shared.ErrNotFound("субаккаунт не найден: "+saID))
			return
		}
	}

	// Deactivate existing tariffs for target
	_, _ = h.pool.Exec(r.Context(),
		`UPDATE aggregator_tariffs SET active = false, updated_at = NOW()
		 WHERE aggregator_id = $1 AND sub_account_id = $2 AND active = true`,
		resellerID, req.ToSubAccountID,
	)

	// Copy from source
	result, err := h.pool.Exec(r.Context(),
		`INSERT INTO aggregator_tariffs (aggregator_id, sub_account_id, operator_id, sender_category, price_per_segment)
		 SELECT aggregator_id, $2, operator_id, sender_category, price_per_segment
		 FROM aggregator_tariffs
		 WHERE aggregator_id = $1 AND sub_account_id = $3 AND active = true`,
		resellerID, req.ToSubAccountID, req.FromSubAccountID,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка копирования тарифов")
		respondError(w, shared.ErrInternalServer("ошибка копирования"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"copied": result.RowsAffected()})
}

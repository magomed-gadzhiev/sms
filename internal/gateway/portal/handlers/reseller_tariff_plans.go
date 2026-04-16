package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// ResellerTariffPlanHandlers handles reseller tariff plans, templates, periods, and tiers.
type ResellerTariffPlanHandlers struct {
	pool *pgxpool.Pool
}

// NewResellerTariffPlanHandlers creates a new handler instance.
func NewResellerTariffPlanHandlers(pool *pgxpool.Pool) *ResellerTariffPlanHandlers {
	return &ResellerTariffPlanHandlers{pool: pool}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (h *ResellerTariffPlanHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
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

func (h *ResellerTariffPlanHandlers) checkSubAccountOwnership(w http.ResponseWriter, r *http.Request, resellerID, subAccountID string) bool {
	var parentID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT parent_client_id::text FROM clients WHERE id = $1`, subAccountID,
	).Scan(&parentID)
	if err != nil || parentID != resellerID {
		respondError(w, shared.ErrNotFound("субаккаунт не найден"))
		return false
	}
	return true
}

func (h *ResellerTariffPlanHandlers) checkTemplateOwnership(w http.ResponseWriter, r *http.Request, resellerID, templateID string) bool {
	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT reseller_id::text FROM reseller_tariff_templates WHERE id = $1 AND active = true`, templateID,
	).Scan(&ownerID)
	if err != nil || ownerID != resellerID {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return false
	}
	return true
}

func (h *ResellerTariffPlanHandlers) checkPlanOwnership(w http.ResponseWriter, r *http.Request, resellerID, planID string) bool {
	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT reseller_id::text FROM reseller_tariff_plans WHERE id = $1 AND active = true`, planID,
	).Scan(&ownerID)
	if err != nil || ownerID != resellerID {
		respondError(w, shared.ErrNotFound("тарифный план не найден"))
		return false
	}
	return true
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return false
}

func isExclusionViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
		return true
	}
	return false
}

// effectivePrice represents a resolved tariff price with its source.
type effectivePrice struct {
	CountryID      *string `json:"country_id"`
	CountryName    *string `json:"country_name"`
	OperatorID     *string `json:"operator_id"`
	OperatorName   *string `json:"operator_name"`
	SenderCategory string  `json:"sender_category"`
	TrafficType    string  `json:"traffic_type"`
	Price          string  `json:"price"`
	Source         string  `json:"source"`
}

// ---------------------------------------------------------------------------
// Templates CRUD
// ---------------------------------------------------------------------------

// ListTemplates GET /portal/v1/reseller/tariff-templates
func (h *ResellerTariffPlanHandlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT t.id, t.name, t.description, t.created_at, t.updated_at,
		        COALESCE(a.cnt, 0) AS assignment_count
		 FROM reseller_tariff_templates t
		 LEFT JOIN (
		     SELECT template_id, COUNT(*) AS cnt
		     FROM sub_account_template_assignments
		     GROUP BY template_id
		 ) a ON a.template_id = t.id
		 WHERE t.reseller_id = $1 AND t.active = true
		 ORDER BY t.name`, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения шаблонов тарифов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type templateJSON struct {
		ID              string    `json:"id"`
		Name            string    `json:"name"`
		Description     *string   `json:"description"`
		CreatedAt       time.Time `json:"created_at"`
		UpdatedAt       time.Time `json:"updated_at"`
		AssignmentCount int       `json:"assignment_count"`
	}
	items := make([]templateJSON, 0)
	for rows.Next() {
		var t templateJSON
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt, &t.AssignmentCount); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, t)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"templates": items, "total": len(items)})
}

// CreateTemplate POST /portal/v1/reseller/tariff-templates
func (h *ResellerTariffPlanHandlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("name обязательно"))
		return
	}

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO reseller_tariff_templates (reseller_id, name, description)
		 VALUES ($1, $2, $3) RETURNING id`, resellerID, req.Name, req.Description,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			respondError(w, shared.ErrConflict("шаблон с таким именем уже существует"))
			return
		}
		log.Error().Err(err).Msg("ошибка создания шаблона тарифов")
		respondError(w, shared.ErrInternalServer("ошибка создания"))
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

// UpdateTemplate PUT /portal/v1/reseller/tariff-templates/{id}
func (h *ResellerTariffPlanHandlers) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	templateID := mux.Vars(r)["id"]
	if !h.checkTemplateOwnership(w, r, resellerID, templateID) {
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`UPDATE reseller_tariff_templates
		 SET name = COALESCE($1, name), description = COALESCE($2, description), updated_at = NOW()
		 WHERE id = $3`, req.Name, req.Description, templateID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			respondError(w, shared.ErrConflict("шаблон с таким именем уже существует"))
			return
		}
		log.Error().Err(err).Msg("ошибка обновления шаблона")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// DeleteTemplate DELETE /portal/v1/reseller/tariff-templates/{id}
func (h *ResellerTariffPlanHandlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	templateID := mux.Vars(r)["id"]
	if !h.checkTemplateOwnership(w, r, resellerID, templateID) {
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка начала транзакции"))
		return
	}
	defer tx.Rollback(r.Context())

	// Remove template assignments
	_, _ = tx.Exec(r.Context(),
		`DELETE FROM sub_account_template_assignments WHERE template_id = $1`, templateID)

	// Deactivate plans belonging to this template
	_, _ = tx.Exec(r.Context(),
		`UPDATE reseller_tariff_plans SET active = false, updated_at = NOW()
		 WHERE template_id = $1 AND active = true`, templateID)

	// Soft-delete the template
	_, err = tx.Exec(r.Context(),
		`UPDATE reseller_tariff_templates SET active = false, updated_at = NOW()
		 WHERE id = $1`, templateID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка удаления шаблона")
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// AssignTemplate POST /portal/v1/reseller/tariff-templates/{id}/assign
func (h *ResellerTariffPlanHandlers) AssignTemplate(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	templateID := mux.Vars(r)["id"]
	if !h.checkTemplateOwnership(w, r, resellerID, templateID) {
		return
	}

	var req struct {
		SubAccountID string `json:"sub_account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SubAccountID == "" {
		respondError(w, shared.ErrInvalidInput("sub_account_id обязательно"))
		return
	}
	if !h.checkSubAccountOwnership(w, r, resellerID, req.SubAccountID) {
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
		 VALUES ($1, $2)
		 ON CONFLICT (sub_account_id) DO UPDATE SET template_id = $2, assigned_at = NOW()`,
		req.SubAccountID, templateID,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка назначения шаблона")
		respondError(w, shared.ErrInternalServer("ошибка назначения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// UnassignTemplate POST /portal/v1/reseller/tariff-templates/{id}/unassign
func (h *ResellerTariffPlanHandlers) UnassignTemplate(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	templateID := mux.Vars(r)["id"]
	if !h.checkTemplateOwnership(w, r, resellerID, templateID) {
		return
	}

	var req struct {
		SubAccountID string `json:"sub_account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SubAccountID == "" {
		respondError(w, shared.ErrInvalidInput("sub_account_id обязательно"))
		return
	}
	if !h.checkSubAccountOwnership(w, r, resellerID, req.SubAccountID) {
		return
	}

	ct, err := h.pool.Exec(r.Context(),
		`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1 AND template_id = $2`,
		req.SubAccountID, templateID,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка отвязки шаблона")
		respondError(w, shared.ErrInternalServer("ошибка отвязки"))
		return
	}
	if ct.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("назначение"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// ---------------------------------------------------------------------------
// Plans CRUD
// ---------------------------------------------------------------------------

// ListPlans GET /portal/v1/reseller/tariff-plans?template_id=...&sub_account_id=...
func (h *ResellerTariffPlanHandlers) ListPlans(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	templateID := r.URL.Query().Get("template_id")
	subAccountID := r.URL.Query().Get("sub_account_id")

	args := []interface{}{resellerID}
	where := `p.reseller_id = $1 AND p.active = true`

	if templateID != "" {
		args = append(args, templateID)
		where += fmt.Sprintf(` AND p.template_id = $%d`, len(args))
	}
	if subAccountID != "" {
		args = append(args, subAccountID)
		where += fmt.Sprintf(` AND p.sub_account_id = $%d`, len(args))
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.template_id, p.sub_account_id,
		       p.country_id, p.operator_id, p.sender_category, p.traffic_type,
		       p.strategy, p.created_at, p.updated_at,
		       o.name AS operator_name, c.name AS country_name
		FROM reseller_tariff_plans p
		LEFT JOIN operators o ON o.id = p.operator_id
		LEFT JOIN countries c ON c.id = p.country_id
		WHERE %s
		ORDER BY COALESCE(c.name, ''), COALESCE(o.name, ''), p.sender_category`, where)

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения тарифных планов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type planJSON struct {
		ID             string    `json:"id"`
		TemplateID     *string   `json:"template_id"`
		SubAccountID   *string   `json:"sub_account_id"`
		CountryID      *string   `json:"country_id"`
		OperatorID     *string   `json:"operator_id"`
		SenderCategory string    `json:"sender_category"`
		TrafficType    string    `json:"traffic_type"`
		Strategy       string    `json:"strategy"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
		OperatorName   *string   `json:"operator_name"`
		CountryName    *string   `json:"country_name"`
	}
	items := make([]planJSON, 0)
	for rows.Next() {
		var p planJSON
		if err := rows.Scan(
			&p.ID, &p.TemplateID, &p.SubAccountID,
			&p.CountryID, &p.OperatorID, &p.SenderCategory, &p.TrafficType,
			&p.Strategy, &p.CreatedAt, &p.UpdatedAt,
			&p.OperatorName, &p.CountryName,
		); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, p)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"plans": items, "total": len(items)})
}

var validStrategies = map[string]bool{
	"fixed":              true,
	"threshold":          true,
	"threshold_recalc":   true,
	"prepaid_threshold":  true,
}

// CreatePlan POST /portal/v1/reseller/tariff-plans
func (h *ResellerTariffPlanHandlers) CreatePlan(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var req struct {
		TemplateID     *string `json:"template_id"`
		SubAccountID   *string `json:"sub_account_id"`
		CountryID      *string `json:"country_id"`
		OperatorID     *string `json:"operator_id"`
		SenderCategory string  `json:"sender_category"`
		TrafficType    string  `json:"traffic_type"`
		Strategy       string  `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Normalize empty strings to nil
	if req.TemplateID != nil && *req.TemplateID == "" {
		req.TemplateID = nil
	}
	if req.SubAccountID != nil && *req.SubAccountID == "" {
		req.SubAccountID = nil
	}
	if req.CountryID != nil && *req.CountryID == "" {
		req.CountryID = nil
	}
	if req.OperatorID != nil && *req.OperatorID == "" {
		req.OperatorID = nil
	}

	// Exactly one of template_id or sub_account_id
	hasTemplate := req.TemplateID != nil
	hasSubAccount := req.SubAccountID != nil
	if hasTemplate == hasSubAccount {
		respondError(w, shared.ErrInvalidInput("укажите ровно одно из template_id или sub_account_id"))
		return
	}
	if !validStrategies[req.Strategy] {
		respondError(w, shared.ErrInvalidInput("недопустимая стратегия: "+req.Strategy))
		return
	}
	if req.SenderCategory == "" {
		req.SenderCategory = "standard"
	}
	if req.TrafficType == "" {
		req.TrafficType = "any"
	}

	// Verify ownership
	if hasTemplate {
		if !h.checkTemplateOwnership(w, r, resellerID, *req.TemplateID) {
			return
		}
	}
	if hasSubAccount {
		if !h.checkSubAccountOwnership(w, r, resellerID, *req.SubAccountID) {
			return
		}
	}

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO reseller_tariff_plans
		 (reseller_id, template_id, sub_account_id, country_id, operator_id, sender_category, traffic_type, strategy)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		resellerID, req.TemplateID, req.SubAccountID, req.CountryID, req.OperatorID,
		req.SenderCategory, req.TrafficType, req.Strategy,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			respondError(w, shared.ErrConflict("план с такими параметрами уже существует"))
			return
		}
		log.Error().Err(err).Msg("ошибка создания тарифного плана")
		respondError(w, shared.ErrInternalServer("ошибка создания"))
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

// UpdatePlan PUT /portal/v1/reseller/tariff-plans/{id}
func (h *ResellerTariffPlanHandlers) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	planID := mux.Vars(r)["id"]
	if !h.checkPlanOwnership(w, r, resellerID, planID) {
		return
	}

	var req struct {
		Strategy *string `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Strategy != nil && !validStrategies[*req.Strategy] {
		respondError(w, shared.ErrInvalidInput("недопустимая стратегия: "+*req.Strategy))
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`UPDATE reseller_tariff_plans SET strategy = COALESCE($1, strategy), updated_at = NOW()
		 WHERE id = $2`, req.Strategy, planID,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка обновления плана")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// DeletePlan DELETE /portal/v1/reseller/tariff-plans/{id}
func (h *ResellerTariffPlanHandlers) DeletePlan(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	planID := mux.Vars(r)["id"]
	if !h.checkPlanOwnership(w, r, resellerID, planID) {
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`UPDATE reseller_tariff_plans SET active = false, updated_at = NOW() WHERE id = $1`, planID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка удаления плана")
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// ---------------------------------------------------------------------------
// Periods & Tiers
// ---------------------------------------------------------------------------

// ListPeriods GET /portal/v1/reseller/tariff-plans/{id}/periods
func (h *ResellerTariffPlanHandlers) ListPeriods(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	planID := mux.Vars(r)["id"]
	if !h.checkPlanOwnership(w, r, resellerID, planID) {
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, start_date, end_date, created_at FROM reseller_tariff_periods
		 WHERE tariff_plan_id = $1 ORDER BY start_date`, planID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка получения периодов"))
		return
	}
	defer rows.Close()

	type periodJSON struct {
		ID        string    `json:"id"`
		StartDate string    `json:"start_date"`
		EndDate   string    `json:"end_date"`
		CreatedAt time.Time `json:"created_at"`
	}
	items := make([]periodJSON, 0)
	for rows.Next() {
		var p periodJSON
		var startDate, endDate time.Time
		if err := rows.Scan(&p.ID, &startDate, &endDate, &p.CreatedAt); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		p.StartDate = startDate.Format("2006-01-02")
		p.EndDate = endDate.Format("2006-01-02")
		items = append(items, p)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"periods": items, "total": len(items)})
}

// CreatePeriod POST /portal/v1/reseller/tariff-plans/{id}/periods
func (h *ResellerTariffPlanHandlers) CreatePeriod(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	planID := mux.Vars(r)["id"]
	if !h.checkPlanOwnership(w, r, resellerID, planID) {
		return
	}

	var req struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.StartDate == "" || req.EndDate == "" {
		respondError(w, shared.ErrInvalidInput("start_date и end_date обязательны"))
		return
	}
	startDate, err1 := time.Parse("2006-01-02", req.StartDate)
	endDate, err2 := time.Parse("2006-01-02", req.EndDate)
	if err1 != nil || err2 != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат даты (YYYY-MM-DD)"))
		return
	}
	if !endDate.After(startDate) {
		respondError(w, shared.ErrInvalidInput("end_date должен быть больше start_date"))
		return
	}

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		 VALUES ($1, $2, $3) RETURNING id`, planID, startDate, endDate,
	).Scan(&id)
	if err != nil {
		if isExclusionViolation(err) {
			respondError(w, shared.ErrConflict("период пересекается с существующим"))
			return
		}
		log.Error().Err(err).Msg("ошибка создания периода")
		respondError(w, shared.ErrInternalServer("ошибка создания"))
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

// UpdatePeriod PUT /portal/v1/reseller/tariff-plans/{planId}/periods/{periodId}
func (h *ResellerTariffPlanHandlers) UpdatePeriod(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	planID := vars["id"]
	periodID := vars["periodId"]
	if !h.checkPlanOwnership(w, r, resellerID, planID) {
		return
	}

	var req struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.StartDate == "" || req.EndDate == "" {
		respondError(w, shared.ErrInvalidInput("start_date и end_date обязательны"))
		return
	}
	startDate, err1 := time.Parse("2006-01-02", req.StartDate)
	endDate, err2 := time.Parse("2006-01-02", req.EndDate)
	if err1 != nil || err2 != nil {
		respondError(w, shared.ErrInvalidInput("неверный формат даты (YYYY-MM-DD)"))
		return
	}
	if !endDate.After(startDate) {
		respondError(w, shared.ErrInvalidInput("end_date должен быть больше start_date"))
		return
	}

	ct, err := h.pool.Exec(r.Context(),
		`UPDATE reseller_tariff_periods SET start_date = $1, end_date = $2
		 WHERE id = $3 AND tariff_plan_id = $4`, startDate, endDate, periodID, planID,
	)
	if err != nil {
		if isExclusionViolation(err) {
			respondError(w, shared.ErrConflict("период пересекается с существующим"))
			return
		}
		log.Error().Err(err).Msg("ошибка обновления периода")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	if ct.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("период"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// DeletePeriod DELETE /portal/v1/reseller/tariff-plans/{planId}/periods/{periodId}
func (h *ResellerTariffPlanHandlers) DeletePeriod(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	planID := vars["id"]
	periodID := vars["periodId"]
	if !h.checkPlanOwnership(w, r, resellerID, planID) {
		return
	}

	ct, err := h.pool.Exec(r.Context(),
		`DELETE FROM reseller_tariff_periods WHERE id = $1 AND tariff_plan_id = $2`, periodID, planID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка удаления периода")
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}
	if ct.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("период"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// ListTiers GET /portal/v1/reseller/tariff-periods/{periodId}/tiers
func (h *ResellerTariffPlanHandlers) ListTiers(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	periodID := mux.Vars(r)["periodId"]

	// Verify period ownership via plan
	var planResellerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT p.reseller_id::text FROM reseller_tariff_plans p
		 JOIN reseller_tariff_periods per ON per.tariff_plan_id = p.id
		 WHERE per.id = $1 AND p.active = true`, periodID,
	).Scan(&planResellerID)
	if err != nil || planResellerID != resellerID {
		respondError(w, shared.ErrNotFound("период"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, from_count, price_per_segment::text FROM reseller_tariff_tiers
		 WHERE tariff_period_id = $1 ORDER BY from_count`, periodID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка получения тиров"))
		return
	}
	defer rows.Close()

	type tierJSON struct {
		ID              string `json:"id"`
		FromCount       int    `json:"from_count"`
		PricePerSegment string `json:"price_per_segment"`
	}
	items := make([]tierJSON, 0)
	for rows.Next() {
		var t tierJSON
		if err := rows.Scan(&t.ID, &t.FromCount, &t.PricePerSegment); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, t)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"tiers": items, "total": len(items)})
}

// UpsertTiers PUT /portal/v1/reseller/tariff-periods/{periodId}/tiers
func (h *ResellerTariffPlanHandlers) UpsertTiers(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	periodID := mux.Vars(r)["periodId"]

	// Verify ownership and get strategy
	var planResellerID, strategy string
	err := h.pool.QueryRow(r.Context(),
		`SELECT p.reseller_id::text, p.strategy
		 FROM reseller_tariff_plans p
		 JOIN reseller_tariff_periods per ON per.tariff_plan_id = p.id
		 WHERE per.id = $1 AND p.active = true`, periodID,
	).Scan(&planResellerID, &strategy)
	if err != nil || planResellerID != resellerID {
		respondError(w, shared.ErrNotFound("период"))
		return
	}

	var req struct {
		Tiers []struct {
			FromCount       int    `json:"from_count"`
			PricePerSegment string `json:"price_per_segment"`
		} `json:"tiers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.Tiers) == 0 {
		respondError(w, shared.ErrInvalidInput("tiers не может быть пустым"))
		return
	}

	// Sort by from_count
	sort.Slice(req.Tiers, func(i, j int) bool {
		return req.Tiers[i].FromCount < req.Tiers[j].FromCount
	})

	// Validate base tier (from_count = 0 must be present)
	if req.Tiers[0].FromCount != 0 {
		respondError(w, shared.ErrInvalidInput("необходим базовый тир с from_count = 0"))
		return
	}

	// Validate fixed strategy: exactly 1 tier
	if strategy == "fixed" && len(req.Tiers) != 1 {
		respondError(w, shared.ErrInvalidInput("стратегия fixed допускает ровно 1 тир"))
		return
	}

	// Validate monotonically increasing from_count
	for i := 1; i < len(req.Tiers); i++ {
		if req.Tiers[i].FromCount <= req.Tiers[i-1].FromCount {
			respondError(w, shared.ErrInvalidInput("from_count должен быть строго возрастающим"))
			return
		}
	}

	// Validate prices
	for _, t := range req.Tiers {
		if t.PricePerSegment == "" {
			respondError(w, shared.ErrInvalidInput("price_per_segment обязательно"))
			return
		}
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка начала транзакции"))
		return
	}
	defer tx.Rollback(r.Context())

	// Delete existing tiers
	_, err = tx.Exec(r.Context(),
		`DELETE FROM reseller_tariff_tiers WHERE tariff_period_id = $1`, periodID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка очистки тиров"))
		return
	}

	// Insert new tiers
	for _, t := range req.Tiers {
		_, err = tx.Exec(r.Context(),
			`INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
			 VALUES ($1, $2, $3)`, periodID, t.FromCount, t.PricePerSegment)
		if err != nil {
			log.Error().Err(err).Msg("ошибка вставки тира")
			respondError(w, shared.ErrInternalServer("ошибка сохранения тиров"))
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "count": len(req.Tiers)})
}

// ---------------------------------------------------------------------------
// Overview & Copy
// ---------------------------------------------------------------------------

// TariffOverview GET /portal/v1/reseller/tariff-overview/{sub_account_id}
func (h *ResellerTariffPlanHandlers) TariffOverview(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	subAccountID := r.URL.Query().Get("sub_account_id")
	if subAccountID == "" {
		respondError(w, shared.ErrInvalidInput("sub_account_id обязателен"))
		return
	}
	if !h.checkSubAccountOwnership(w, r, resellerID, subAccountID) {
		return
	}

	// key = country_id|operator_id|category|traffic_type
	effectiveMap := make(map[string]effectivePrice)

	// 1. Legacy fallback (aggregator_tariffs) — lowest priority
	legacyRows, err := h.pool.Query(r.Context(),
		`SELECT at.operator_id::text, o.name, at.sender_category, at.price_per_segment::text
		 FROM aggregator_tariffs at
		 JOIN operators o ON o.id = at.operator_id
		 WHERE at.aggregator_id = $1 AND at.active = true
		   AND (at.sub_account_id = $2 OR at.sub_account_id IS NULL)
		 ORDER BY at.operator_id, at.sender_category, at.sub_account_id NULLS LAST`,
		resellerID, subAccountID)
	if err == nil {
		defer legacyRows.Close()
		seen := make(map[string]bool)
		for legacyRows.Next() {
			var opID, opName, category, price string
			if err := legacyRows.Scan(&opID, &opName, &category, &price); err != nil {
				continue
			}
			key := "|" + opID + "|" + category + "|any"
			if !seen[key] {
				seen[key] = true
				effectiveMap[key] = effectivePrice{
					OperatorID:     &opID,
					OperatorName:   &opName,
					SenderCategory: category,
					TrafficType:    "any",
					Price:          price,
					Source:         "legacy",
				}
			}
		}
	}

	// 2. Template plans — medium priority
	h.overlayPlans(r, effectiveMap, subAccountID, "template", "template")

	// 3. Override plans (direct sub-account) — highest priority
	h.overlayPlans(r, effectiveMap, subAccountID, "sub_account_id", "override")

	items := make([]effectivePrice, 0, len(effectiveMap))
	for _, v := range effectiveMap {
		items = append(items, v)
	}
	// Sort for stable output
	sort.Slice(items, func(i, j int) bool {
		ci := ""
		if items[i].CountryName != nil {
			ci = *items[i].CountryName
		}
		cj := ""
		if items[j].CountryName != nil {
			cj = *items[j].CountryName
		}
		if ci != cj {
			return ci < cj
		}
		oi := ""
		if items[i].OperatorName != nil {
			oi = *items[i].OperatorName
		}
		oj := ""
		if items[j].OperatorName != nil {
			oj = *items[j].OperatorName
		}
		if oi != oj {
			return oi < oj
		}
		return items[i].SenderCategory < items[j].SenderCategory
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{"prices": items, "total": len(items)})
}

// overlayPlans queries plans+periods+tiers for today's date and overlays base tier prices.
func (h *ResellerTariffPlanHandlers) overlayPlans(
	r *http.Request,
	effectiveMap map[string]effectivePrice,
	ownerID string,
	ownerColumn string,
	source string,
) {
	var query string
	var args []interface{}
	today := time.Now().Format("2006-01-02")

	if ownerColumn == "template" {
		// Query via template assignment
		query = `
			SELECT p.country_id::text, c.name, p.operator_id::text, o.name,
			       p.sender_category, p.traffic_type,
			       t.price_per_segment::text
			FROM sub_account_template_assignments sta
			JOIN reseller_tariff_plans p ON p.template_id = sta.template_id AND p.active = true
			JOIN reseller_tariff_periods per ON per.tariff_plan_id = p.id
			  AND per.start_date <= $2 AND per.end_date >= $2
			JOIN reseller_tariff_tiers t ON t.tariff_period_id = per.id AND t.from_count = 0
			LEFT JOIN operators o ON o.id = p.operator_id
			LEFT JOIN countries c ON c.id = p.country_id
			WHERE sta.sub_account_id = $1`
		args = []interface{}{ownerID, today}
	} else {
		query = `
			SELECT p.country_id::text, c.name, p.operator_id::text, o.name,
			       p.sender_category, p.traffic_type,
			       t.price_per_segment::text
			FROM reseller_tariff_plans p
			JOIN reseller_tariff_periods per ON per.tariff_plan_id = p.id
			  AND per.start_date <= $2 AND per.end_date >= $2
			JOIN reseller_tariff_tiers t ON t.tariff_period_id = per.id AND t.from_count = 0
			LEFT JOIN operators o ON o.id = p.operator_id
			LEFT JOIN countries c ON c.id = p.country_id
			WHERE p.sub_account_id = $1 AND p.active = true`
		args = []interface{}{ownerID, today}
	}

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var countryID, countryName, operatorID, operatorName *string
		var category, trafficType, price string
		if err := rows.Scan(&countryID, &countryName, &operatorID, &operatorName,
			&category, &trafficType, &price); err != nil {
			continue
		}
		cid := ""
		if countryID != nil {
			cid = *countryID
		}
		oid := ""
		if operatorID != nil {
			oid = *operatorID
		}
		key := cid + "|" + oid + "|" + category + "|" + trafficType
		effectiveMap[key] = effectivePrice{
			CountryID:      countryID,
			CountryName:    countryName,
			OperatorID:     operatorID,
			OperatorName:   operatorName,
			SenderCategory: category,
			TrafficType:    trafficType,
			Price:          price,
			Source:         source,
		}
	}
}

// CopyPlans POST /portal/v1/reseller/tariff-plans/copy
func (h *ResellerTariffPlanHandlers) CopyPlans(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var req struct {
		SourceTemplateID   *string `json:"source_template_id"`
		SourceSubAccountID *string `json:"source_sub_account_id"`
		TargetTemplateID   *string `json:"target_template_id"`
		TargetSubAccountID *string `json:"target_sub_account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Normalize
	if req.SourceTemplateID != nil && *req.SourceTemplateID == "" {
		req.SourceTemplateID = nil
	}
	if req.SourceSubAccountID != nil && *req.SourceSubAccountID == "" {
		req.SourceSubAccountID = nil
	}
	if req.TargetTemplateID != nil && *req.TargetTemplateID == "" {
		req.TargetTemplateID = nil
	}
	if req.TargetSubAccountID != nil && *req.TargetSubAccountID == "" {
		req.TargetSubAccountID = nil
	}

	// Exactly one source
	hasSourceTpl := req.SourceTemplateID != nil
	hasSourceSA := req.SourceSubAccountID != nil
	if hasSourceTpl == hasSourceSA {
		respondError(w, shared.ErrInvalidInput("укажите ровно один источник: source_template_id или source_sub_account_id"))
		return
	}
	// Exactly one target
	hasTargetTpl := req.TargetTemplateID != nil
	hasTargetSA := req.TargetSubAccountID != nil
	if hasTargetTpl == hasTargetSA {
		respondError(w, shared.ErrInvalidInput("укажите ровно одну цель: target_template_id или target_sub_account_id"))
		return
	}

	// Verify ownership
	if hasSourceTpl {
		if !h.checkTemplateOwnership(w, r, resellerID, *req.SourceTemplateID) {
			return
		}
	}
	if hasSourceSA {
		if !h.checkSubAccountOwnership(w, r, resellerID, *req.SourceSubAccountID) {
			return
		}
	}
	if hasTargetTpl {
		if !h.checkTemplateOwnership(w, r, resellerID, *req.TargetTemplateID) {
			return
		}
	}
	if hasTargetSA {
		if !h.checkSubAccountOwnership(w, r, resellerID, *req.TargetSubAccountID) {
			return
		}
	}

	// Build source filter
	var sourceColumn string
	var sourceID string
	if hasSourceTpl {
		sourceColumn = "template_id"
		sourceID = *req.SourceTemplateID
	} else {
		sourceColumn = "sub_account_id"
		sourceID = *req.SourceSubAccountID
	}
	var targetColumn string
	var targetID string
	if hasTargetTpl {
		targetColumn = "template_id"
		targetID = *req.TargetTemplateID
	} else {
		targetColumn = "sub_account_id"
		targetID = *req.TargetSubAccountID
	}

	// Fetch source plans
	srcRows, err := h.pool.Query(r.Context(),
		fmt.Sprintf(`SELECT id, country_id, operator_id, sender_category, traffic_type, strategy
		 FROM reseller_tariff_plans
		 WHERE %s = $1 AND reseller_id = $2 AND active = true`, sourceColumn),
		sourceID, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка чтения планов источника")
		respondError(w, shared.ErrInternalServer("ошибка чтения"))
		return
	}
	defer srcRows.Close()

	type srcPlan struct {
		ID             string
		CountryID      *string
		OperatorID     *string
		SenderCategory string
		TrafficType    string
		Strategy       string
	}
	var plans []srcPlan
	for srcRows.Next() {
		var p srcPlan
		if err := srcRows.Scan(&p.ID, &p.CountryID, &p.OperatorID, &p.SenderCategory, &p.TrafficType, &p.Strategy); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения"))
			return
		}
		plans = append(plans, p)
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка начала транзакции"))
		return
	}
	defer tx.Rollback(r.Context())

	var copied int
	for _, sp := range plans {
		// Build target insert: set the right FK column, null out the other
		var tplID, saID *string
		if targetColumn == "template_id" {
			tplID = &targetID
		} else {
			saID = &targetID
		}

		var newPlanID string
		err := tx.QueryRow(r.Context(),
			`INSERT INTO reseller_tariff_plans
			 (reseller_id, template_id, sub_account_id, country_id, operator_id, sender_category, traffic_type, strategy)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT DO NOTHING
			 RETURNING id`,
			resellerID, tplID, saID, sp.CountryID, sp.OperatorID, sp.SenderCategory, sp.TrafficType, sp.Strategy,
		).Scan(&newPlanID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Conflict — skip
				continue
			}
			log.Error().Err(err).Msg("ошибка копирования плана")
			respondError(w, shared.ErrInternalServer("ошибка копирования"))
			return
		}

		// Copy periods and tiers for this plan
		periodRows, err := tx.Query(r.Context(),
			`SELECT id, start_date, end_date FROM reseller_tariff_periods WHERE tariff_plan_id = $1`, sp.ID)
		if err != nil {
			log.Error().Err(err).Msg("ошибка чтения периодов")
			respondError(w, shared.ErrInternalServer("ошибка копирования"))
			return
		}
		for periodRows.Next() {
			var oldPeriodID string
			var startDate, endDate time.Time
			if err := periodRows.Scan(&oldPeriodID, &startDate, &endDate); err != nil {
				periodRows.Close()
				respondError(w, shared.ErrInternalServer("ошибка чтения"))
				return
			}
			var newPeriodID string
			err := tx.QueryRow(r.Context(),
				`INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
				 VALUES ($1, $2, $3) RETURNING id`, newPlanID, startDate, endDate,
			).Scan(&newPeriodID)
			if err != nil {
				periodRows.Close()
				if isExclusionViolation(err) {
					continue
				}
				log.Error().Err(err).Msg("ошибка копирования периода")
				respondError(w, shared.ErrInternalServer("ошибка копирования"))
				return
			}

			// Copy tiers
			_, err = tx.Exec(r.Context(),
				`INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
				 SELECT $1, from_count, price_per_segment
				 FROM reseller_tariff_tiers WHERE tariff_period_id = $2`,
				newPeriodID, oldPeriodID)
			if err != nil {
				periodRows.Close()
				log.Error().Err(err).Msg("ошибка копирования тиров")
				respondError(w, shared.ErrInternalServer("ошибка копирования"))
				return
			}
		}
		periodRows.Close()
		copied++
	}

	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"copied": copied})
}

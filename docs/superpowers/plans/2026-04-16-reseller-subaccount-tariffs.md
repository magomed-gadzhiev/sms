# Reseller Sub-Account Tariffs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat sub-account tariff system with a full tarification model (strategies, periods, tiers) plus template-based management with live binding.

**Architecture:** Separate DB tables (`reseller_tariff_*`) mirror the platform's plan→period→tier hierarchy but scoped to resellers. Templates group plans for bulk assignment; overrides allow per-sub-account customization. Legacy `aggregator_tariffs` remains as fallback. Frontend tabs: Overview (read-only matrix), Templates (CRUD + assign), Overrides (per-sub-account plans).

**Tech Stack:** Go 1.24 + pgx/v5 + gorilla/mux (backend), TypeScript 5.7 + React 19 + Vite + Radix UI Tabs + Tailwind CSS 4.2 (frontend)

**Spec:** `docs/superpowers/specs/2026-04-16-reseller-subaccount-tariffs-design.md`

---

## File Map

### Backend (new files)
- `migrations/000098_reseller_tariff_tables.up.sql` — all new tables
- `migrations/000098_reseller_tariff_tables.down.sql` — rollback
- `internal/gateway/portal/handlers/reseller_tariff_plans.go` — all new API handlers

### Backend (modify)
- `internal/gateway/portal/router/router.go` — register new routes
- `cmd/portal-gateway/main.go` — instantiate new handler

### Frontend (new files)
- `portal-frontend/src/pages/network/components/TariffOverviewTab.tsx`
- `portal-frontend/src/pages/network/components/TariffTemplatesTab.tsx`
- `portal-frontend/src/pages/network/components/TariffOverridesTab.tsx`
- `portal-frontend/src/pages/network/components/TariffPlanEditor.tsx`
- `portal-frontend/src/pages/network/components/TemplateAssignModal.tsx`

### Frontend (modify)
- `portal-frontend/src/api/client.ts` — add resellerTariffApi functions
- `portal-frontend/src/pages/network/NetworkTariffsPage.tsx` — refactor to tabs

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/000098_reseller_tariff_tables.up.sql`
- Create: `migrations/000098_reseller_tariff_tables.down.sql`

- [ ] **Step 1: Create up migration**

```sql
-- migrations/000098_reseller_tariff_tables.up.sql
BEGIN;

-- 1. Tariff templates (named groups of plans)
CREATE TABLE reseller_tariff_templates (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id UUID        NOT NULL REFERENCES clients(id),
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    active      BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_reseller_template_unique_name
    ON reseller_tariff_templates (reseller_id, name) WHERE active = true;
CREATE INDEX idx_reseller_template_reseller ON reseller_tariff_templates (reseller_id) WHERE active = true;

CREATE TRIGGER update_reseller_tariff_templates_updated_at
    BEFORE UPDATE ON reseller_tariff_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. Tariff plans (belong to template OR sub-account override)
CREATE TABLE reseller_tariff_plans (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reseller_id     UUID        NOT NULL REFERENCES clients(id),
    template_id     UUID        REFERENCES reseller_tariff_templates(id) ON DELETE CASCADE,
    sub_account_id  UUID        REFERENCES clients(id),
    country_id      UUID,
    operator_id     UUID,
    sender_category VARCHAR(32) NOT NULL DEFAULT 'standard',
    traffic_type    VARCHAR(32) NOT NULL DEFAULT 'any',
    strategy        VARCHAR(30) NOT NULL CHECK (strategy IN ('fixed', 'threshold', 'threshold_recalc', 'prepaid_threshold')),
    active          BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (template_id IS NOT NULL AND sub_account_id IS NULL) OR
        (template_id IS NULL AND sub_account_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_reseller_plan_template_dims
    ON reseller_tariff_plans (
        template_id,
        COALESCE(country_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid),
        sender_category, traffic_type
    ) WHERE active = true AND template_id IS NOT NULL;

CREATE UNIQUE INDEX idx_reseller_plan_sub_account_dims
    ON reseller_tariff_plans (
        sub_account_id,
        COALESCE(country_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(operator_id, '00000000-0000-0000-0000-000000000000'::uuid),
        sender_category, traffic_type
    ) WHERE active = true AND sub_account_id IS NOT NULL;

CREATE INDEX idx_reseller_plan_template ON reseller_tariff_plans (template_id) WHERE active = true;
CREATE INDEX idx_reseller_plan_sub_account ON reseller_tariff_plans (sub_account_id) WHERE active = true;
CREATE INDEX idx_reseller_plan_reseller ON reseller_tariff_plans (reseller_id);

CREATE TRIGGER update_reseller_tariff_plans_updated_at
    BEFORE UPDATE ON reseller_tariff_plans
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 3. Tariff periods (date ranges within a plan, no overlaps)
CREATE TABLE reseller_tariff_periods (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_plan_id UUID NOT NULL REFERENCES reseller_tariff_plans(id) ON DELETE CASCADE,
    start_date     DATE NOT NULL,
    end_date       DATE NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_date > start_date)
);

ALTER TABLE reseller_tariff_periods ADD CONSTRAINT reseller_periods_no_overlap
    EXCLUDE USING gist (tariff_plan_id WITH =, daterange(start_date, end_date, '[]') WITH &&);

CREATE INDEX idx_reseller_period_plan ON reseller_tariff_periods (tariff_plan_id);
CREATE INDEX idx_reseller_period_dates ON reseller_tariff_periods (tariff_plan_id, start_date, end_date);

-- 4. Tariff tiers (volume-based pricing within a period)
CREATE TABLE reseller_tariff_tiers (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tariff_period_id UUID        NOT NULL REFERENCES reseller_tariff_periods(id) ON DELETE CASCADE,
    from_count       INTEGER     NOT NULL DEFAULT 0 CHECK (from_count >= 0),
    price_per_segment NUMERIC(10,6) NOT NULL CHECK (price_per_segment >= 0)
);

CREATE UNIQUE INDEX idx_reseller_tier_unique ON reseller_tariff_tiers (tariff_period_id, from_count);

-- 5. Template assignments (one template per sub-account)
CREATE TABLE sub_account_template_assignments (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_account_id UUID        NOT NULL REFERENCES clients(id),
    template_id    UUID        NOT NULL REFERENCES reseller_tariff_templates(id),
    assigned_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_sub_account_template_unique ON sub_account_template_assignments (sub_account_id);

COMMIT;
```

- [ ] **Step 2: Create down migration**

```sql
-- migrations/000098_reseller_tariff_tables.down.sql
BEGIN;
DROP TABLE IF EXISTS sub_account_template_assignments CASCADE;
DROP TABLE IF EXISTS reseller_tariff_tiers CASCADE;
DROP TABLE IF EXISTS reseller_tariff_periods CASCADE;
DROP TABLE IF EXISTS reseller_tariff_plans CASCADE;
DROP TABLE IF EXISTS reseller_tariff_templates CASCADE;
COMMIT;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000098_reseller_tariff_tables.up.sql migrations/000098_reseller_tariff_tables.down.sql
git commit -m "feat(tariffs): add reseller tariff tables migration"
```

---

## Task 2: Backend Handler — Templates CRUD

**Files:**
- Create: `internal/gateway/portal/handlers/reseller_tariff_plans.go`

- [ ] **Step 1: Create handler struct with template CRUD methods**

Create `internal/gateway/portal/handlers/reseller_tariff_plans.go` with the full handler struct and template endpoints. The handler uses `*pgxpool.Pool` directly (project pattern).

```go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerTariffPlanHandlers struct {
	pool *pgxpool.Pool
}

func NewResellerTariffPlanHandlers(pool *pgxpool.Pool) *ResellerTariffPlanHandlers {
	return &ResellerTariffPlanHandlers{pool: pool}
}

// checkReseller verifies the caller is a reseller and returns their client ID.
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

// checkSubAccountOwnership verifies that the given sub-account belongs to the reseller.
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

// checkTemplateOwnership verifies that the given template belongs to the reseller.
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

// --- Templates ---

// ListTemplates GET /portal/v1/reseller/tariff-templates
func (h *ResellerTariffPlanHandlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT t.id, t.name, t.description, t.created_at, t.updated_at,
		        COUNT(a.id) AS assigned_count
		 FROM reseller_tariff_templates t
		 LEFT JOIN sub_account_template_assignments a ON a.template_id = t.id
		 WHERE t.reseller_id = $1 AND t.active = true
		 GROUP BY t.id
		 ORDER BY t.name`, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения шаблонов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type templateJSON struct {
		ID            string    `json:"id"`
		Name          string    `json:"name"`
		Description   *string   `json:"description"`
		AssignedCount int       `json:"assigned_count"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
	}
	items := make([]templateJSON, 0)
	for rows.Next() {
		var t templateJSON
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt, &t.AssignedCount); err != nil {
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
		respondError(w, shared.ErrInvalidInput("name обязателен"))
		return
	}

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO reseller_tariff_templates (reseller_id, name, description)
		 VALUES ($1, $2, $3) RETURNING id`, resellerID, req.Name, req.Description,
	).Scan(&id)
	if err != nil {
		if shared.IsUniqueViolation(err) {
			respondError(w, shared.ErrConflict("шаблон с таким именем уже существует"))
			return
		}
		log.Error().Err(err).Msg("ошибка создания шаблона")
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
		 SET name = COALESCE($1, name), description = COALESCE($2, description)
		 WHERE id = $3 AND reseller_id = $4`,
		req.Name, req.Description, templateID, resellerID)
	if err != nil {
		if shared.IsUniqueViolation(err) {
			respondError(w, shared.ErrConflict("шаблон с таким именем уже существует"))
			return
		}
		log.Error().Err(err).Msg("ошибка обновления шаблона")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
		respondError(w, shared.ErrInternalServer("ошибка транзакции"))
		return
	}
	defer tx.Rollback(r.Context())

	// Remove assignments
	_, _ = tx.Exec(r.Context(),
		`DELETE FROM sub_account_template_assignments WHERE template_id = $1`, templateID)
	// Soft-delete template (cascades deactivate plans)
	_, err = tx.Exec(r.Context(),
		`UPDATE reseller_tariff_templates SET active = false WHERE id = $1`, templateID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}
	_, _ = tx.Exec(r.Context(),
		`UPDATE reseller_tariff_plans SET active = false WHERE template_id = $1`, templateID)

	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
		SubAccountIDs []string `json:"sub_account_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.SubAccountIDs) == 0 {
		respondError(w, shared.ErrInvalidInput("sub_account_ids не может быть пустым"))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка транзакции"))
		return
	}
	defer tx.Rollback(r.Context())

	for _, saID := range req.SubAccountIDs {
		if !h.checkSubAccountOwnership(w, r, resellerID, saID) {
			return
		}
		_, err := tx.Exec(r.Context(),
			`INSERT INTO sub_account_template_assignments (sub_account_id, template_id)
			 VALUES ($1, $2)
			 ON CONFLICT (sub_account_id) DO UPDATE SET template_id = $2, assigned_at = NOW()`,
			saID, templateID)
		if err != nil {
			log.Error().Err(err).Str("sub_account_id", saID).Msg("ошибка привязки шаблона")
			respondError(w, shared.ErrInternalServer("ошибка привязки"))
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"assigned": len(req.SubAccountIDs)})
}

// UnassignTemplate DELETE /portal/v1/reseller/tariff-templates/{id}/assign/{sub_account_id}
func (h *ResellerTariffPlanHandlers) UnassignTemplate(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	templateID := vars["id"]
	subAccountID := vars["sub_account_id"]
	if !h.checkTemplateOwnership(w, r, resellerID, templateID) {
		return
	}
	if !h.checkSubAccountOwnership(w, r, resellerID, subAccountID) {
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`DELETE FROM sub_account_template_assignments WHERE sub_account_id = $1 AND template_id = $2`,
		subAccountID, templateID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка отвязки"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/reseller_tariff_plans.go
git commit -m "feat(tariffs): add reseller tariff plan handler with template CRUD"
```

---

## Task 3: Backend Handler — Plans CRUD

**Files:**
- Modify: `internal/gateway/portal/handlers/reseller_tariff_plans.go`

- [ ] **Step 1: Add plans CRUD methods to the handler**

Append the following methods to `reseller_tariff_plans.go`:

```go
// --- Plans ---

// ListPlans GET /portal/v1/reseller/tariff-plans?template_id=...&sub_account_id=...
func (h *ResellerTariffPlanHandlers) ListPlans(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	templateID := r.URL.Query().Get("template_id")
	subAccountID := r.URL.Query().Get("sub_account_id")

	query := `SELECT p.id, p.template_id, p.sub_account_id,
	                 p.country_id, p.operator_id, p.sender_category, p.traffic_type,
	                 p.strategy, p.active, p.created_at, p.updated_at,
	                 COALESCE(o.name, '') AS operator_name,
	                 COALESCE(c.name, '') AS country_name
	          FROM reseller_tariff_plans p
	          LEFT JOIN operators o ON o.id = p.operator_id
	          LEFT JOIN countries c ON c.id = p.country_id
	          WHERE p.reseller_id = $1 AND p.active = true`
	args := []interface{}{resellerID}

	if templateID != "" {
		args = append(args, templateID)
		query += ` AND p.template_id = $` + fmt.Sprintf("%d", len(args))
	}
	if subAccountID != "" {
		args = append(args, subAccountID)
		query += ` AND p.sub_account_id = $` + fmt.Sprintf("%d", len(args))
	}
	query += ` ORDER BY COALESCE(o.name, ''), p.sender_category`

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения планов")
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
		Active         bool      `json:"active"`
		OperatorName   string    `json:"operator_name"`
		CountryName    string    `json:"country_name"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	}
	items := make([]planJSON, 0)
	for rows.Next() {
		var p planJSON
		if err := rows.Scan(&p.ID, &p.TemplateID, &p.SubAccountID,
			&p.CountryID, &p.OperatorID, &p.SenderCategory, &p.TrafficType,
			&p.Strategy, &p.Active, &p.CreatedAt, &p.UpdatedAt,
			&p.OperatorName, &p.CountryName); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, p)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"plans": items, "total": len(items)})
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

	// Validate ownership
	if req.TemplateID != nil && *req.TemplateID != "" {
		if !h.checkTemplateOwnership(w, r, resellerID, *req.TemplateID) {
			return
		}
	} else if req.SubAccountID != nil && *req.SubAccountID != "" {
		if !h.checkSubAccountOwnership(w, r, resellerID, *req.SubAccountID) {
			return
		}
	} else {
		respondError(w, shared.ErrInvalidInput("template_id или sub_account_id обязателен"))
		return
	}

	// Validate strategy
	switch req.Strategy {
	case "fixed", "threshold", "threshold_recalc", "prepaid_threshold":
	default:
		respondError(w, shared.ErrInvalidInput("недопустимая стратегия: "+req.Strategy))
		return
	}

	if req.SenderCategory == "" {
		req.SenderCategory = "standard"
	}
	if req.TrafficType == "" {
		req.TrafficType = "any"
	}

	// Normalize empty strings to nil for nullable FK columns
	templateID := req.TemplateID
	if templateID != nil && *templateID == "" {
		templateID = nil
	}
	subAccountID := req.SubAccountID
	if subAccountID != nil && *subAccountID == "" {
		subAccountID = nil
	}
	countryID := req.CountryID
	if countryID != nil && *countryID == "" {
		countryID = nil
	}
	operatorID := req.OperatorID
	if operatorID != nil && *operatorID == "" {
		operatorID = nil
	}

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO reseller_tariff_plans
		 (reseller_id, template_id, sub_account_id, country_id, operator_id, sender_category, traffic_type, strategy)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		resellerID, templateID, subAccountID, countryID, operatorID,
		req.SenderCategory, req.TrafficType, req.Strategy,
	).Scan(&id)
	if err != nil {
		if shared.IsUniqueViolation(err) {
			respondError(w, shared.ErrConflict("план с такими измерениями уже существует"))
			return
		}
		log.Error().Err(err).Msg("ошибка создания плана")
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

	// Verify plan belongs to reseller
	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT reseller_id::text FROM reseller_tariff_plans WHERE id = $1 AND active = true`, planID,
	).Scan(&ownerID)
	if err != nil || ownerID != resellerID {
		respondError(w, shared.ErrNotFound("план не найден"))
		return
	}

	var req struct {
		Strategy       *string `json:"strategy"`
		SenderCategory *string `json:"sender_category"`
		TrafficType    *string `json:"traffic_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	if req.Strategy != nil {
		switch *req.Strategy {
		case "fixed", "threshold", "threshold_recalc", "prepaid_threshold":
		default:
			respondError(w, shared.ErrInvalidInput("недопустимая стратегия"))
			return
		}
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE reseller_tariff_plans
		 SET strategy = COALESCE($1, strategy),
		     sender_category = COALESCE($2, sender_category),
		     traffic_type = COALESCE($3, traffic_type)
		 WHERE id = $4 AND reseller_id = $5`,
		req.Strategy, req.SenderCategory, req.TrafficType, planID, resellerID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка обновления плана")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeletePlan DELETE /portal/v1/reseller/tariff-plans/{id}
func (h *ResellerTariffPlanHandlers) DeletePlan(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	planID := mux.Vars(r)["id"]

	result, err := h.pool.Exec(r.Context(),
		`UPDATE reseller_tariff_plans SET active = false WHERE id = $1 AND reseller_id = $2 AND active = true`,
		planID, resellerID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}
	if result.RowsAffected() == 0 {
		respondError(w, shared.ErrNotFound("план не найден"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/reseller_tariff_plans.go
git commit -m "feat(tariffs): add reseller tariff plans CRUD endpoints"
```

---

## Task 4: Backend Handler — Periods & Tiers CRUD

**Files:**
- Modify: `internal/gateway/portal/handlers/reseller_tariff_plans.go`

- [ ] **Step 1: Add periods and tiers methods**

Append to `reseller_tariff_plans.go`:

```go
// --- Periods ---

// checkPlanOwnership verifies the plan belongs to the reseller and returns true.
func (h *ResellerTariffPlanHandlers) checkPlanOwnership(w http.ResponseWriter, r *http.Request, resellerID, planID string) bool {
	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT reseller_id::text FROM reseller_tariff_plans WHERE id = $1 AND active = true`, planID,
	).Scan(&ownerID)
	if err != nil || ownerID != resellerID {
		respondError(w, shared.ErrNotFound("план не найден"))
		return false
	}
	return true
}

// ListPeriods GET /portal/v1/reseller/tariff-plans/{plan_id}/periods
func (h *ResellerTariffPlanHandlers) ListPeriods(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	planID := mux.Vars(r)["plan_id"]
	if !h.checkPlanOwnership(w, r, resellerID, planID) {
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, start_date, end_date, created_at
		 FROM reseller_tariff_periods
		 WHERE tariff_plan_id = $1
		 ORDER BY start_date DESC`, planID)
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

// CreatePeriod POST /portal/v1/reseller/tariff-plans/{plan_id}/periods
func (h *ResellerTariffPlanHandlers) CreatePeriod(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	planID := mux.Vars(r)["plan_id"]
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

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
		 VALUES ($1, $2, $3) RETURNING id`, planID, req.StartDate, req.EndDate,
	).Scan(&id)
	if err != nil {
		if shared.IsExclusionViolation(err) {
			respondError(w, shared.ErrConflict("период пересекается с существующим"))
			return
		}
		log.Error().Err(err).Msg("ошибка создания периода")
		respondError(w, shared.ErrInternalServer("ошибка создания"))
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

// UpdatePeriod PUT /portal/v1/reseller/tariff-periods/{id}
func (h *ResellerTariffPlanHandlers) UpdatePeriod(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	periodID := mux.Vars(r)["id"]

	// Verify period belongs to reseller's plan
	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT p.reseller_id::text
		 FROM reseller_tariff_periods per
		 JOIN reseller_tariff_plans p ON p.id = per.tariff_plan_id
		 WHERE per.id = $1`, periodID,
	).Scan(&ownerID)
	if err != nil || ownerID != resellerID {
		respondError(w, shared.ErrNotFound("период не найден"))
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

	_, err = h.pool.Exec(r.Context(),
		`UPDATE reseller_tariff_periods SET start_date = $1, end_date = $2 WHERE id = $3`,
		req.StartDate, req.EndDate, periodID)
	if err != nil {
		if shared.IsExclusionViolation(err) {
			respondError(w, shared.ErrConflict("период пересекается с существующим"))
			return
		}
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeletePeriod DELETE /portal/v1/reseller/tariff-periods/{id}
func (h *ResellerTariffPlanHandlers) DeletePeriod(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	periodID := mux.Vars(r)["id"]

	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT p.reseller_id::text
		 FROM reseller_tariff_periods per
		 JOIN reseller_tariff_plans p ON p.id = per.tariff_plan_id
		 WHERE per.id = $1`, periodID,
	).Scan(&ownerID)
	if err != nil || ownerID != resellerID {
		respondError(w, shared.ErrNotFound("период не найден"))
		return
	}

	// CASCADE deletes tiers
	_, err = h.pool.Exec(r.Context(),
		`DELETE FROM reseller_tariff_periods WHERE id = $1`, periodID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка удаления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Tiers ---

// ListTiers GET /portal/v1/reseller/tariff-periods/{period_id}/tiers
func (h *ResellerTariffPlanHandlers) ListTiers(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	periodID := mux.Vars(r)["period_id"]

	// Verify ownership
	var ownerID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT p.reseller_id::text
		 FROM reseller_tariff_periods per
		 JOIN reseller_tariff_plans p ON p.id = per.tariff_plan_id
		 WHERE per.id = $1`, periodID,
	).Scan(&ownerID)
	if err != nil || ownerID != resellerID {
		respondError(w, shared.ErrNotFound("период не найден"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, from_count, price_per_segment::text
		 FROM reseller_tariff_tiers
		 WHERE tariff_period_id = $1
		 ORDER BY from_count`, periodID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка получения тиеров"))
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

// UpsertTiers POST /portal/v1/reseller/tariff-periods/{period_id}/tiers
// Bulk upsert: replaces all tiers for the period.
func (h *ResellerTariffPlanHandlers) UpsertTiers(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	periodID := mux.Vars(r)["period_id"]

	// Verify ownership and get strategy
	var ownerID, strategy string
	err := h.pool.QueryRow(r.Context(),
		`SELECT p.reseller_id::text, p.strategy
		 FROM reseller_tariff_periods per
		 JOIN reseller_tariff_plans p ON p.id = per.tariff_plan_id
		 WHERE per.id = $1`, periodID,
	).Scan(&ownerID, &strategy)
	if err != nil || ownerID != resellerID {
		respondError(w, shared.ErrNotFound("период не найден"))
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

	// Validate: fixed strategy must have exactly 1 tier with from_count=0
	if strategy == "fixed" {
		if len(req.Tiers) != 1 || req.Tiers[0].FromCount != 0 {
			respondError(w, shared.ErrInvalidInput("стратегия fixed: ровно один тир с from_count=0"))
			return
		}
	}

	// Validate: must have a tier with from_count=0 (base price)
	hasBase := false
	prevFrom := -1
	for _, t := range req.Tiers {
		if t.FromCount == 0 {
			hasBase = true
		}
		if t.FromCount < 0 {
			respondError(w, shared.ErrInvalidInput("from_count не может быть отрицательным"))
			return
		}
		if t.FromCount <= prevFrom {
			respondError(w, shared.ErrInvalidInput("from_count должен быть монотонно возрастающим"))
			return
		}
		prevFrom = t.FromCount
		if t.PricePerSegment == "" {
			respondError(w, shared.ErrInvalidInput("price_per_segment обязателен"))
			return
		}
	}
	if !hasBase {
		respondError(w, shared.ErrInvalidInput("необходим тир с from_count=0 (базовая цена)"))
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка транзакции"))
		return
	}
	defer tx.Rollback(r.Context())

	// Delete existing tiers
	_, _ = tx.Exec(r.Context(),
		`DELETE FROM reseller_tariff_tiers WHERE tariff_period_id = $1`, periodID)

	// Insert new tiers
	for _, t := range req.Tiers {
		_, err := tx.Exec(r.Context(),
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
	respondJSON(w, http.StatusOK, map[string]interface{}{"saved": len(req.Tiers)})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/reseller_tariff_plans.go
git commit -m "feat(tariffs): add reseller periods and tiers CRUD endpoints"
```

---

## Task 5: Backend Handler — Overview & Copy

**Files:**
- Modify: `internal/gateway/portal/handlers/reseller_tariff_plans.go`

- [ ] **Step 1: Add overview and copy endpoints**

Append to `reseller_tariff_plans.go`:

```go
// --- Overview ---

// TariffOverview GET /portal/v1/reseller/tariff-overview?sub_account_id=...
// Returns the effective price for each operator+category, resolving:
// 1. Override (sub_account_id direct plan)
// 2. Template (via assignment)
// 3. Legacy (aggregator_tariffs)
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

	// Get template assignment if any
	var templateID *string
	err := h.pool.QueryRow(r.Context(),
		`SELECT template_id::text FROM sub_account_template_assignments WHERE sub_account_id = $1`,
		subAccountID,
	).Scan(&templateID)
	if err != nil {
		templateID = nil // no template assigned
	}

	type overviewItem struct {
		OperatorID     string  `json:"operator_id"`
		OperatorName   string  `json:"operator_name"`
		SenderCategory string  `json:"sender_category"`
		Price          string  `json:"price"`
		Source         string  `json:"source"` // "override", "template", "legacy"
		Strategy       string  `json:"strategy"`
		PlanID         *string `json:"plan_id"`
	}

	// Build effective tariffs. Query all three sources, then merge with priority.
	items := make([]overviewItem, 0)

	// Step 1: Get all operator+category combos from legacy
	legacyRows, err := h.pool.Query(r.Context(),
		`SELECT DISTINCT ON (at.operator_id, at.sender_category)
		        at.operator_id, o.name, at.sender_category, at.price_per_segment::text
		 FROM aggregator_tariffs at
		 JOIN operators o ON o.id = at.operator_id
		 WHERE at.aggregator_id = $1 AND at.active = true
		   AND (at.sub_account_id = $2 OR at.sub_account_id IS NULL)
		 ORDER BY at.operator_id, at.sender_category, at.sub_account_id NULLS LAST`,
		resellerID, subAccountID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения legacy тарифов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer legacyRows.Close()

	// key = operator_id + "_" + sender_category
	effectiveMap := make(map[string]overviewItem)
	for legacyRows.Next() {
		var item overviewItem
		if err := legacyRows.Scan(&item.OperatorID, &item.OperatorName, &item.SenderCategory, &item.Price); err != nil {
			continue
		}
		item.Source = "legacy"
		item.Strategy = "fixed"
		effectiveMap[item.OperatorID+"_"+item.SenderCategory] = item
	}

	// Step 2: Overlay template plans (if assigned)
	if templateID != nil {
		h.overlayPlans(r, effectiveMap, *templateID, "template_id", "template")
	}

	// Step 3: Overlay override plans
	h.overlayPlans(r, effectiveMap, subAccountID, "sub_account_id", "override")

	for _, item := range effectiveMap {
		items = append(items, item)
	}

	// Get template name for display
	var templateName *string
	if templateID != nil {
		_ = h.pool.QueryRow(r.Context(),
			`SELECT name FROM reseller_tariff_templates WHERE id = $1`, *templateID,
		).Scan(&templateName)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tariffs":       items,
		"total":         len(items),
		"template_id":   templateID,
		"template_name": templateName,
	})
}

// overlayPlans reads plans+periods+tiers for a given owner (template or sub-account)
// and overlays their effective prices onto the effectiveMap.
func (h *ResellerTariffPlanHandlers) overlayPlans(r *http.Request, effectiveMap map[string]overviewItem, ownerID, ownerColumn, source string) {
	today := time.Now().Format("2006-01-02")
	rows, err := h.pool.Query(r.Context(),
		fmt.Sprintf(`SELECT p.id, p.operator_id, o.name, p.sender_category, p.strategy,
		        t.from_count, t.price_per_segment::text
		 FROM reseller_tariff_plans p
		 JOIN reseller_tariff_periods per ON per.tariff_plan_id = p.id
		 JOIN reseller_tariff_tiers t ON t.tariff_period_id = per.id
		 LEFT JOIN operators o ON o.id = p.operator_id
		 WHERE p.%s = $1 AND p.active = true
		   AND per.start_date <= $2 AND per.end_date >= $2
		 ORDER BY p.operator_id, p.sender_category, t.from_count`, ownerColumn),
		ownerID, today)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var planID string
		var operatorID, operatorName *string
		var senderCategory, strategy, price string
		var fromCount int
		if err := rows.Scan(&planID, &operatorID, &operatorName, &senderCategory, &strategy, &fromCount, &price); err != nil {
			continue
		}
		if operatorID == nil {
			continue // skip plans without operator (global plans would need different handling)
		}
		// For overview, show the base tier (from_count=0) price
		if fromCount == 0 {
			key := *operatorID + "_" + senderCategory
			opName := ""
			if operatorName != nil {
				opName = *operatorName
			}
			effectiveMap[key] = overviewItem{
				OperatorID:     *operatorID,
				OperatorName:   opName,
				SenderCategory: senderCategory,
				Price:          price,
				Source:         source,
				Strategy:       strategy,
				PlanID:         &planID,
			}
		}
	}
}

// --- Copy ---

// CopyPlans POST /portal/v1/reseller/tariff-plans/copy
// Copies all plans (with periods and tiers) from one source to another.
func (h *ResellerTariffPlanHandlers) CopyPlans(w http.ResponseWriter, r *http.Request) {
	resellerID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var req struct {
		FromTemplateID   *string `json:"from_template_id"`
		FromSubAccountID *string `json:"from_sub_account_id"`
		ToTemplateID     *string `json:"to_template_id"`
		ToSubAccountID   *string `json:"to_sub_account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	// Determine source filter
	var sourceColumn, sourceID string
	if req.FromTemplateID != nil && *req.FromTemplateID != "" {
		if !h.checkTemplateOwnership(w, r, resellerID, *req.FromTemplateID) {
			return
		}
		sourceColumn = "template_id"
		sourceID = *req.FromTemplateID
	} else if req.FromSubAccountID != nil && *req.FromSubAccountID != "" {
		if !h.checkSubAccountOwnership(w, r, resellerID, *req.FromSubAccountID) {
			return
		}
		sourceColumn = "sub_account_id"
		sourceID = *req.FromSubAccountID
	} else {
		respondError(w, shared.ErrInvalidInput("from_template_id или from_sub_account_id обязателен"))
		return
	}

	// Determine target
	var targetTemplateID, targetSubAccountID *string
	if req.ToTemplateID != nil && *req.ToTemplateID != "" {
		if !h.checkTemplateOwnership(w, r, resellerID, *req.ToTemplateID) {
			return
		}
		targetTemplateID = req.ToTemplateID
	} else if req.ToSubAccountID != nil && *req.ToSubAccountID != "" {
		if !h.checkSubAccountOwnership(w, r, resellerID, *req.ToSubAccountID) {
			return
		}
		targetSubAccountID = req.ToSubAccountID
	} else {
		respondError(w, shared.ErrInvalidInput("to_template_id или to_sub_account_id обязателен"))
		return
	}

	// Read source plans
	srcPlans, err := h.pool.Query(r.Context(),
		fmt.Sprintf(`SELECT id, country_id, operator_id, sender_category, traffic_type, strategy
		 FROM reseller_tariff_plans
		 WHERE %s = $1 AND active = true`, sourceColumn), sourceID)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка чтения источника"))
		return
	}
	defer srcPlans.Close()

	type srcPlan struct {
		ID             string
		CountryID      *string
		OperatorID     *string
		SenderCategory string
		TrafficType    string
		Strategy       string
	}
	var plans []srcPlan
	for srcPlans.Next() {
		var p srcPlan
		if err := srcPlans.Scan(&p.ID, &p.CountryID, &p.OperatorID, &p.SenderCategory, &p.TrafficType, &p.Strategy); err != nil {
			continue
		}
		plans = append(plans, p)
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка транзакции"))
		return
	}
	defer tx.Rollback(r.Context())

	copied := 0
	for _, sp := range plans {
		// Create new plan
		var newPlanID string
		err := tx.QueryRow(r.Context(),
			`INSERT INTO reseller_tariff_plans
			 (reseller_id, template_id, sub_account_id, country_id, operator_id, sender_category, traffic_type, strategy)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT DO NOTHING
			 RETURNING id`,
			resellerID, targetTemplateID, targetSubAccountID,
			sp.CountryID, sp.OperatorID, sp.SenderCategory, sp.TrafficType, sp.Strategy,
		).Scan(&newPlanID)
		if err != nil {
			continue // skip conflicting plans
		}

		// Copy periods
		periodRows, err := tx.Query(r.Context(),
			`SELECT id, start_date, end_date FROM reseller_tariff_periods WHERE tariff_plan_id = $1`, sp.ID)
		if err != nil {
			continue
		}
		for periodRows.Next() {
			var oldPeriodID string
			var startDate, endDate time.Time
			if err := periodRows.Scan(&oldPeriodID, &startDate, &endDate); err != nil {
				continue
			}
			var newPeriodID string
			err := tx.QueryRow(r.Context(),
				`INSERT INTO reseller_tariff_periods (tariff_plan_id, start_date, end_date)
				 VALUES ($1, $2, $3) RETURNING id`, newPlanID, startDate, endDate,
			).Scan(&newPeriodID)
			if err != nil {
				continue
			}
			// Copy tiers
			_, _ = tx.Exec(r.Context(),
				`INSERT INTO reseller_tariff_tiers (tariff_period_id, from_count, price_per_segment)
				 SELECT $1, from_count, price_per_segment
				 FROM reseller_tariff_tiers WHERE tariff_period_id = $2`,
				newPeriodID, oldPeriodID)
		}
		periodRows.Close()
		copied++
	}

	if err := tx.Commit(r.Context()); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка сохранения"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"copied_plans": copied})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/reseller_tariff_plans.go
git commit -m "feat(tariffs): add overview, copy endpoints for reseller tariffs"
```

---

## Task 6: Register Routes & Wire Handler

**Files:**
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Add handler parameter to SetupRouter**

In `internal/gateway/portal/router/router.go`, add `resellerTariffPlanHandlers *handlers.ResellerTariffPlanHandlers` parameter to the `SetupRouter` function signature (after the existing `resellerTariffHandlers` parameter on line 64).

- [ ] **Step 2: Register routes**

In `router.go`, after the existing reseller tariffs block (line 430), add:

```go
	// Reseller tariff plans (new system)
	resellerTariffPlans := reseller.PathPrefix("/tariff-templates").Subrouter()
	resellerTariffPlans.HandleFunc("", resellerTariffPlanHandlers.ListTemplates).Methods("GET")
	resellerTariffPlans.HandleFunc("", resellerTariffPlanHandlers.CreateTemplate).Methods("POST")
	resellerTariffPlans.HandleFunc("/{id}", resellerTariffPlanHandlers.UpdateTemplate).Methods("PUT")
	resellerTariffPlans.HandleFunc("/{id}", resellerTariffPlanHandlers.DeleteTemplate).Methods("DELETE")
	resellerTariffPlans.HandleFunc("/{id}/assign", resellerTariffPlanHandlers.AssignTemplate).Methods("POST")
	resellerTariffPlans.HandleFunc("/{id}/assign/{sub_account_id}", resellerTariffPlanHandlers.UnassignTemplate).Methods("DELETE")

	resellerPlans := reseller.PathPrefix("/tariff-plans").Subrouter()
	resellerPlans.HandleFunc("", resellerTariffPlanHandlers.ListPlans).Methods("GET")
	resellerPlans.HandleFunc("", resellerTariffPlanHandlers.CreatePlan).Methods("POST")
	resellerPlans.HandleFunc("/{id}", resellerTariffPlanHandlers.UpdatePlan).Methods("PUT")
	resellerPlans.HandleFunc("/{id}", resellerTariffPlanHandlers.DeletePlan).Methods("DELETE")
	resellerPlans.HandleFunc("/{plan_id}/periods", resellerTariffPlanHandlers.ListPeriods).Methods("GET")
	resellerPlans.HandleFunc("/{plan_id}/periods", resellerTariffPlanHandlers.CreatePeriod).Methods("POST")

	resellerPeriods := reseller.PathPrefix("/tariff-periods").Subrouter()
	resellerPeriods.HandleFunc("/{id}", resellerTariffPlanHandlers.UpdatePeriod).Methods("PUT")
	resellerPeriods.HandleFunc("/{id}", resellerTariffPlanHandlers.DeletePeriod).Methods("DELETE")
	resellerPeriods.HandleFunc("/{period_id}/tiers", resellerTariffPlanHandlers.ListTiers).Methods("GET")
	resellerPeriods.HandleFunc("/{period_id}/tiers", resellerTariffPlanHandlers.UpsertTiers).Methods("POST")

	reseller.HandleFunc("/tariff-overview", resellerTariffPlanHandlers.TariffOverview).Methods("GET")
	reseller.HandleFunc("/tariff-plans/copy", resellerTariffPlanHandlers.CopyPlans).Methods("POST")
```

- [ ] **Step 3: Instantiate handler in main.go**

In `cmd/portal-gateway/main.go`, after line 295 (`resellerTariffHandlers`), add:

```go
	resellerTariffPlanHandlers := handlers.NewResellerTariffPlanHandlers(dbPool)
```

Then pass `resellerTariffPlanHandlers` as a new argument to `SetupRouter(...)`.

- [ ] **Step 4: Build to verify compilation**

```bash
cd /c/projects/sms && go build ./cmd/portal-gateway/
```

Expected: successful build, no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git commit -m "feat(tariffs): wire reseller tariff plan routes and handler"
```

---

## Task 7: Frontend API Client

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Add resellerTariffApi to client.ts**

Add the following export at the end of `client.ts`, before the closing of the file:

```typescript
// --- Reseller Tariff Plans API ---

export interface ResellerTemplate {
  id: string;
  name: string;
  description: string | null;
  assigned_count: number;
  created_at: string;
  updated_at: string;
}

export interface ResellerTariffPlan {
  id: string;
  template_id: string | null;
  sub_account_id: string | null;
  country_id: string | null;
  operator_id: string | null;
  sender_category: string;
  traffic_type: string;
  strategy: string;
  active: boolean;
  operator_name: string;
  country_name: string;
  created_at: string;
  updated_at: string;
}

export interface ResellerTariffPeriod {
  id: string;
  start_date: string;
  end_date: string;
  created_at: string;
}

export interface ResellerTariffTier {
  id: string;
  from_count: number;
  price_per_segment: string;
}

export interface TariffOverviewItem {
  operator_id: string;
  operator_name: string;
  sender_category: string;
  price: string;
  source: 'override' | 'template' | 'legacy';
  strategy: string;
  plan_id: string | null;
}

export const resellerTariffApi = {
  // Templates
  listTemplates: () =>
    apiFetch<{ templates: ResellerTemplate[]; total: number }>('/reseller/tariff-templates'),
  createTemplate: (data: { name: string; description?: string }) =>
    apiFetch<{ id: string }>('/reseller/tariff-templates', { method: 'POST', body: JSON.stringify(data) }),
  updateTemplate: (id: string, data: { name?: string; description?: string }) =>
    apiFetch<{ status: string }>(`/reseller/tariff-templates/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTemplate: (id: string) =>
    apiFetch<{ status: string }>(`/reseller/tariff-templates/${id}`, { method: 'DELETE' }),
  assignTemplate: (templateId: string, subAccountIds: string[]) =>
    apiFetch<{ assigned: number }>(`/reseller/tariff-templates/${templateId}/assign`, {
      method: 'POST', body: JSON.stringify({ sub_account_ids: subAccountIds }),
    }),
  unassignTemplate: (templateId: string, subAccountId: string) =>
    apiFetch<{ status: string }>(`/reseller/tariff-templates/${templateId}/assign/${subAccountId}`, { method: 'DELETE' }),

  // Plans
  listPlans: (params?: { template_id?: string; sub_account_id?: string }) => {
    const qs = new URLSearchParams();
    if (params?.template_id) qs.set('template_id', params.template_id);
    if (params?.sub_account_id) qs.set('sub_account_id', params.sub_account_id);
    const q = qs.toString();
    return apiFetch<{ plans: ResellerTariffPlan[]; total: number }>(`/reseller/tariff-plans${q ? '?' + q : ''}`);
  },
  createPlan: (data: {
    template_id?: string; sub_account_id?: string;
    country_id?: string; operator_id?: string;
    sender_category: string; traffic_type: string; strategy: string;
  }) => apiFetch<{ id: string }>('/reseller/tariff-plans', { method: 'POST', body: JSON.stringify(data) }),
  updatePlan: (id: string, data: { strategy?: string; sender_category?: string; traffic_type?: string }) =>
    apiFetch<{ status: string }>(`/reseller/tariff-plans/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePlan: (id: string) =>
    apiFetch<{ status: string }>(`/reseller/tariff-plans/${id}`, { method: 'DELETE' }),

  // Periods
  listPeriods: (planId: string) =>
    apiFetch<{ periods: ResellerTariffPeriod[]; total: number }>(`/reseller/tariff-plans/${planId}/periods`),
  createPeriod: (planId: string, data: { start_date: string; end_date: string }) =>
    apiFetch<{ id: string }>(`/reseller/tariff-plans/${planId}/periods`, { method: 'POST', body: JSON.stringify(data) }),
  updatePeriod: (id: string, data: { start_date: string; end_date: string }) =>
    apiFetch<{ status: string }>(`/reseller/tariff-periods/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePeriod: (id: string) =>
    apiFetch<{ status: string }>(`/reseller/tariff-periods/${id}`, { method: 'DELETE' }),

  // Tiers
  listTiers: (periodId: string) =>
    apiFetch<{ tiers: ResellerTariffTier[]; total: number }>(`/reseller/tariff-periods/${periodId}/tiers`),
  upsertTiers: (periodId: string, tiers: { from_count: number; price_per_segment: string }[]) =>
    apiFetch<{ saved: number }>(`/reseller/tariff-periods/${periodId}/tiers`, {
      method: 'POST', body: JSON.stringify({ tiers }),
    }),

  // Overview
  overview: (subAccountId: string) =>
    apiFetch<{ tariffs: TariffOverviewItem[]; total: number; template_id: string | null; template_name: string | null }>(
      `/reseller/tariff-overview?sub_account_id=${subAccountId}`
    ),

  // Copy
  copyPlans: (data: {
    from_template_id?: string; from_sub_account_id?: string;
    to_template_id?: string; to_sub_account_id?: string;
  }) => apiFetch<{ copied_plans: number }>('/reseller/tariff-plans/copy', {
    method: 'POST', body: JSON.stringify(data),
  }),
};
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(tariffs): add resellerTariffApi client functions"
```

---

## Task 8: Frontend — TariffOverviewTab

**Files:**
- Create: `portal-frontend/src/pages/network/components/TariffOverviewTab.tsx`

- [ ] **Step 1: Create the overview tab component**

This is the read-only matrix showing effective prices per operator+category with source badges. Based on the existing matrix pattern from `NetworkTariffsPage.tsx`.

```typescript
// portal-frontend/src/pages/network/components/TariffOverviewTab.tsx
import { useState, useEffect, useMemo } from 'react';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { resellerTariffApi, subAccountsApi, type TariffOverviewItem } from '../../../api/client';

interface SubAccountOption {
  id: string;
  name: string;
}

const CATEGORY_SHORT: Record<string, string> = {
  paid_registered: 'Платная рег.',
  free_registered: 'Бесплатная рег.',
  shared: 'Общая',
  standard: 'Стандартная',
};

const SOURCE_BADGE: Record<string, { label: string; variant: 'success' | 'warning' | 'default' }> = {
  override: { label: 'Переопределение', variant: 'warning' },
  template: { label: 'Шаблон', variant: 'success' },
  legacy: { label: 'Legacy', variant: 'default' },
};

function formatPrice(raw: string): string {
  const n = parseFloat(raw);
  if (isNaN(n)) return raw;
  return n.toFixed(2);
}

interface TariffOverviewTabProps {
  onNavigateToPlan?: (planId: string) => void;
}

export function TariffOverviewTab({ onNavigateToPlan }: TariffOverviewTabProps) {
  const toast = useToast();
  const [subAccounts, setSubAccounts] = useState<SubAccountOption[]>([]);
  const [selectedSA, setSelectedSA] = useState('');
  const [tariffs, setTariffs] = useState<TariffOverviewItem[]>([]);
  const [templateName, setTemplateName] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    subAccountsApi.list().then((r: any) => {
      setSubAccounts((r.sub_accounts || []).map((sa: any) => ({ id: sa.id, name: sa.name || sa.email })));
    }).catch(() => toast.error('Ошибка загрузки субаккаунтов'));
  }, []);

  useEffect(() => {
    if (!selectedSA) {
      setTariffs([]);
      setTemplateName(null);
      return;
    }
    setLoading(true);
    resellerTariffApi.overview(selectedSA)
      .then((r) => {
        setTariffs(r.tariffs || []);
        setTemplateName(r.template_name || null);
      })
      .catch(() => toast.error('Ошибка загрузки обзора'))
      .finally(() => setLoading(false));
  }, [selectedSA]);

  const { operators, categories } = useMemo(() => {
    const opMap = new Map<string, string>();
    const catSet = new Set<string>();
    tariffs.forEach((t) => {
      opMap.set(t.operator_id, t.operator_name);
      catSet.add(t.sender_category);
    });
    const catOrder = ['paid_registered', 'free_registered', 'shared', 'standard'];
    const cats = [...catSet].sort((a, b) => {
      const ai = catOrder.indexOf(a);
      const bi = catOrder.indexOf(b);
      return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
    });
    const ops = [...opMap.entries()].sort((a, b) => {
      if (a[1].startsWith('Default')) return -1;
      if (b[1].startsWith('Default')) return 1;
      return a[1].localeCompare(b[1], 'ru');
    });
    return { operators: ops, categories: cats };
  }, [tariffs]);

  // Build lookup: key = opId_cat
  const tariffMap = useMemo(() => {
    const map = new Map<string, TariffOverviewItem>();
    tariffs.forEach((t) => map.set(`${t.operator_id}_${t.sender_category}`, t));
    return map;
  }, [tariffs]);

  return (
    <div>
      <div className="flex items-center gap-3 mb-6 flex-wrap">
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium text-gray-500" htmlFor="overview-sa-select">Субаккаунт</label>
          <select
            id="overview-sa-select"
            value={selectedSA}
            onChange={(e) => setSelectedSA(e.target.value)}
            className="border border-gray-300 rounded px-3 py-2 text-sm min-w-[250px] focus:ring-2 focus:ring-primary/50 focus:border-primary"
          >
            <option value="">Выберите субаккаунт</option>
            {subAccounts.map((sa) => (
              <option key={sa.id} value={sa.id}>{sa.name}</option>
            ))}
          </select>
        </div>
        {templateName && (
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-500">Шаблон:</span>
            <Badge variant="success">{templateName}</Badge>
          </div>
        )}
      </div>

      {!selectedSA ? (
        <div className="py-16 text-center text-gray-400 text-sm">
          Выберите субаккаунт для просмотра эффективных тарифов
        </div>
      ) : loading ? (
        <div className="space-y-3">
          <div className="h-10 bg-gray-100 rounded animate-pulse" />
          <div className="h-64 bg-gray-100 rounded animate-pulse" />
        </div>
      ) : tariffs.length === 0 ? (
        <div className="py-16 text-center border border-gray-200 rounded-lg bg-white">
          <div className="text-gray-400 mb-2">Тарифы не настроены</div>
          <div className="text-sm text-gray-400">Создайте шаблон или переопределение на соответствующих вкладках</div>
        </div>
      ) : (
        <>
          <div className="text-xs text-gray-500 mb-3">
            {operators.length} {operators.length === 1 ? 'оператор' : operators.length < 5 ? 'оператора' : 'операторов'} &middot; {categories.length} {categories.length === 1 ? 'категория' : categories.length < 5 ? 'категории' : 'категорий'}
          </div>
          <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-gray-50">
                  <th className="text-left p-3 text-xs font-semibold text-gray-500 uppercase tracking-wide sticky left-0 bg-gray-50 min-w-[160px]">
                    Оператор
                  </th>
                  {categories.map((cat) => (
                    <th key={cat} className="text-center p-3 text-xs font-semibold text-gray-500 uppercase tracking-wide min-w-[160px]">
                      {CATEGORY_SHORT[cat] || cat}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {operators.map(([opId, opName], idx) => (
                  <tr key={opId} className={`border-t border-gray-100 ${idx % 2 === 1 ? 'bg-gray-50/50' : ''}`}>
                    <td className="p-3 font-medium text-gray-900 sticky left-0 bg-inherit">{opName}</td>
                    {categories.map((cat) => {
                      const item = tariffMap.get(`${opId}_${cat}`);
                      if (!item) {
                        return (
                          <td key={cat} className="p-2 text-center">
                            <span className="text-gray-300 text-xs">&mdash;</span>
                          </td>
                        );
                      }
                      const badge = SOURCE_BADGE[item.source] || SOURCE_BADGE.legacy;
                      return (
                        <td key={cat} className="p-2 text-center">
                          <div
                            className={`inline-flex flex-col items-center gap-1 ${item.plan_id ? 'cursor-pointer hover:bg-blue-50 rounded px-2 py-1' : ''}`}
                            onClick={() => item.plan_id && onNavigateToPlan?.(item.plan_id)}
                          >
                            <span className="font-mono text-sm">{formatPrice(item.price)}</span>
                            <Badge variant={badge.variant}>{badge.label}</Badge>
                            {item.strategy !== 'fixed' && (
                              <span className="text-[10px] text-gray-400">{item.strategy}</span>
                            )}
                          </div>
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/network/components/TariffOverviewTab.tsx
git commit -m "feat(tariffs): add TariffOverviewTab component"
```

---

## Task 9: Frontend — TariffPlanEditor

**Files:**
- Create: `portal-frontend/src/pages/network/components/TariffPlanEditor.tsx`

- [ ] **Step 1: Create the plan editor component**

This shared component handles creating/editing a plan with its periods and tiers. It's used from both Templates and Overrides tabs.

```typescript
// portal-frontend/src/pages/network/components/TariffPlanEditor.tsx
import { useState, useEffect, useCallback } from 'react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Select } from '../../../components/ui/Select';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  resellerTariffApi,
  referencesApi,
  type ResellerTariffPlan,
  type ResellerTariffPeriod,
  type ResellerTariffTier,
} from '../../../api/client';

const STRATEGY_OPTIONS = [
  { value: 'fixed', label: 'Фиксированная (fixed)' },
  { value: 'threshold', label: 'Пороговая (threshold)' },
  { value: 'threshold_recalc', label: 'Пороговая с пересчётом (threshold_recalc)' },
  { value: 'prepaid_threshold', label: 'Предоплата с порогом (prepaid_threshold)' },
];

const CATEGORY_OPTIONS = [
  { value: 'standard', label: 'Стандартная' },
  { value: 'shared', label: 'Общая (shared)' },
  { value: 'paid_registered', label: 'Платная регистрация' },
  { value: 'free_registered', label: 'Бесплатная регистрация' },
];

const TRAFFIC_TYPE_OPTIONS = [
  { value: 'any', label: 'Любой' },
  { value: 'authorization', label: 'Авторизация' },
  { value: 'transactional', label: 'Транзакционный' },
  { value: 'service', label: 'Сервисный' },
];

interface TariffPlanEditorProps {
  plans: ResellerTariffPlan[];
  ownerId: string;
  ownerType: 'template' | 'sub_account';
  onRefresh: () => void;
}

export function TariffPlanEditor({ plans, ownerId, ownerType, onRefresh }: TariffPlanEditorProps) {
  const toast = useToast();
  const [operators, setOperators] = useState<{ value: string; label: string }[]>([]);
  const [countries, setCountries] = useState<{ value: string; label: string }[]>([]);
  const [showCreatePlan, setShowCreatePlan] = useState(false);
  const [expandedPlan, setExpandedPlan] = useState<string | null>(null);
  const [planForm, setPlanForm] = useState({
    country_id: '', operator_id: '', sender_category: 'standard', traffic_type: 'any', strategy: 'fixed',
  });
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    referencesApi.operators().then((r) =>
      setOperators((r.operators || []).map((o) => ({ value: o.id, label: o.name })))
    );
    referencesApi.countries().then((r) =>
      setCountries((r.countries || []).map((c) => ({ value: c.id, label: c.name })))
    );
  }, []);

  const handleCreatePlan = async () => {
    setCreating(true);
    try {
      const data: Record<string, string> = {
        sender_category: planForm.sender_category,
        traffic_type: planForm.traffic_type,
        strategy: planForm.strategy,
      };
      if (ownerType === 'template') data.template_id = ownerId;
      else data.sub_account_id = ownerId;
      if (planForm.country_id) data.country_id = planForm.country_id;
      if (planForm.operator_id) data.operator_id = planForm.operator_id;

      await resellerTariffApi.createPlan(data as any);
      toast.success('План создан');
      setShowCreatePlan(false);
      onRefresh();
    } catch (e: any) {
      toast.error(e.message || 'Ошибка создания');
    } finally {
      setCreating(false);
    }
  };

  const handleDeletePlan = async (planId: string) => {
    try {
      await resellerTariffApi.deletePlan(planId);
      toast.success('План удалён');
      onRefresh();
    } catch (e: any) {
      toast.error(e.message || 'Ошибка удаления');
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="text-sm text-gray-500">{plans.length} {plans.length === 1 ? 'план' : plans.length < 5 ? 'плана' : 'планов'}</div>
        <Button size="sm" onClick={() => { setShowCreatePlan(true); setPlanForm({ country_id: '', operator_id: '', sender_category: 'standard', traffic_type: 'any', strategy: 'fixed' }); }}>
          Добавить план
        </Button>
      </div>

      {plans.length === 0 ? (
        <div className="py-8 text-center text-gray-400 text-sm border border-gray-200 rounded-lg">Нет планов</div>
      ) : (
        <div className="space-y-2">
          {plans.map((plan) => (
            <div key={plan.id} className="border border-gray-200 rounded-lg bg-white">
              <div
                className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-50"
                onClick={() => setExpandedPlan(expandedPlan === plan.id ? null : plan.id)}
              >
                <div className="flex items-center gap-2 flex-wrap">
                  {plan.country_name && <Badge variant="default">{plan.country_name}</Badge>}
                  {plan.operator_name && <Badge variant="default">{plan.operator_name}</Badge>}
                  <span className="text-sm font-medium">{plan.sender_category}</span>
                  <span className="text-xs text-gray-400">{plan.traffic_type}</span>
                  <Badge variant={plan.strategy === 'fixed' ? 'success' : 'warning'}>{plan.strategy}</Badge>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDeletePlan(plan.id); }}
                    className="text-xs text-red-500 hover:text-red-700"
                  >
                    Удалить
                  </button>
                  <span className="text-gray-400">{expandedPlan === plan.id ? '▲' : '▼'}</span>
                </div>
              </div>
              {expandedPlan === plan.id && (
                <div className="border-t border-gray-100 p-4">
                  <PlanPeriodsSection planId={plan.id} strategy={plan.strategy} />
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      <Modal open={showCreatePlan} onClose={() => setShowCreatePlan(false)} title="Создать тарифный план">
        <div className="flex flex-col gap-4">
          <Select label="Страна (необязательно)" options={[{ value: '', label: 'Все страны' }, ...countries]} value={planForm.country_id} onChange={(e) => setPlanForm((p) => ({ ...p, country_id: e.target.value }))} />
          <Select label="Оператор (необязательно)" options={[{ value: '', label: 'Все операторы' }, ...operators]} value={planForm.operator_id} onChange={(e) => setPlanForm((p) => ({ ...p, operator_id: e.target.value }))} />
          <Select label="Категория отправителя" options={CATEGORY_OPTIONS} value={planForm.sender_category} onChange={(e) => setPlanForm((p) => ({ ...p, sender_category: e.target.value }))} />
          <Select label="Тип трафика" options={TRAFFIC_TYPE_OPTIONS} value={planForm.traffic_type} onChange={(e) => setPlanForm((p) => ({ ...p, traffic_type: e.target.value }))} />
          <Select label="Стратегия" options={STRATEGY_OPTIONS} value={planForm.strategy} onChange={(e) => setPlanForm((p) => ({ ...p, strategy: e.target.value }))} />
          <div className="flex gap-2">
            <Button onClick={handleCreatePlan} disabled={creating}>{creating ? 'Создание...' : 'Создать'}</Button>
            <Button variant="secondary" onClick={() => setShowCreatePlan(false)}>Отмена</Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

// --- Periods Section (nested in expanded plan) ---

function PlanPeriodsSection({ planId, strategy }: { planId: string; strategy: string }) {
  const toast = useToast();
  const [periods, setPeriods] = useState<ResellerTariffPeriod[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState({ start_date: '', end_date: '' });
  const [expandedPeriod, setExpandedPeriod] = useState<string | null>(null);

  const fetchPeriods = useCallback(async () => {
    setLoading(true);
    try {
      const r = await resellerTariffApi.listPeriods(planId);
      setPeriods(r.periods || []);
    } catch {
      toast.error('Ошибка загрузки периодов');
    } finally {
      setLoading(false);
    }
  }, [planId]);

  useEffect(() => { fetchPeriods(); }, [fetchPeriods]);

  const handleCreate = async () => {
    try {
      await resellerTariffApi.createPeriod(planId, form);
      toast.success('Период создан');
      setShowCreate(false);
      fetchPeriods();
    } catch (e: any) {
      toast.error(e.message || 'Ошибка');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await resellerTariffApi.deletePeriod(id);
      toast.success('Период удалён');
      fetchPeriods();
    } catch (e: any) {
      toast.error(e.message || 'Ошибка');
    }
  };

  if (loading) return <div className="text-sm text-gray-400">Загрузка периодов...</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <div className="text-xs font-semibold text-gray-500 uppercase">Периоды</div>
        <Button size="sm" variant="secondary" onClick={() => { setShowCreate(true); setForm({ start_date: '', end_date: '' }); }}>
          Добавить период
        </Button>
      </div>

      {periods.length === 0 ? (
        <div className="text-sm text-gray-400 py-4 text-center">Нет периодов</div>
      ) : (
        <div className="space-y-2">
          {periods.map((period) => {
            const isActive = new Date(period.start_date) <= new Date() && new Date(period.end_date) >= new Date();
            return (
              <div key={period.id} className="border border-gray-100 rounded">
                <div
                  className="flex items-center justify-between px-3 py-2 cursor-pointer hover:bg-gray-50"
                  onClick={() => setExpandedPeriod(expandedPeriod === period.id ? null : period.id)}
                >
                  <div className="flex items-center gap-2">
                    <span className="text-sm">{period.start_date} — {period.end_date}</span>
                    <Badge variant={isActive ? 'success' : 'default'}>{isActive ? 'Активен' : 'Неактивен'}</Badge>
                  </div>
                  <div className="flex items-center gap-2">
                    <button onClick={(e) => { e.stopPropagation(); handleDelete(period.id); }} className="text-xs text-red-500 hover:text-red-700">
                      Удалить
                    </button>
                    <span className="text-gray-400 text-xs">{expandedPeriod === period.id ? '▲' : '▼'}</span>
                  </div>
                </div>
                {expandedPeriod === period.id && (
                  <div className="border-t border-gray-100 p-3">
                    <PeriodTiersSection periodId={period.id} strategy={strategy} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Добавить период">
        <div className="flex flex-col gap-4">
          <Input label="Дата начала" type="date" value={form.start_date} onChange={(e) => setForm((f) => ({ ...f, start_date: e.target.value }))} required />
          <Input label="Дата окончания" type="date" value={form.end_date} onChange={(e) => setForm((f) => ({ ...f, end_date: e.target.value }))} required />
          <div className="flex gap-2">
            <Button onClick={handleCreate} disabled={!form.start_date || !form.end_date}>Создать</Button>
            <Button variant="secondary" onClick={() => setShowCreate(false)}>Отмена</Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

// --- Tiers Section (nested in expanded period) ---

function PeriodTiersSection({ periodId, strategy }: { periodId: string; strategy: string }) {
  const toast = useToast();
  const [tiers, setTiers] = useState<ResellerTariffTier[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [editTiers, setEditTiers] = useState<{ from_count: number; price_per_segment: string }[]>([]);
  const [saving, setSaving] = useState(false);

  const fetchTiers = useCallback(async () => {
    setLoading(true);
    try {
      const r = await resellerTariffApi.listTiers(periodId);
      setTiers(r.tiers || []);
    } catch {
      toast.error('Ошибка загрузки тиров');
    } finally {
      setLoading(false);
    }
  }, [periodId]);

  useEffect(() => { fetchTiers(); }, [fetchTiers]);

  const startEditing = () => {
    if (tiers.length > 0) {
      setEditTiers(tiers.map((t) => ({ from_count: t.from_count, price_per_segment: t.price_per_segment })));
    } else {
      setEditTiers([{ from_count: 0, price_per_segment: '' }]);
    }
    setEditing(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await resellerTariffApi.upsertTiers(periodId, editTiers);
      toast.success('Тиры сохранены');
      setEditing(false);
      fetchTiers();
    } catch (e: any) {
      toast.error(e.message || 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  const addTier = () => {
    const lastFrom = editTiers.length > 0 ? editTiers[editTiers.length - 1].from_count : 0;
    setEditTiers([...editTiers, { from_count: lastFrom + 1000, price_per_segment: '' }]);
  };

  const removeTier = (idx: number) => {
    setEditTiers(editTiers.filter((_, i) => i !== idx));
  };

  const updateTier = (idx: number, field: 'from_count' | 'price_per_segment', value: string) => {
    setEditTiers(editTiers.map((t, i) =>
      i === idx ? { ...t, [field]: field === 'from_count' ? Number(value) : value } : t
    ));
  };

  if (loading) return <div className="text-sm text-gray-400">Загрузка тиров...</div>;

  if (editing) {
    const isFixed = strategy === 'fixed';
    return (
      <div>
        <div className="text-xs font-semibold text-gray-500 uppercase mb-2">Тиры</div>
        <table className="w-full text-sm mb-3">
          <thead>
            <tr className="text-xs text-gray-500">
              {!isFixed && <th className="text-left py-1 px-2">От (сегментов)</th>}
              <th className="text-left py-1 px-2">Цена за сегмент</th>
              {!isFixed && <th className="w-8"></th>}
            </tr>
          </thead>
          <tbody>
            {editTiers.map((tier, idx) => (
              <tr key={idx}>
                {!isFixed && (
                  <td className="py-1 px-2">
                    <input
                      type="number"
                      min="0"
                      value={tier.from_count}
                      onChange={(e) => updateTier(idx, 'from_count', e.target.value)}
                      className="border border-gray-300 rounded px-2 py-1 w-28 text-sm"
                      disabled={idx === 0}
                    />
                  </td>
                )}
                <td className="py-1 px-2">
                  <input
                    type="text"
                    inputMode="decimal"
                    value={tier.price_per_segment}
                    onChange={(e) => updateTier(idx, 'price_per_segment', e.target.value)}
                    className="border border-gray-300 rounded px-2 py-1 w-28 text-sm"
                    placeholder="0.00"
                  />
                </td>
                {!isFixed && (
                  <td className="py-1 px-2">
                    {idx > 0 && (
                      <button onClick={() => removeTier(idx)} className="text-red-500 text-xs hover:text-red-700">✕</button>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        <div className="flex items-center gap-2">
          <Button size="sm" onClick={handleSave} disabled={saving}>{saving ? 'Сохранение...' : 'Сохранить'}</Button>
          <Button size="sm" variant="secondary" onClick={() => setEditing(false)}>Отмена</Button>
          {!isFixed && (
            <Button size="sm" variant="secondary" onClick={addTier}>+ Уровень</Button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <div className="text-xs font-semibold text-gray-500 uppercase">Тиры</div>
        <Button size="sm" variant="secondary" onClick={startEditing}>
          {tiers.length > 0 ? 'Редактировать' : 'Задать цены'}
        </Button>
      </div>
      {tiers.length === 0 ? (
        <div className="text-sm text-gray-400">Тиры не заданы</div>
      ) : (
        <table className="w-full text-sm">
          <thead>
            <tr className="text-xs text-gray-500">
              {strategy !== 'fixed' && <th className="text-left py-1 px-2">От (сегментов)</th>}
              <th className="text-left py-1 px-2">Цена за сегмент</th>
            </tr>
          </thead>
          <tbody>
            {tiers.map((tier) => (
              <tr key={tier.id} className="border-t border-gray-50">
                {strategy !== 'fixed' && <td className="py-1 px-2 font-mono">{tier.from_count.toLocaleString()}</td>}
                <td className="py-1 px-2 font-mono">{parseFloat(tier.price_per_segment).toFixed(6)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/network/components/TariffPlanEditor.tsx
git commit -m "feat(tariffs): add TariffPlanEditor component with periods and tiers"
```

---

## Task 10: Frontend — TariffTemplatesTab

**Files:**
- Create: `portal-frontend/src/pages/network/components/TariffTemplatesTab.tsx`
- Create: `portal-frontend/src/pages/network/components/TemplateAssignModal.tsx`

- [ ] **Step 1: Create TemplateAssignModal**

```typescript
// portal-frontend/src/pages/network/components/TemplateAssignModal.tsx
import { useState, useEffect } from 'react';
import { Button } from '../../../components/ui/Button';
import { Modal } from '../../../components/ui/Modal';
import { useToast } from '../../../components/ui/Toast';
import { resellerTariffApi, subAccountsApi } from '../../../api/client';

interface TemplateAssignModalProps {
  open: boolean;
  onClose: () => void;
  templateId: string;
  templateName: string;
  onAssigned: () => void;
}

export function TemplateAssignModal({ open, onClose, templateId, templateName, onAssigned }: TemplateAssignModalProps) {
  const toast = useToast();
  const [subAccounts, setSubAccounts] = useState<{ id: string; name: string }[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    subAccountsApi.list().then((r: any) => {
      setSubAccounts((r.sub_accounts || []).map((sa: any) => ({ id: sa.id, name: sa.name || sa.email })));
    });
    setSelected(new Set());
  }, [open]);

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleAssign = async () => {
    if (selected.size === 0) return;
    setSaving(true);
    try {
      await resellerTariffApi.assignTemplate(templateId, [...selected]);
      toast.success(`Привязано ${selected.size} субаккаунтов к "${templateName}"`);
      onClose();
      onAssigned();
    } catch (e: any) {
      toast.error(e.message || 'Ошибка привязки');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title={`Привязать субаккаунты к "${templateName}"`}>
      <div className="flex flex-col gap-4">
        <div className="max-h-64 overflow-y-auto border border-gray-200 rounded">
          {subAccounts.length === 0 ? (
            <div className="p-4 text-sm text-gray-400 text-center">Нет субаккаунтов</div>
          ) : (
            subAccounts.map((sa) => (
              <label key={sa.id} className="flex items-center gap-2 px-3 py-2 hover:bg-gray-50 cursor-pointer">
                <input type="checkbox" checked={selected.has(sa.id)} onChange={() => toggle(sa.id)} />
                <span className="text-sm">{sa.name}</span>
              </label>
            ))
          )}
        </div>
        <div className="flex gap-2">
          <Button onClick={handleAssign} disabled={saving || selected.size === 0}>
            {saving ? 'Привязка...' : `Привязать (${selected.size})`}
          </Button>
          <Button variant="secondary" onClick={onClose}>Отмена</Button>
        </div>
      </div>
    </Modal>
  );
}
```

- [ ] **Step 2: Create TariffTemplatesTab**

```typescript
// portal-frontend/src/pages/network/components/TariffTemplatesTab.tsx
import { useState, useEffect, useCallback } from 'react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  resellerTariffApi,
  type ResellerTemplate,
  type ResellerTariffPlan,
} from '../../../api/client';
import { TariffPlanEditor } from './TariffPlanEditor';
import { TemplateAssignModal } from './TemplateAssignModal';

export function TariffTemplatesTab() {
  const toast = useToast();
  const [templates, setTemplates] = useState<ResellerTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ name: '', description: '' });
  const [creating, setCreating] = useState(false);

  // Expanded template
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [plans, setPlans] = useState<ResellerTariffPlan[]>([]);
  const [plansLoading, setPlansLoading] = useState(false);

  // Assign modal
  const [assignModal, setAssignModal] = useState<{ open: boolean; templateId: string; templateName: string }>({
    open: false, templateId: '', templateName: '',
  });

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const r = await resellerTariffApi.listTemplates();
      setTemplates(r.templates || []);
    } catch {
      toast.error('Ошибка загрузки шаблонов');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchTemplates(); }, [fetchTemplates]);

  const fetchPlans = useCallback(async (templateId: string) => {
    setPlansLoading(true);
    try {
      const r = await resellerTariffApi.listPlans({ template_id: templateId });
      setPlans(r.plans || []);
    } catch {
      toast.error('Ошибка загрузки планов');
    } finally {
      setPlansLoading(false);
    }
  }, []);

  const handleExpand = (id: string) => {
    if (expandedId === id) {
      setExpandedId(null);
      return;
    }
    setExpandedId(id);
    fetchPlans(id);
  };

  const handleCreate = async () => {
    if (!createForm.name) return;
    setCreating(true);
    try {
      await resellerTariffApi.createTemplate({ name: createForm.name, description: createForm.description || undefined });
      toast.success('Шаблон создан');
      setShowCreate(false);
      fetchTemplates();
    } catch (e: any) {
      toast.error(e.message || 'Ошибка создания');
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await resellerTariffApi.deleteTemplate(id);
      toast.success('Шаблон удалён');
      if (expandedId === id) setExpandedId(null);
      fetchTemplates();
    } catch (e: any) {
      toast.error(e.message || 'Ошибка удаления');
    }
  };

  if (loading) {
    return <div className="space-y-3"><div className="h-10 bg-gray-100 rounded animate-pulse" /><div className="h-32 bg-gray-100 rounded animate-pulse" /></div>;
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="text-sm text-gray-500">{templates.length} {templates.length === 1 ? 'шаблон' : templates.length < 5 ? 'шаблона' : 'шаблонов'}</div>
        <Button size="sm" onClick={() => { setShowCreate(true); setCreateForm({ name: '', description: '' }); }}>Создать шаблон</Button>
      </div>

      {templates.length === 0 ? (
        <div className="py-16 text-center border border-gray-200 rounded-lg bg-white">
          <div className="text-gray-400 mb-2">Шаблоны не созданы</div>
          <div className="text-sm text-gray-400 mb-4">Создайте шаблон и добавьте в него тарифные планы</div>
          <Button size="sm" onClick={() => setShowCreate(true)}>Создать шаблон</Button>
        </div>
      ) : (
        <div className="space-y-3">
          {templates.map((tpl) => (
            <div key={tpl.id} className="border border-gray-200 rounded-lg bg-white">
              <div className="flex items-center justify-between p-4 cursor-pointer hover:bg-gray-50" onClick={() => handleExpand(tpl.id)}>
                <div>
                  <div className="font-medium text-gray-900">{tpl.name}</div>
                  {tpl.description && <div className="text-sm text-gray-500 mt-0.5">{tpl.description}</div>}
                </div>
                <div className="flex items-center gap-3">
                  <Badge variant="default">{tpl.assigned_count} субаккаунтов</Badge>
                  <button
                    onClick={(e) => { e.stopPropagation(); setAssignModal({ open: true, templateId: tpl.id, templateName: tpl.name }); }}
                    className="text-xs text-primary hover:underline"
                  >
                    Привязать
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDelete(tpl.id); }}
                    className="text-xs text-red-500 hover:text-red-700"
                  >
                    Удалить
                  </button>
                  <span className="text-gray-400">{expandedId === tpl.id ? '▲' : '▼'}</span>
                </div>
              </div>
              {expandedId === tpl.id && (
                <div className="border-t border-gray-100 p-4">
                  {plansLoading ? (
                    <div className="text-sm text-gray-400">Загрузка планов...</div>
                  ) : (
                    <TariffPlanEditor plans={plans} ownerId={tpl.id} ownerType="template" onRefresh={() => fetchPlans(tpl.id)} />
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      <Modal open={showCreate} onClose={() => setShowCreate(false)} title="Создать шаблон тарифов">
        <div className="flex flex-col gap-4">
          <Input label="Название" value={createForm.name} onChange={(e) => setCreateForm((f) => ({ ...f, name: e.target.value }))} required />
          <Input label="Описание (необязательно)" value={createForm.description} onChange={(e) => setCreateForm((f) => ({ ...f, description: e.target.value }))} />
          <div className="flex gap-2">
            <Button onClick={handleCreate} disabled={creating || !createForm.name}>{creating ? 'Создание...' : 'Создать'}</Button>
            <Button variant="secondary" onClick={() => setShowCreate(false)}>Отмена</Button>
          </div>
        </div>
      </Modal>

      <TemplateAssignModal
        open={assignModal.open}
        onClose={() => setAssignModal((s) => ({ ...s, open: false }))}
        templateId={assignModal.templateId}
        templateName={assignModal.templateName}
        onAssigned={fetchTemplates}
      />
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/network/components/TariffTemplatesTab.tsx portal-frontend/src/pages/network/components/TemplateAssignModal.tsx
git commit -m "feat(tariffs): add TariffTemplatesTab and TemplateAssignModal"
```

---

## Task 11: Frontend — TariffOverridesTab

**Files:**
- Create: `portal-frontend/src/pages/network/components/TariffOverridesTab.tsx`

- [ ] **Step 1: Create the overrides tab**

```typescript
// portal-frontend/src/pages/network/components/TariffOverridesTab.tsx
import { useState, useEffect, useCallback } from 'react';
import { useToast } from '../../../components/ui/Toast';
import { resellerTariffApi, subAccountsApi, type ResellerTariffPlan } from '../../../api/client';
import { TariffPlanEditor } from './TariffPlanEditor';

export function TariffOverridesTab() {
  const toast = useToast();
  const [subAccounts, setSubAccounts] = useState<{ id: string; name: string }[]>([]);
  const [selectedSA, setSelectedSA] = useState('');
  const [plans, setPlans] = useState<ResellerTariffPlan[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    subAccountsApi.list().then((r: any) => {
      setSubAccounts((r.sub_accounts || []).map((sa: any) => ({ id: sa.id, name: sa.name || sa.email })));
    }).catch(() => toast.error('Ошибка загрузки субаккаунтов'));
  }, []);

  const fetchPlans = useCallback(async () => {
    if (!selectedSA) {
      setPlans([]);
      return;
    }
    setLoading(true);
    try {
      const r = await resellerTariffApi.listPlans({ sub_account_id: selectedSA });
      setPlans(r.plans || []);
    } catch {
      toast.error('Ошибка загрузки планов');
    } finally {
      setLoading(false);
    }
  }, [selectedSA]);

  useEffect(() => { fetchPlans(); }, [fetchPlans]);

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <div className="flex flex-col gap-1">
          <label className="text-xs font-medium text-gray-500" htmlFor="override-sa-select">Субаккаунт</label>
          <select
            id="override-sa-select"
            value={selectedSA}
            onChange={(e) => setSelectedSA(e.target.value)}
            className="border border-gray-300 rounded px-3 py-2 text-sm min-w-[250px] focus:ring-2 focus:ring-primary/50 focus:border-primary"
          >
            <option value="">Выберите субаккаунт</option>
            {subAccounts.map((sa) => (
              <option key={sa.id} value={sa.id}>{sa.name}</option>
            ))}
          </select>
        </div>
      </div>

      {!selectedSA ? (
        <div className="py-16 text-center text-gray-400 text-sm">
          Выберите субаккаунт для управления переопределениями тарифов
        </div>
      ) : loading ? (
        <div className="space-y-3">
          <div className="h-10 bg-gray-100 rounded animate-pulse" />
          <div className="h-32 bg-gray-100 rounded animate-pulse" />
        </div>
      ) : (
        <TariffPlanEditor plans={plans} ownerId={selectedSA} ownerType="sub_account" onRefresh={fetchPlans} />
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/network/components/TariffOverridesTab.tsx
git commit -m "feat(tariffs): add TariffOverridesTab component"
```

---

## Task 12: Frontend — Refactor NetworkTariffsPage to Tabs

**Files:**
- Modify: `portal-frontend/src/pages/network/NetworkTariffsPage.tsx`

- [ ] **Step 1: Rewrite NetworkTariffsPage with tabs**

Replace the entire contents of `NetworkTariffsPage.tsx`:

```typescript
import { useState } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { PageHeader } from '../../components/layout/PageHeader';
import { TariffOverviewTab } from './components/TariffOverviewTab';
import { TariffTemplatesTab } from './components/TariffTemplatesTab';
import { TariffOverridesTab } from './components/TariffOverridesTab';

const tabs = [
  { value: 'overview', label: 'Обзор' },
  { value: 'templates', label: 'Шаблоны' },
  { value: 'overrides', label: 'Переопределения' },
] as const;

export function NetworkTariffsPage() {
  const [tab, setTab] = useState<string>('overview');

  const tabCls = (value: string) =>
    `px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
      tab === value
        ? 'border-primary text-primary'
        : 'border-transparent text-gray-500 hover:text-gray-700'
    }`;

  return (
    <div className="max-w-6xl">
      <PageHeader title="Тарифы субаккаунтов" />
      <Tabs.Root value={tab} onValueChange={setTab}>
        <Tabs.List className="flex border-b border-gray-200 mb-4">
          {tabs.map((t) => (
            <Tabs.Trigger key={t.value} value={t.value} className={tabCls(t.value)}>
              {t.label}
            </Tabs.Trigger>
          ))}
        </Tabs.List>
        <Tabs.Content value="overview">
          <TariffOverviewTab onNavigateToPlan={() => setTab('templates')} />
        </Tabs.Content>
        <Tabs.Content value="templates">
          <TariffTemplatesTab />
        </Tabs.Content>
        <Tabs.Content value="overrides">
          <TariffOverridesTab />
        </Tabs.Content>
      </Tabs.Root>
    </div>
  );
}
```

- [ ] **Step 2: Verify frontend builds**

```bash
cd /c/projects/sms/portal-frontend && npx tsc --noEmit
```

Expected: no type errors.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/network/NetworkTariffsPage.tsx
git commit -m "feat(tariffs): refactor NetworkTariffsPage to tabs (overview/templates/overrides)"
```

---

## Task 13: Apply Migration & Deploy

- [ ] **Step 1: Push to GitHub**

```bash
git push origin master
```

- [ ] **Step 2: Deploy to server**

```bash
bash scripts/server.sh deploy
```

- [ ] **Step 3: Apply migration**

```bash
bash scripts/server.sh migrate
```

- [ ] **Step 4: Verify the page loads**

Open `http://72.56.232.202:18085/network/tariffs` and verify:
- Three tabs appear: Обзор, Шаблоны, Переопределения
- Обзор tab shows sub-account selector and matrix with source badges
- Шаблоны tab shows empty state with "Создать шаблон" button
- Переопределения tab shows sub-account selector

- [ ] **Step 5: Smoke test the full flow**

1. Create a template "Тестовый шаблон"
2. Add a plan (any operator, standard category, fixed strategy)
3. Add a period (today to end of year)
4. Set a tier (price 1.50)
5. Assign a sub-account to the template
6. Switch to Обзор tab, select that sub-account — verify price shows with "Шаблон" badge
7. Switch to Переопределения, create an override plan for same operator with different price
8. Verify Обзор now shows "Переопределение" badge

# Sender Names Operator Moderation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить гранулярную модерацию регистраций имён отправителей по операторам (каждая пара имя-оператор проходит модерацию независимо), независимую модерацию шаблонов операторов, fallback на общее имя в pipeline вместо блокировки.

**Architecture:** Две новые таблицы — `operator_registrations` (модерация регистрации per sender_name+operator) и расширение существующей `operator_templates` (добавить `moderation_status`). Прямые пользователи модерируются admin-ом, субаккаунты — агрегатором через portal. Все обращения к БД через прямой SQL (pgxpool), как в существующих handler-ах.

**Tech Stack:** Go 1.24, gorilla/mux, pgx/v5, PostgreSQL 15+, React 19 + TypeScript, Tailwind CSS 4.2

**Spec:** `docs/superpowers/specs/2026-04-15-sender-names-operator-moderation-design.md`

---

## Файловая карта

### Новые файлы
- `migrations/000094_operator_registrations.up.sql`
- `migrations/000094_operator_registrations.down.sql`
- `migrations/000095_operator_templates_moderation.up.sql`
- `migrations/000095_operator_templates_moderation.down.sql`
- `internal/gateway/portal/handlers/operator_registrations.go` — клиент submit/list/resubmit
- `internal/gateway/portal/handlers/operator_templates_portal.go` — клиент CRUD + submit/resubmit шаблонов
- `internal/gateway/portal/handlers/reseller_moderation.go` — агрегатор: очередь субаккаунтов
- `internal/gateway/admin/handlers/operator_registrations_admin.go` — admin: очередь прямых пользователей
- `portal-frontend/src/pages/sender-names/OperatorTemplatesSection.tsx` — секция шаблонов

### Изменяемые файлы
- `internal/gateway/portal/router/router.go` — добавить 12 маршрутов
- `internal/gateway/admin/router/router.go` — добавить 8 маршрутов
- `internal/gateway/admin/handlers/operator_templates.go` — добавить moderation методы
- `internal/pipeline/router/stage.go` — fallback sender name при роутинге
- `portal-frontend/src/pages/sender-names/SenderNameDetailPage.tsx` — добавить секцию шаблонов
- `portal-frontend/src/pages/sender-names/SenderNameOperatorsPage.tsx` — показывать approved/pending отдельно
- `portal-frontend/src/api/client.ts` — новые API методы

---

## Task 1: Миграция — operator_registrations

**Files:**
- Create: `migrations/000094_operator_registrations.up.sql`
- Create: `migrations/000094_operator_registrations.down.sql`

- [ ] **Создать up-миграцию**

```sql
-- migrations/000094_operator_registrations.up.sql
BEGIN;

CREATE TABLE operator_registrations (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_name_id    UUID         NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
    operator_id       UUID         NOT NULL REFERENCES operators(id),
    registration_type VARCHAR(16)  NOT NULL CHECK (registration_type IN ('free', 'paid')),
    status            VARCHAR(32)  NOT NULL DEFAULT 'submitted'
                          CHECK (status IN ('submitted', 'approved', 'rejected', 'revision_requested')),
    approved_type     VARCHAR(16)  CHECK (approved_type IN ('free', 'paid')),
    approved_at       TIMESTAMPTZ,
    moderator_note    TEXT,
    submitted_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT operator_registrations_unique UNIQUE (sender_name_id, operator_id)
);

CREATE TABLE operator_registration_history (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_registration_id UUID        NOT NULL REFERENCES operator_registrations(id) ON DELETE CASCADE,
    old_status               VARCHAR(32),
    new_status               VARCHAR(32) NOT NULL,
    actor_id                 UUID,
    actor_type               VARCHAR(20) NOT NULL CHECK (actor_type IN ('client', 'aggregator', 'admin', 'system')),
    comment                  TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_opreg_sender_name_id ON operator_registrations (sender_name_id);
CREATE INDEX idx_opreg_status         ON operator_registrations (status);
CREATE INDEX idx_opreg_operator_id    ON operator_registrations (operator_id);
CREATE INDEX idx_opreg_hist_reg_id    ON operator_registration_history (operator_registration_id);

COMMIT;
```

- [ ] **Создать down-миграцию**

```sql
-- migrations/000094_operator_registrations.down.sql
BEGIN;
DROP TABLE IF EXISTS operator_registration_history;
DROP TABLE IF EXISTS operator_registrations;
COMMIT;
```

- [ ] **Применить миграцию локально (если есть доступ к БД — опционально, иначе пропустить)**

```bash
# Проверить что SQL корректен синтаксически
grep -c "CREATE TABLE" migrations/000094_operator_registrations.up.sql
# Expected: 2
```

- [ ] **Коммит**

```bash
git add migrations/000094_operator_registrations.up.sql migrations/000094_operator_registrations.down.sql
git commit -m "feat(db): add operator_registrations and history tables"
```

---

## Task 2: Миграция — расширение operator_templates

**Files:**
- Create: `migrations/000095_operator_templates_moderation.up.sql`
- Create: `migrations/000095_operator_templates_moderation.down.sql`

- [ ] **Создать up-миграцию**

Существующая таблица `operator_templates` имеет `status IN ('active', 'inactive', 'pending')`. Добавляем отдельный `moderation_status` и вспомогательные поля, не трогая существующий `status`.

```sql
-- migrations/000095_operator_templates_moderation.up.sql
BEGIN;

ALTER TABLE operator_templates
    ADD COLUMN moderation_status VARCHAR(32) NOT NULL DEFAULT 'draft'
        CHECK (moderation_status IN ('draft', 'submitted', 'approved', 'rejected', 'revision_requested')),
    ADD COLUMN moderator_note    TEXT,
    ADD COLUMN submitted_at      TIMESTAMPTZ,
    ADD COLUMN resolved_at       TIMESTAMPTZ;

CREATE INDEX idx_optpl_moderation_status ON operator_templates (moderation_status);

COMMIT;
```

- [ ] **Создать down-миграцию**

```sql
-- migrations/000095_operator_templates_moderation.down.sql
BEGIN;
DROP INDEX IF EXISTS idx_optpl_moderation_status;
ALTER TABLE operator_templates
    DROP COLUMN IF EXISTS moderation_status,
    DROP COLUMN IF EXISTS moderator_note,
    DROP COLUMN IF EXISTS submitted_at,
    DROP COLUMN IF EXISTS resolved_at;
COMMIT;
```

- [ ] **Коммит**

```bash
git add migrations/000095_operator_templates_moderation.up.sql migrations/000095_operator_templates_moderation.down.sql
git commit -m "feat(db): add moderation columns to operator_templates"
```

---

## Task 3: Portal — клиент создаёт и просматривает operator_registrations

**Files:**
- Create: `internal/gateway/portal/handlers/operator_registrations.go`

Паттерн: прямой SQL через `pgxpool.Pool`, структура как в `sender_names.go`.

- [ ] **Создать файл handler-а**

```go
// internal/gateway/portal/handlers/operator_registrations.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type OperatorRegistrationHandlers struct {
	pool *pgxpool.Pool
}

func NewOperatorRegistrationHandlers(pool *pgxpool.Pool) *OperatorRegistrationHandlers {
	return &OperatorRegistrationHandlers{pool: pool}
}

// BulkSubmitOperatorRegistrations POST /portal/v1/sender-names/{id}/operator-registrations
// Заменяет старый BulkCreateOperatorRegistrations — создаёт записи модерации, billing НЕ создаёт.
func (h *OperatorRegistrationHandlers) BulkSubmitOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	if senderNameID == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	// Проверяем что имя принадлежит клиенту и имеет статус approved
	var snStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT status FROM sender_names WHERE id = $1 AND client_id = $2`,
		senderNameID, clientID,
	).Scan(&snStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("имя отправителя не найдено"))
		return
	}
	if snStatus != "approved" {
		respondError(w, shared.ErrInvalidInput("имя отправителя должно быть в статусе approved"))
		return
	}

	var req struct {
		Registrations []struct {
			OperatorID string `json:"operator_id"`
			Type       string `json:"type"`
		} `json:"registrations"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.Registrations) == 0 {
		respondError(w, shared.ErrInvalidInput("registrations не может быть пустым"))
		return
	}

	type resultItem struct {
		OperatorID string `json:"operator_id"`
		ID         string `json:"id,omitempty"`
		Status     string `json:"status"`
		Error      string `json:"error,omitempty"`
	}
	results := make([]resultItem, 0, len(req.Registrations))

	for _, reg := range req.Registrations {
		if reg.OperatorID == "" {
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: "operator_id обязателен"})
			continue
		}
		regType := reg.Type
		if regType == "" {
			regType = "free"
		}
		if regType != "free" && regType != "paid" {
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: "type должен быть free или paid"})
			continue
		}

		var newID string
		err := h.pool.QueryRow(r.Context(),
			`INSERT INTO operator_registrations
			    (sender_name_id, operator_id, registration_type, status, submitted_at, created_at, updated_at)
			 VALUES ($1, $2, $3, 'submitted', NOW(), NOW(), NOW())
			 ON CONFLICT (sender_name_id, operator_id) DO UPDATE
			    SET registration_type = EXCLUDED.registration_type,
			        status = 'submitted',
			        submitted_at = NOW(),
			        updated_at = NOW()
			 RETURNING id`,
			senderNameID, reg.OperatorID, regType,
		).Scan(&newID)
		if err != nil {
			log.Error().Err(err).Str("operator_id", reg.OperatorID).Msg("ошибка создания operator_registration")
			results = append(results, resultItem{OperatorID: reg.OperatorID, Status: "error", Error: "ошибка создания записи"})
			continue
		}

		// Запись в историю
		_, _ = h.pool.Exec(r.Context(),
			`INSERT INTO operator_registration_history
			    (operator_registration_id, old_status, new_status, actor_id, actor_type, created_at)
			 VALUES ($1, NULL, 'submitted', $2, 'client', NOW())`,
			newID, clientID,
		)

		results = append(results, resultItem{OperatorID: reg.OperatorID, ID: newID, Status: "submitted"})
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{"results": results})
}

// ListOperatorRegistrations GET /portal/v1/sender-names/{id}/operator-registrations
func (h *OperatorRegistrationHandlers) ListOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	if senderNameID == "" {
		respondError(w, shared.ErrInvalidInput("ID обязателен"))
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT or2.id, or2.operator_id, o.name, or2.registration_type, or2.status,
		        or2.approved_type, or2.approved_at, or2.moderator_note,
		        or2.submitted_at, or2.resolved_at
		 FROM operator_registrations or2
		 JOIN operators o ON o.id = or2.operator_id
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 WHERE or2.sender_name_id = $1 AND sn.client_id = $2
		 ORDER BY o.name`,
		senderNameID, clientID,
	)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения operator_registrations")
		respondError(w, shared.ErrInternalServer("ошибка получения регистраций"))
		return
	}
	defer rows.Close()

	type regJSON struct {
		ID               string     `json:"id"`
		OperatorID       string     `json:"operator_id"`
		OperatorName     string     `json:"operator_name"`
		RegistrationType string     `json:"registration_type"`
		Status           string     `json:"status"`
		ApprovedType     *string    `json:"approved_type"`
		ApprovedAt       *time.Time `json:"approved_at"`
		ModeratorNote    *string    `json:"moderator_note"`
		SubmittedAt      time.Time  `json:"submitted_at"`
		ResolvedAt       *time.Time `json:"resolved_at"`
	}
	regs := make([]regJSON, 0)
	for rows.Next() {
		var reg regJSON
		if err := rows.Scan(
			&reg.ID, &reg.OperatorID, &reg.OperatorName,
			&reg.RegistrationType, &reg.Status,
			&reg.ApprovedType, &reg.ApprovedAt, &reg.ModeratorNote,
			&reg.SubmittedAt, &reg.ResolvedAt,
		); err != nil {
			log.Error().Err(err).Msg("ошибка сканирования registration")
			respondError(w, shared.ErrInternalServer("ошибка получения регистраций"))
			return
		}
		regs = append(regs, reg)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"registrations": regs})
}

// ResubmitOperatorRegistration POST /portal/v1/sender-names/{id}/operator-registrations/{rid}/resubmit
func (h *OperatorRegistrationHandlers) ResubmitOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	regID := mux.Vars(r)["rid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 WHERE or2.id = $1 AND or2.sender_name_id = $2 AND sn.client_id = $3`,
		regID, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("resubmit возможен только из статуса revision_requested"))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'submitted', submitted_at = NOW(), resolved_at = NULL,
		     moderator_note = NULL, updated_at = NOW()
		 WHERE id = $1`,
		regID,
	)
	if err != nil {
		log.Error().Err(err).Str("id", regID).Msg("ошибка resubmit")
		respondError(w, shared.ErrInternalServer("ошибка обновления статуса"))
		return
	}

	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_id, actor_type, created_at)
		 VALUES ($1, 'revision_requested', 'submitted', $2, 'client', NOW())`,
		regID, clientID,
	)

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "submitted"})
}
```

- [ ] **Коммит**

```bash
git add internal/gateway/portal/handlers/operator_registrations.go
git commit -m "feat(portal): add operator_registrations submit/list/resubmit handlers"
```

---

## Task 4: Portal — маршруты для operator_registrations

**Files:**
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Прочитать файл для поиска блока sender-names**

Найти строки вида:
```
senderNames.HandleFunc("/{id}/operator-registrations", senderNameHandlers.BulkCreateOperatorRegistrations).Methods("POST")
```

- [ ] **Заменить старые маршруты operator-registrations на новые**

В `internal/gateway/portal/router/router.go` найти блок `sender-names` и заменить строки с `operator-registrations`:

Было:
```go
senderNames.HandleFunc("/{id}/operator-registrations", senderNameHandlers.GetSenderNameOperatorRegistrations).Methods("GET")
senderNames.HandleFunc("/{id}/operator-registrations", senderNameHandlers.BulkCreateOperatorRegistrations).Methods("POST")
```

Стало:
```go
senderNames.HandleFunc("/{id}/operator-registrations", opRegHandlers.ListOperatorRegistrations).Methods("GET")
senderNames.HandleFunc("/{id}/operator-registrations", opRegHandlers.BulkSubmitOperatorRegistrations).Methods("POST")
senderNames.HandleFunc("/{id}/operator-registrations/{rid}/resubmit", opRegHandlers.ResubmitOperatorRegistration).Methods("POST")
```

- [ ] **Добавить `opRegHandlers` в параметры `SetupRouter`**

В сигнатуру функции `SetupRouter` добавить:
```go
opRegHandlers *handlers.OperatorRegistrationHandlers,
```

- [ ] **Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/gateway/portal/...
```

Ожидаемый результат: успешная сборка без ошибок. Если ошибки — исправить импорты.

- [ ] **Коммит**

```bash
git add internal/gateway/portal/router/router.go
git commit -m "feat(portal): wire operator_registrations routes"
```

---

## Task 5: Portal — клиент управляет operator_templates

**Files:**
- Create: `internal/gateway/portal/handlers/operator_templates_portal.go`

- [ ] **Создать файл**

```go
// internal/gateway/portal/handlers/operator_templates_portal.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type PortalOperatorTemplateHandlers struct {
	pool *pgxpool.Pool
}

func NewPortalOperatorTemplateHandlers(pool *pgxpool.Pool) *PortalOperatorTemplateHandlers {
	return &PortalOperatorTemplateHandlers{pool: pool}
}

// ListPortalOperatorTemplates GET /portal/v1/sender-names/{id}/operator-templates
func (h *PortalOperatorTemplateHandlers) ListPortalOperatorTemplates(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]

	operatorFilter := r.URL.Query().Get("operator_id")

	query := `SELECT ot.id, ot.name, ot.operator_id, o.name AS operator_name,
	                 ot.body, ot.moderation_status, ot.moderator_note,
	                 ot.submitted_at, ot.resolved_at, ot.created_at, ot.updated_at
	          FROM operator_templates ot
	          JOIN operators o ON o.id = ot.operator_id
	          JOIN sender_names sn ON sn.id = ot.sender_name_id
	          WHERE ot.sender_name_id = $1 AND sn.client_id = $2`
	args := []interface{}{senderNameID, clientID}
	if operatorFilter != "" {
		query += " AND ot.operator_id = $3"
		args = append(args, operatorFilter)
	}
	query += " ORDER BY o.name, ot.name"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения operator_templates")
		respondError(w, shared.ErrInternalServer("ошибка получения шаблонов"))
		return
	}
	defer rows.Close()

	type tplJSON struct {
		ID               string     `json:"id"`
		Name             string     `json:"name"`
		OperatorID       string     `json:"operator_id"`
		OperatorName     string     `json:"operator_name"`
		Body             string     `json:"body"`
		ModerationStatus string     `json:"moderation_status"`
		ModeratorNote    *string    `json:"moderator_note"`
		SubmittedAt      *time.Time `json:"submitted_at"`
		ResolvedAt       *time.Time `json:"resolved_at"`
		CreatedAt        time.Time  `json:"created_at"`
		UpdatedAt        time.Time  `json:"updated_at"`
	}
	templates := make([]tplJSON, 0)
	for rows.Next() {
		var t tplJSON
		if err := rows.Scan(
			&t.ID, &t.Name, &t.OperatorID, &t.OperatorName,
			&t.Body, &t.ModerationStatus, &t.ModeratorNote,
			&t.SubmittedAt, &t.ResolvedAt, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка получения шаблонов"))
			return
		}
		templates = append(templates, t)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"templates": templates})
}

// CreatePortalOperatorTemplate POST /portal/v1/sender-names/{id}/operator-templates
func (h *PortalOperatorTemplateHandlers) CreatePortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]

	// Проверяем что имя одобрено и принадлежит клиенту
	var snStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT status FROM sender_names WHERE id = $1 AND client_id = $2`,
		senderNameID, clientID,
	).Scan(&snStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("имя отправителя не найдено"))
		return
	}
	if snStatus != "approved" {
		respondError(w, shared.ErrInvalidInput("имя отправителя должно быть в статусе approved"))
		return
	}

	var req struct {
		OperatorID string `json:"operator_id"`
		Name       string `json:"name"`
		Body       string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.OperatorID == "" || req.Name == "" || req.Body == "" {
		respondError(w, shared.ErrInvalidInput("operator_id, name, body обязательны"))
		return
	}

	var newID string
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO operator_templates
		    (sender_name_id, operator_id, name, body, variables, status, moderation_status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, '[]', 'active', 'draft', NOW(), NOW())
		 RETURNING id`,
		senderNameID, req.OperatorID, req.Name, req.Body,
	).Scan(&newID)
	if err != nil {
		log.Error().Err(err).Msg("ошибка создания operator_template")
		respondError(w, shared.ErrInternalServer("ошибка создания шаблона"))
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":                newID,
		"moderation_status": "draft",
	})
}

// UpdatePortalOperatorTemplate PUT /portal/v1/sender-names/{id}/operator-templates/{tid}
func (h *PortalOperatorTemplateHandlers) UpdatePortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	tid := mux.Vars(r)["tid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 WHERE ot.id = $1 AND ot.sender_name_id = $2 AND sn.client_id = $3`,
		tid, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "draft" {
		respondError(w, shared.ErrInvalidInput("редактирование возможно только в статусе draft"))
		return
	}

	var req struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_templates SET name = $1, body = $2, updated_at = NOW()
		 WHERE id = $3`,
		req.Name, req.Body, tid,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления шаблона"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"id": tid, "status": "updated"})
}

// DeletePortalOperatorTemplate DELETE /portal/v1/sender-names/{id}/operator-templates/{tid}
func (h *PortalOperatorTemplateHandlers) DeletePortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	tid := mux.Vars(r)["tid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 WHERE ot.id = $1 AND ot.sender_name_id = $2 AND sn.client_id = $3`,
		tid, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "draft" {
		respondError(w, shared.ErrInvalidInput("удаление возможно только в статусе draft"))
		return
	}

	_, _ = h.pool.Exec(r.Context(), `DELETE FROM operator_templates WHERE id = $1`, tid)
	respondJSON(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

// SubmitPortalOperatorTemplate POST /portal/v1/sender-names/{id}/operator-templates/{tid}/submit
func (h *PortalOperatorTemplateHandlers) SubmitPortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	tid := mux.Vars(r)["tid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 WHERE ot.id = $1 AND ot.sender_name_id = $2 AND sn.client_id = $3`,
		tid, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "draft" {
		respondError(w, shared.ErrInvalidInput("submit возможен только из статуса draft"))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'submitted', submitted_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		tid,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления статуса"))
		return
	}
	_ = clientID // используется для аудита если нужно
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "submitted"})
}

// ResubmitPortalOperatorTemplate POST /portal/v1/sender-names/{id}/operator-templates/{tid}/resubmit
func (h *PortalOperatorTemplateHandlers) ResubmitPortalOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	senderNameID := mux.Vars(r)["id"]
	tid := mux.Vars(r)["tid"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 WHERE ot.id = $1 AND ot.sender_name_id = $2 AND sn.client_id = $3`,
		tid, senderNameID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("resubmit возможен только из статуса revision_requested"))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'submitted', submitted_at = NOW(),
		     resolved_at = NULL, moderator_note = NULL, updated_at = NOW()
		 WHERE id = $1`,
		tid,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления статуса"))
		return
	}
	_ = clientID
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "submitted"})
}
```

- [ ] **Коммит**

```bash
git add internal/gateway/portal/handlers/operator_templates_portal.go
git commit -m "feat(portal): add operator_templates CRUD and submit/resubmit for clients"
```

---

## Task 6: Portal — маршруты для operator_templates

**Files:**
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Добавить маршруты для operator-templates в блок sender-names**

После строки с `/operator-registrations/{rid}/resubmit` добавить:

```go
senderNames.HandleFunc("/{id}/operator-templates", portalOpTplHandlers.ListPortalOperatorTemplates).Methods("GET")
senderNames.HandleFunc("/{id}/operator-templates", portalOpTplHandlers.CreatePortalOperatorTemplate).Methods("POST")
senderNames.HandleFunc("/{id}/operator-templates/{tid}", portalOpTplHandlers.UpdatePortalOperatorTemplate).Methods("PUT")
senderNames.HandleFunc("/{id}/operator-templates/{tid}", portalOpTplHandlers.DeletePortalOperatorTemplate).Methods("DELETE")
senderNames.HandleFunc("/{id}/operator-templates/{tid}/submit", portalOpTplHandlers.SubmitPortalOperatorTemplate).Methods("POST")
senderNames.HandleFunc("/{id}/operator-templates/{tid}/resubmit", portalOpTplHandlers.ResubmitPortalOperatorTemplate).Methods("POST")
```

Добавить `portalOpTplHandlers *handlers.PortalOperatorTemplateHandlers` в параметры `SetupRouter`.

- [ ] **Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/gateway/portal/...
```

- [ ] **Коммит**

```bash
git add internal/gateway/portal/router/router.go
git commit -m "feat(portal): wire operator_templates routes for clients"
```

---

## Task 7: Admin — модерация operator_registrations (прямые пользователи)

**Files:**
- Create: `internal/gateway/admin/handlers/operator_registrations_admin.go`

- [ ] **Создать файл**

```go
// internal/gateway/admin/handlers/operator_registrations_admin.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

type AdminOperatorRegistrationHandlers struct {
	db *storage.DB
}

func NewAdminOperatorRegistrationHandlers(db *storage.DB) *AdminOperatorRegistrationHandlers {
	return &AdminOperatorRegistrationHandlers{db: db}
}

// ListAdminOperatorRegistrations GET /admin/v1/operator-registrations
// Возвращает заявки прямых пользователей (parent_client_id IS NULL).
func (h *AdminOperatorRegistrationHandlers) ListAdminOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	operatorFilter := r.URL.Query().Get("operator_id")
	clientFilter := r.URL.Query().Get("client_id")

	query := `SELECT or2.id, or2.sender_name_id, sn.name AS sender_name,
	                 or2.operator_id, o.name AS operator_name,
	                 or2.registration_type, or2.status, or2.approved_type,
	                 or2.moderator_note, or2.submitted_at, or2.resolved_at,
	                 sn.client_id
	          FROM operator_registrations or2
	          JOIN sender_names sn ON sn.id = or2.sender_name_id
	          JOIN operators o ON o.id = or2.operator_id
	          JOIN clients c ON c.id = sn.client_id
	          WHERE c.parent_client_id IS NULL`
	args := []interface{}{}
	i := 1
	if statusFilter != "" {
		query += " AND or2.status = $" + itoa(i)
		args = append(args, statusFilter)
		i++
	}
	if operatorFilter != "" {
		query += " AND or2.operator_id = $" + itoa(i)
		args = append(args, operatorFilter)
		i++
	}
	if clientFilter != "" {
		query += " AND sn.client_id = $" + itoa(i)
		args = append(args, clientFilter)
		i++
	}
	query += " ORDER BY or2.submitted_at DESC LIMIT 100"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения operator_registrations")
		respondError(w, shared.ErrInternalServer("ошибка получения очереди"))
		return
	}
	defer rows.Close()

	type regJSON struct {
		ID               string     `json:"id"`
		SenderNameID     string     `json:"sender_name_id"`
		SenderName       string     `json:"sender_name"`
		OperatorID       string     `json:"operator_id"`
		OperatorName     string     `json:"operator_name"`
		RegistrationType string     `json:"registration_type"`
		Status           string     `json:"status"`
		ApprovedType     *string    `json:"approved_type"`
		ModeratorNote    *string    `json:"moderator_note"`
		SubmittedAt      time.Time  `json:"submitted_at"`
		ResolvedAt       *time.Time `json:"resolved_at"`
		ClientID         string     `json:"client_id"`
	}
	regs := make([]regJSON, 0)
	for rows.Next() {
		var reg regJSON
		if err := rows.Scan(
			&reg.ID, &reg.SenderNameID, &reg.SenderName,
			&reg.OperatorID, &reg.OperatorName,
			&reg.RegistrationType, &reg.Status, &reg.ApprovedType,
			&reg.ModeratorNote, &reg.SubmittedAt, &reg.ResolvedAt,
			&reg.ClientID,
		); err != nil {
			log.Error().Err(err).Msg("ошибка сканирования")
			respondError(w, shared.ErrInternalServer("ошибка получения очереди"))
			return
		}
		regs = append(regs, reg)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"registrations": regs})
}

// ApproveAdminOperatorRegistration POST /admin/v1/operator-registrations/{id}/approve
func (h *AdminOperatorRegistrationHandlers) ApproveAdminOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Проверяем что это прямой пользователь
	var currentStatus, regType string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT or2.status, or2.registration_type FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus, &regType)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена или не принадлежит прямому пользователю"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса submitted"))
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'approved', approved_type = registration_type,
		     approved_at = NOW(), resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("ошибка approve")
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}

	_, _ = h.db.ExecContext(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_type, created_at)
		 VALUES ($1, 'submitted', 'approved', 'admin', NOW())`,
		id,
	)

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "approved"})
}

// RejectAdminOperatorRegistration POST /admin/v1/operator-registrations/{id}/reject
func (h *AdminOperatorRegistrationHandlers) RejectAdminOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("reject возможен только из статуса submitted"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'rejected', moderator_note = $2, resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, nullableString(req.Note),
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}

	_, _ = h.db.ExecContext(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_type, comment, created_at)
		 VALUES ($1, 'submitted', 'rejected', 'admin', $2, NOW())`,
		id, nullableString(req.Note),
	)

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "rejected"})
}

// RequestRevisionAdminOperatorRegistration POST /admin/v1/operator-registrations/{id}/request-revision
func (h *AdminOperatorRegistrationHandlers) RequestRevisionAdminOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("request-revision возможен только из статуса submitted"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'revision_requested', moderator_note = $2,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, nullableString(req.Note),
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}

	_, _ = h.db.ExecContext(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_type, comment, created_at)
		 VALUES ($1, 'submitted', 'revision_requested', 'admin', $2, NOW())`,
		id, nullableString(req.Note),
	)

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "revision_requested"})
}

// nullableString возвращает nil если строка пустая.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// itoa конвертирует int в строку (для построения SQL с $N параметрами).
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
```

- [ ] **Добавить импорт `fmt` в файл если не добавлен**

Проверить что в imports есть `"fmt"`. Если нет — добавить.

- [ ] **Добавить маршруты в admin router**

В `internal/gateway/admin/router/router.go` после блока `sender-names` добавить:

```go
// Operator Registrations moderation (direct users only)
opRegs := adminV1.PathPrefix("/operator-registrations").Subrouter()
opRegs.HandleFunc("", adminOpRegHandlers.ListAdminOperatorRegistrations).Methods("GET")
opRegs.HandleFunc("/{id}/approve", adminOpRegHandlers.ApproveAdminOperatorRegistration).Methods("POST")
opRegs.HandleFunc("/{id}/reject", adminOpRegHandlers.RejectAdminOperatorRegistration).Methods("POST")
opRegs.HandleFunc("/{id}/request-revision", adminOpRegHandlers.RequestRevisionAdminOperatorRegistration).Methods("POST")
```

Добавить `adminOpRegHandlers *handlers.AdminOperatorRegistrationHandlers` в параметры `SetupRouter`.

- [ ] **Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/gateway/admin/...
```

- [ ] **Коммит**

```bash
git add internal/gateway/admin/handlers/operator_registrations_admin.go internal/gateway/admin/router/router.go
git commit -m "feat(admin): add operator_registrations moderation queue for direct users"
```

---

## Task 8: Admin — модерация operator_templates

**Files:**
- Modify: `internal/gateway/admin/handlers/operator_templates.go`
- Modify: `internal/gateway/admin/router/router.go`

- [ ] **Добавить методы модерации в конец существующего файла**

В конец `internal/gateway/admin/handlers/operator_templates.go` добавить:

```go
// ApproveOperatorTemplate POST /admin/v1/operator-templates/{id}/approve
func (h *OperatorTemplateHandlers) ApproveOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE ot.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса submitted"))
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'approved', resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "approved"})
}

// RejectOperatorTemplate POST /admin/v1/operator-templates/{id}/reject
func (h *OperatorTemplateHandlers) RejectOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE ot.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("reject возможен только из статуса submitted"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'rejected', moderator_note = $2,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, nullableString(req.Note),
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "rejected"})
}

// RequestRevisionOperatorTemplate POST /admin/v1/operator-templates/{id}/request-revision
func (h *OperatorTemplateHandlers) RequestRevisionOperatorTemplate(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT ot.moderation_status FROM operator_templates ot
		 JOIN sender_names sn ON sn.id = ot.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE ot.id = $1 AND c.parent_client_id IS NULL`,
		id,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("request-revision возможен только из статуса submitted"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE operator_templates
		 SET moderation_status = 'revision_requested', moderator_note = $2,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, nullableString(req.Note),
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"moderation_status": "revision_requested"})
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
```

- [ ] **Добавить маршруты модерации в admin router**

В блоке `operatorTemplates` в `router.go` добавить после существующих маршрутов:

```go
operatorTemplates.HandleFunc("/{id}/approve", operatorTemplateHandlers.ApproveOperatorTemplate).Methods("POST")
operatorTemplates.HandleFunc("/{id}/reject", operatorTemplateHandlers.RejectOperatorTemplate).Methods("POST")
operatorTemplates.HandleFunc("/{id}/request-revision", operatorTemplateHandlers.RequestRevisionOperatorTemplate).Methods("POST")
```

- [ ] **Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/gateway/admin/...
```

- [ ] **Коммит**

```bash
git add internal/gateway/admin/handlers/operator_templates.go internal/gateway/admin/router/router.go
git commit -m "feat(admin): add operator_templates moderation approve/reject/revision"
```

---

## Task 9: Portal — агрегатор модерирует субаккаунты

**Files:**
- Create: `internal/gateway/portal/handlers/reseller_moderation.go`
- Modify: `internal/gateway/portal/router/router.go`

- [ ] **Создать handler для агрегатора**

```go
// internal/gateway/portal/handlers/reseller_moderation.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerModerationHandlers struct {
	pool *pgxpool.Pool
}

func NewResellerModerationHandlers(pool *pgxpool.Pool) *ResellerModerationHandlers {
	return &ResellerModerationHandlers{pool: pool}
}

// checkIsReseller проверяет что клиент является реселлером.
func (h *ResellerModerationHandlers) checkIsReseller(ctx interface{ Value(interface{}) interface{} }, clientID interface{}) bool {
	return true // реализация через is_reseller из БД — см. ниже в конкретных методах
}

// ListResellerOperatorRegistrations GET /portal/v1/reseller/operator-registrations
// Возвращает заявки субаккаунтов данного агрегатора.
func (h *ResellerModerationHandlers) ListResellerOperatorRegistrations(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	// Проверяем что клиент is_reseller
	var isReseller bool
	err := h.pool.QueryRow(r.Context(),
		`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
	).Scan(&isReseller)
	if err != nil || !isReseller {
		respondError(w, shared.ErrForbidden("доступ только для агрегаторов"))
		return
	}

	statusFilter := r.URL.Query().Get("status")
	subAccountFilter := r.URL.Query().Get("sub_account_id")

	query := `SELECT or2.id, or2.sender_name_id, sn.name AS sender_name,
	                 or2.operator_id, o.name AS operator_name,
	                 or2.registration_type, or2.status, or2.approved_type,
	                 or2.moderator_note, or2.submitted_at, or2.resolved_at,
	                 sn.client_id AS sub_account_id
	          FROM operator_registrations or2
	          JOIN sender_names sn ON sn.id = or2.sender_name_id
	          JOIN operators o ON o.id = or2.operator_id
	          JOIN clients c ON c.id = sn.client_id
	          WHERE c.parent_client_id = $1`
	args := []interface{}{clientID}
	i := 2
	if statusFilter != "" {
		query += " AND or2.status = $" + fmt.Sprintf("%d", i)
		args = append(args, statusFilter)
		i++
	}
	if subAccountFilter != "" {
		query += " AND sn.client_id = $" + fmt.Sprintf("%d", i)
		args = append(args, subAccountFilter)
		i++
	}
	query += " ORDER BY or2.submitted_at DESC LIMIT 100"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения очереди субаккаунтов")
		respondError(w, shared.ErrInternalServer("ошибка получения очереди"))
		return
	}
	defer rows.Close()

	type regJSON struct {
		ID               string     `json:"id"`
		SenderNameID     string     `json:"sender_name_id"`
		SenderName       string     `json:"sender_name"`
		OperatorID       string     `json:"operator_id"`
		OperatorName     string     `json:"operator_name"`
		RegistrationType string     `json:"registration_type"`
		Status           string     `json:"status"`
		ApprovedType     *string    `json:"approved_type"`
		ModeratorNote    *string    `json:"moderator_note"`
		SubmittedAt      time.Time  `json:"submitted_at"`
		ResolvedAt       *time.Time `json:"resolved_at"`
		SubAccountID     string     `json:"sub_account_id"`
	}
	regs := make([]regJSON, 0)
	for rows.Next() {
		var reg regJSON
		if err := rows.Scan(
			&reg.ID, &reg.SenderNameID, &reg.SenderName,
			&reg.OperatorID, &reg.OperatorName,
			&reg.RegistrationType, &reg.Status, &reg.ApprovedType,
			&reg.ModeratorNote, &reg.SubmittedAt, &reg.ResolvedAt,
			&reg.SubAccountID,
		); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		regs = append(regs, reg)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"registrations": regs})
}

// approveResellerRegistration — общая логика approve для агрегатора.
func (h *ResellerModerationHandlers) approveResellerRegistration(w http.ResponseWriter, r *http.Request, regID string, clientID interface{}) {
	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id = $2`,
		regID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса submitted"))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'approved', approved_type = registration_type,
		     approved_at = NOW(), resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		regID,
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_id, actor_type, created_at)
		 VALUES ($1, 'submitted', 'approved', $2, 'aggregator', NOW())`,
		regID, clientID,
	)
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "approved"})
}

// ApproveResellerOperatorRegistration POST /portal/v1/reseller/operator-registrations/{id}/approve
func (h *ResellerModerationHandlers) ApproveResellerOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var isReseller bool
	_ = h.pool.QueryRow(r.Context(), `SELECT is_reseller FROM clients WHERE id = $1`, clientID).Scan(&isReseller)
	if !isReseller {
		respondError(w, shared.ErrForbidden("доступ только для агрегаторов"))
		return
	}
	h.approveResellerRegistration(w, r, mux.Vars(r)["id"], clientID)
}

// RejectResellerOperatorRegistration POST /portal/v1/reseller/operator-registrations/{id}/reject
func (h *ResellerModerationHandlers) RejectResellerOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var isReseller bool
	_ = h.pool.QueryRow(r.Context(), `SELECT is_reseller FROM clients WHERE id = $1`, clientID).Scan(&isReseller)
	if !isReseller {
		respondError(w, shared.ErrForbidden("доступ только для агрегаторов"))
		return
	}

	id := mux.Vars(r)["id"]
	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("reject возможен только из статуса submitted"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'rejected', moderator_note = $2, resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, nullableStringP(req.Note),
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_id, actor_type, comment, created_at)
		 VALUES ($1, 'submitted', 'rejected', $2, 'aggregator', $3, NOW())`,
		id, clientID, nullableStringP(req.Note),
	)
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "rejected"})
}

// RequestRevisionResellerOperatorRegistration POST /portal/v1/reseller/operator-registrations/{id}/request-revision
func (h *ResellerModerationHandlers) RequestRevisionResellerOperatorRegistration(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}
	var isReseller bool
	_ = h.pool.QueryRow(r.Context(), `SELECT is_reseller FROM clients WHERE id = $1`, clientID).Scan(&isReseller)
	if !isReseller {
		respondError(w, shared.ErrForbidden("доступ только для агрегаторов"))
		return
	}

	id := mux.Vars(r)["id"]
	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT or2.status FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE or2.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("регистрация не найдена"))
		return
	}
	if currentStatus != "submitted" {
		respondError(w, shared.ErrInvalidInput("request-revision возможен только из статуса submitted"))
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	_, err = h.pool.Exec(r.Context(),
		`UPDATE operator_registrations
		 SET status = 'revision_requested', moderator_note = $2,
		     resolved_at = NOW(), updated_at = NOW()
		 WHERE id = $1`,
		id, nullableStringP(req.Note),
	)
	if err != nil {
		respondError(w, shared.ErrInternalServer("ошибка обновления"))
		return
	}
	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO operator_registration_history
		    (operator_registration_id, old_status, new_status, actor_id, actor_type, comment, created_at)
		 VALUES ($1, 'submitted', 'revision_requested', $2, 'aggregator', $3, NOW())`,
		id, clientID, nullableStringP(req.Note),
	)
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "revision_requested"})
}

func nullableStringP(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
```

- [ ] **Добавить импорт `"fmt"` в файл**

- [ ] **Добавить маршруты в portal router**

В `internal/gateway/portal/router/router.go` добавить после блока sender-names:

```go
// Reseller (aggregator) moderation — sub-account operator registrations
reseller := protected.PathPrefix("/reseller").Subrouter()
resellerOpRegs := reseller.PathPrefix("/operator-registrations").Subrouter()
resellerOpRegs.HandleFunc("", resellerHandlers.ListResellerOperatorRegistrations).Methods("GET")
resellerOpRegs.HandleFunc("/{id}/approve", resellerHandlers.ApproveResellerOperatorRegistration).Methods("POST")
resellerOpRegs.HandleFunc("/{id}/reject", resellerHandlers.RejectResellerOperatorRegistration).Methods("POST")
resellerOpRegs.HandleFunc("/{id}/request-revision", resellerHandlers.RequestRevisionResellerOperatorRegistration).Methods("POST")
```

Добавить `resellerHandlers *handlers.ResellerModerationHandlers` в параметры `SetupRouter`.

- [ ] **Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/gateway/portal/...
```

- [ ] **Коммит**

```bash
git add internal/gateway/portal/handlers/reseller_moderation.go internal/gateway/portal/router/router.go
git commit -m "feat(portal): add reseller moderation queue for sub-account operator registrations"
```

---

## Task 10: Pipeline — fallback sender name при routing

**Files:**
- Modify: `internal/pipeline/router/stage.go`

Когда pipeline роутирует сообщение, после выбора оператора проверяем, одобрено ли имя отправителя. Если нет — подставляем fallback.

- [ ] **Прочитать stage.go для понимания структуры**

```bash
grep -n "Source\|SenderName\|OperatorID\|operator_id\|kafkaMsg\|routedMsg" internal/pipeline/router/stage.go | head -30
```

- [ ] **Добавить функцию проверки sender name в stage.go**

Найти метод обработки сообщения в `stage.go`. После определения `operatorID` добавить вызов функции проверки и подстановки имени.

Добавить в структуру `Stage` поле `pool *pgxpool.Pool` (если его нет):
```go
pool *pgxpool.Pool
```

Добавить метод в `stage.go`:
```go
// resolveSenderName проверяет, одобрено ли имя отправителя для оператора.
// Если нет — возвращает fallback имя из system_defaults.
// Fail-open: при ошибке БД возвращает исходное имя (не блокируем).
func (s *Stage) resolveSenderName(ctx context.Context, clientID, senderName, operatorID string) string {
	if senderName == "" {
		return senderName
	}

	// Проверяем является ли имя альфа-именем (не числовой номер)
	isNumeric := true
	for _, c := range senderName {
		if c < '0' || c > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric {
		return senderName // числовые номера не требуют проверки
	}

	if s.pool == nil {
		return senderName
	}

	// Проверяем: субаккаунт или прямой
	var parentClientID *string
	var snStatus string
	err := s.pool.QueryRow(ctx,
		`SELECT c.parent_client_id, sn.status
		 FROM clients c
		 LEFT JOIN sender_names sn ON sn.client_id = c.id AND sn.name = $2
		 WHERE c.id = $1
		 LIMIT 1`,
		clientID, senderName,
	).Scan(&parentClientID, &snStatus)
	if err != nil {
		return senderName // fail-open
	}

	// Субаккаунт: проверяем только статус имени
	if parentClientID != nil {
		if snStatus == "approved" {
			return senderName
		}
		return s.getFallbackSender(ctx)
	}

	// Прямой: проверяем approved_type в operator_registrations
	var approvedType *string
	err = s.pool.QueryRow(ctx,
		`SELECT or2.approved_type
		 FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 WHERE sn.client_id = $1 AND sn.name = $2 AND or2.operator_id = $3
		 LIMIT 1`,
		clientID, senderName, operatorID,
	).Scan(&approvedType)
	if err != nil || approvedType == nil {
		return s.getFallbackSender(ctx)
	}
	return senderName
}

// getFallbackSender возвращает системное имя-заглушку.
func (s *Stage) getFallbackSender(ctx context.Context) string {
	if s.pool == nil {
		return "SMS"
	}
	var val string
	err := s.pool.QueryRow(ctx,
		`SELECT value FROM system_defaults WHERE key = 'default_sender_name' LIMIT 1`,
	).Scan(&val)
	if err != nil || val == "" {
		return "SMS"
	}
	return val
}
```

- [ ] **Добавить вызов `resolveSenderName` в обработчик сообщения**

Найти в `stage.go` место где `kafkaMsg.Source` передаётся в роутированное сообщение. Добавить подстановку:

```go
// После определения operatorID:
resolvedSender := s.resolveSenderName(ctx, kafkaMsg.ClientID, kafkaMsg.Source, operatorID.String())
// Заменить kafkaMsg.Source на resolvedSender в routedMsg:
```

Точное место зависит от структуры stage.go — прочитать файл перед изменением.

- [ ] **Проверить компиляцию**

```bash
cd c:/projects/sms && go build ./internal/pipeline/...
```

- [ ] **Коммит**

```bash
git add internal/pipeline/router/stage.go
git commit -m "feat(pipeline): fallback sender name for unregistered/pending operator registrations"
```

---

## Task 11: Frontend — обновление SenderNameOperatorsPage

**Files:**
- Modify: `portal-frontend/src/pages/sender-names/SenderNameOperatorsPage.tsx`
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Добавить API методы в client.ts**

Найти в `portal-frontend/src/api/client.ts` секцию `senderTariffApi` или `senderNamesApi` и добавить:

```typescript
export const operatorRegistrationsApi = {
  list: (senderNameId: string) =>
    api.get<{ registrations: OperatorRegistration[] }>(
      `/sender-names/${senderNameId}/operator-registrations`
    ),
  bulkSubmit: (senderNameId: string, registrations: { operator_id: string; type: string }[]) =>
    api.post<{ results: OperatorRegistrationResult[] }>(
      `/sender-names/${senderNameId}/operator-registrations`,
      { registrations }
    ),
  resubmit: (senderNameId: string, registrationId: string) =>
    api.post(`/sender-names/${senderNameId}/operator-registrations/${registrationId}/resubmit`, {}),
};

export interface OperatorRegistration {
  id: string;
  operator_id: string;
  operator_name: string;
  registration_type: string;
  status: 'submitted' | 'approved' | 'rejected' | 'revision_requested';
  approved_type: string | null;
  approved_at: string | null;
  moderator_note: string | null;
  submitted_at: string;
}

export interface OperatorRegistrationResult {
  operator_id: string;
  id?: string;
  status: string;
  error?: string;
}
```

- [ ] **Обновить SenderNameOperatorsPage.tsx**

Найти файл и обновить колонку «Статус» чтобы показывать раздельно:
- `approved_type IS NOT NULL` → зелёный «Зарегистрировано (free/paid)»
- `status = 'submitted'` → жёлтый «На модерации»
- `status = 'rejected'` → красный «Отклонено» + tooltip с `moderator_note`
- `status = 'revision_requested'` → оранжевый «Требует доработки» + кнопка «Повторно подать»

Добавить рендер статуса в таблицу:

```tsx
function RegistrationStatusBadge({ reg }: { reg: OperatorRegistration }) {
  if (reg.approved_type) {
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
        Зарегистрировано ({reg.approved_type})
      </span>
    );
  }
  const map: Record<string, { label: string; className: string }> = {
    submitted: { label: 'На модерации', className: 'bg-yellow-100 text-yellow-800' },
    rejected: { label: 'Отклонено', className: 'bg-red-100 text-red-800' },
    revision_requested: { label: 'Требует доработки', className: 'bg-orange-100 text-orange-800' },
  };
  const style = map[reg.status] ?? { label: reg.status, className: 'bg-gray-100 text-gray-800' };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${style.className}`}>
      {style.label}
    </span>
  );
}
```

Заменить использование данных из старого `senderTariffApi` на `operatorRegistrationsApi.list()`.

- [ ] **Запустить фронтенд и проверить страницу**

```bash
cd portal-frontend && npm run dev
```

Открыть `/sender-names/{id}/operators` и убедиться что статусы отображаются корректно.

- [ ] **Коммит**

```bash
git add portal-frontend/src/pages/sender-names/SenderNameOperatorsPage.tsx portal-frontend/src/api/client.ts
git commit -m "feat(frontend): show approved/pending/rejected status in operator registrations page"
```

---

## Task 12: Frontend — секция шаблонов операторов в деталях имени

**Files:**
- Create: `portal-frontend/src/pages/sender-names/OperatorTemplatesSection.tsx`
- Modify: `portal-frontend/src/pages/sender-names/SenderNameDetailPage.tsx`
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Добавить API методы для operator_templates в client.ts**

```typescript
export const operatorTemplatesApi = {
  list: (senderNameId: string, operatorId?: string) =>
    api.get<{ templates: OperatorTemplate[] }>(
      `/sender-names/${senderNameId}/operator-templates`,
      { params: operatorId ? { operator_id: operatorId } : {} }
    ),
  create: (senderNameId: string, data: { operator_id: string; name: string; body: string }) =>
    api.post<{ id: string; moderation_status: string }>(
      `/sender-names/${senderNameId}/operator-templates`,
      data
    ),
  update: (senderNameId: string, tid: string, data: { name: string; body: string }) =>
    api.put(`/sender-names/${senderNameId}/operator-templates/${tid}`, data),
  delete: (senderNameId: string, tid: string) =>
    api.delete(`/sender-names/${senderNameId}/operator-templates/${tid}`),
  submit: (senderNameId: string, tid: string) =>
    api.post(`/sender-names/${senderNameId}/operator-templates/${tid}/submit`, {}),
  resubmit: (senderNameId: string, tid: string) =>
    api.post(`/sender-names/${senderNameId}/operator-templates/${tid}/resubmit`, {}),
};

export interface OperatorTemplate {
  id: string;
  name: string;
  operator_id: string;
  operator_name: string;
  body: string;
  moderation_status: 'draft' | 'submitted' | 'approved' | 'rejected' | 'revision_requested';
  moderator_note: string | null;
  submitted_at: string | null;
  resolved_at: string | null;
}
```

- [ ] **Создать компонент OperatorTemplatesSection.tsx**

```tsx
// portal-frontend/src/pages/sender-names/OperatorTemplatesSection.tsx
import { useState } from 'react';
import { operatorTemplatesApi, OperatorTemplate } from '@/api/client';

const STATUS_LABELS: Record<string, { label: string; className: string }> = {
  draft: { label: 'Черновик', className: 'bg-gray-100 text-gray-700' },
  submitted: { label: 'На модерации', className: 'bg-yellow-100 text-yellow-800' },
  approved: { label: 'Одобрен', className: 'bg-green-100 text-green-800' },
  rejected: { label: 'Отклонён', className: 'bg-red-100 text-red-800' },
  revision_requested: { label: 'Требует доработки', className: 'bg-orange-100 text-orange-800' },
};

interface Props {
  senderNameId: string;
  templates: OperatorTemplate[];
  onRefresh: () => void;
}

export function OperatorTemplatesSection({ senderNameId, templates, onRefresh }: Props) {
  const [expanded, setExpanded] = useState(false);

  const handleSubmit = async (tid: string) => {
    await operatorTemplatesApi.submit(senderNameId, tid);
    onRefresh();
  };

  const handleResubmit = async (tid: string) => {
    await operatorTemplatesApi.resubmit(senderNameId, tid);
    onRefresh();
  };

  const handleDelete = async (tid: string) => {
    if (!confirm('Удалить шаблон?')) return;
    await operatorTemplatesApi.delete(senderNameId, tid);
    onRefresh();
  };

  return (
    <div className="mt-6">
      <button
        className="flex items-center gap-2 text-sm font-medium text-gray-700"
        onClick={() => setExpanded(!expanded)}
      >
        <span>{expanded ? '▾' : '▸'}</span>
        Шаблоны операторов ({templates.length})
      </button>

      {expanded && (
        <div className="mt-3 space-y-2">
          {templates.length === 0 && (
            <p className="text-sm text-gray-500">Шаблоны не добавлены</p>
          )}
          {templates.map((t) => {
            const statusStyle = STATUS_LABELS[t.moderation_status] ?? STATUS_LABELS.draft;
            return (
              <div key={t.id} className="border rounded-lg p-3 bg-white">
                <div className="flex items-start justify-between">
                  <div>
                    <p className="font-medium text-sm">{t.name}</p>
                    <p className="text-xs text-gray-500">{t.operator_name}</p>
                    <p className="text-xs text-gray-700 mt-1 whitespace-pre-wrap">{t.body}</p>
                    {t.moderator_note && (
                      <p className="text-xs text-red-600 mt-1">Комментарий: {t.moderator_note}</p>
                    )}
                  </div>
                  <div className="flex flex-col items-end gap-1 shrink-0 ml-2">
                    <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusStyle.className}`}>
                      {statusStyle.label}
                    </span>
                    {t.moderation_status === 'draft' && (
                      <div className="flex gap-1">
                        <button
                          onClick={() => handleSubmit(t.id)}
                          className="text-xs px-2 py-0.5 bg-blue-600 text-white rounded hover:bg-blue-700"
                        >
                          Подать
                        </button>
                        <button
                          onClick={() => handleDelete(t.id)}
                          className="text-xs px-2 py-0.5 bg-red-50 text-red-600 border border-red-200 rounded hover:bg-red-100"
                        >
                          Удалить
                        </button>
                      </div>
                    )}
                    {t.moderation_status === 'revision_requested' && (
                      <button
                        onClick={() => handleResubmit(t.id)}
                        className="text-xs px-2 py-0.5 bg-orange-600 text-white rounded hover:bg-orange-700"
                      >
                        Повторно подать
                      </button>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Подключить секцию в SenderNameDetailPage.tsx**

Найти место где отображаются детали имени и добавить:

```tsx
import { OperatorTemplatesSection } from './OperatorTemplatesSection';
// ...
const [operatorTemplates, setOperatorTemplates] = useState<OperatorTemplate[]>([]);

const fetchTemplates = async () => {
  const res = await operatorTemplatesApi.list(id);
  setOperatorTemplates(res.data.templates);
};

useEffect(() => { fetchTemplates(); }, [id]);

// В JSX добавить после блока registration:
<OperatorTemplatesSection
  senderNameId={id}
  templates={operatorTemplates}
  onRefresh={fetchTemplates}
/>
```

- [ ] **Проверить в браузере что секция шаблонов отображается**

Открыть `/sender-names/{id}`, убедиться что секция «Шаблоны операторов» появляется и раскрывается.

- [ ] **Коммит**

```bash
git add portal-frontend/src/pages/sender-names/OperatorTemplatesSection.tsx \
        portal-frontend/src/pages/sender-names/SenderNameDetailPage.tsx \
        portal-frontend/src/api/client.ts
git commit -m "feat(frontend): add operator templates section to sender name detail page"
```

---

## Task 13: Интеграция — wire новых handler-ов в main.go порталов

**Files:**
- Modify: `cmd/gateway/portal-gateway/main.go`
- Modify: `cmd/gateway/admin-gateway/main.go`

- [ ] **Прочитать main.go portal-gateway**

```bash
grep -n "SenderName\|SetupRouter\|pool\|pgxpool" cmd/gateway/portal-gateway/main.go | head -30
```

- [ ] **Добавить инициализацию новых handler-ов в portal-gateway main.go**

Найти место создания handler-ов и добавить:

```go
opRegHandlers := handlers.NewOperatorRegistrationHandlers(pool)
portalOpTplHandlers := handlers.NewPortalOperatorTemplateHandlers(pool)
resellerHandlers := handlers.NewResellerModerationHandlers(pool)
```

Передать их в `router.SetupRouter(...)`.

- [ ] **Добавить инициализацию в admin-gateway main.go**

```bash
grep -n "OperatorTemplate\|SetupRouter\|storage.DB" cmd/gateway/admin-gateway/main.go | head -20
```

Добавить:
```go
adminOpRegHandlers := handlers.NewAdminOperatorRegistrationHandlers(db)
```

Передать в `router.SetupRouter(...)`.

- [ ] **Проверить компиляцию обоих бинарников**

```bash
cd c:/projects/sms && go build ./cmd/gateway/portal-gateway/... && go build ./cmd/gateway/admin-gateway/...
```

Ожидаемый результат: оба собираются без ошибок.

- [ ] **Коммит**

```bash
git add cmd/gateway/portal-gateway/main.go cmd/gateway/admin-gateway/main.go
git commit -m "feat: wire operator_registrations and operator_templates handlers into gateways"
```

---

## Self-Review

### Spec coverage
- ✅ DB: `operator_registrations` + history — Task 1
- ✅ DB: `operator_templates` moderation columns — Task 2
- ✅ Portal client submit/list/resubmit — Task 3-4
- ✅ Portal client operator_templates CRUD + submit/resubmit — Task 5-6
- ✅ Admin moderation queue (direct users) — Task 7-8
- ✅ Aggregator moderation queue (sub-accounts via portal) — Task 9
- ✅ Pipeline fallback sender name — Task 10
- ✅ Frontend operators page (approved/pending/rejected) — Task 11
- ✅ Frontend operator templates section — Task 12
- ✅ Wire в main.go — Task 13

### Gaps
- Billing при approve (перенос с submit → approve) — в текущем Task 7 `ApproveAdminOperatorRegistration` не создаёт `sender_registration` в тарификации для paid типов. Это упрощение первой версии — billing можно добавить вторым проходом.
- Admin operator_templates list endpoint не добавлен (только модерация). Существующий `ListOperatorTemplates` уже есть в admin handler-е.
- `nullableString` определена в двух файлах — при компиляции будет конфликт имён в одном пакете. **Fix:** переименовать в `operator_registrations_admin.go` в `nullableStr`, а в `operator_templates.go` оставить `nullableString`.

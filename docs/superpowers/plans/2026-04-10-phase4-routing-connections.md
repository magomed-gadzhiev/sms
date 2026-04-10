# Phase 4: Routing Overhaul + SMPP Connections — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Переработать страницу маршрутизации под новую модель (оператор + канал + провайдер + юр.лицо с drag-and-drop) и добавить страницу управления SMPP-подключениями.

**Architecture:**
- `platform_routes` — новая таблица (не трогаем старую `routes`). Хендлер напрямую работает с DB (`*storage.DB`), как Phase 3 хендлеры.
- Connections — агрегирует список провайдеров с их health-статусом; reconnect/stop публикуют команды через Redis.
- Frontend: заменяем содержимое `RoutesPage.tsx` новой моделью; `ConnectionsPage.tsx` — новый файл с авто-обновлением каждые 10 с.

**Tech Stack:** Go 1.24 + pgx/v5 (через `*storage.DB`), gorilla/mux, Redis (go-redis/v9), TypeScript 5.7 + React 19, Tailwind CSS 4.2, Radix UI.

---

## File Map

| Действие | Файл |
|----------|------|
| Создать | `migrations/000084_phase4_platform_routes.up.sql` |
| Создать | `migrations/000084_phase4_platform_routes.down.sql` |
| Создать | `internal/gateway/admin/handlers/platform_routes.go` |
| Создать | `internal/gateway/admin/handlers/connections.go` |
| Изменить | `internal/gateway/admin/router/router.go` |
| Изменить | `cmd/admin-gateway/main.go` |
| Изменить | `portal-frontend/src/api/admin.ts` |
| Изменить | `portal-frontend/src/pages/admin/RoutesPage.tsx` |
| Создать | `portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx` |
| Изменить | `portal-frontend/src/components/layout/AdminSidebar.tsx` |
| Изменить | `portal-frontend/src/App.tsx` |

---

## Task 1: DB Migration — platform_routes

**Files:**
- Create: `migrations/000084_phase4_platform_routes.up.sql`
- Create: `migrations/000084_phase4_platform_routes.down.sql`

- [ ] **Step 1: Создать up-миграцию**

```sql
-- migrations/000084_phase4_platform_routes.up.sql
-- Phase 4: New operator-based routing table

CREATE TABLE IF NOT EXISTS platform_routes (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        REFERENCES operators(id) ON DELETE SET NULL,
  -- NULL operator_id means "All Networks"
  channel_type    VARCHAR(50) NOT NULL DEFAULT 'sms',
  -- channel_type: 'sms', 'flash', 'viber', 'whatsapp', etc.
  provider_id     UUID        NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
  legal_entity_id UUID        REFERENCES legal_entities(id) ON DELETE SET NULL,
  priority        INT         NOT NULL DEFAULT 0,
  active          BOOLEAN     NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_all_networks_requires_legal_entity
    CHECK (operator_id IS NOT NULL OR legal_entity_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_platform_routes_operator   ON platform_routes(operator_id);
CREATE INDEX IF NOT EXISTS idx_platform_routes_provider   ON platform_routes(provider_id);
CREATE INDEX IF NOT EXISTS idx_platform_routes_priority   ON platform_routes(priority);
CREATE INDEX IF NOT EXISTS idx_platform_routes_active     ON platform_routes(active);
CREATE INDEX IF NOT EXISTS idx_platform_routes_legal      ON platform_routes(legal_entity_id);
```

- [ ] **Step 2: Создать down-миграцию**

```sql
-- migrations/000084_phase4_platform_routes.down.sql
DROP TABLE IF EXISTS platform_routes;
```

- [ ] **Step 3: Применить миграцию (dev)**

```bash
# Узнать текущую версию
ls migrations/ | sort | tail -5

# Применить через goose или прямым SQL
psql $DATABASE_URL -f migrations/000084_phase4_platform_routes.up.sql
```

Ожидаемый результат: таблица `platform_routes` создана, индексы применены.

- [ ] **Step 4: Commit**

```bash
git add migrations/000084_phase4_platform_routes.up.sql migrations/000084_phase4_platform_routes.down.sql
git commit -m "feat(db): add platform_routes table for operator-based routing"
```

---

## Task 2: Backend — PlatformRoutesHandlers

**Files:**
- Create: `internal/gateway/admin/handlers/platform_routes.go`

- [ ] **Step 1: Создать файл хендлера**

```go
// internal/gateway/admin/handlers/platform_routes.go
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
)

// PlatformRoutesHandlers управляет operator-based маршрутами.
type PlatformRoutesHandlers struct {
	db *storage.DB
}

// NewPlatformRoutesHandlers создаёт обработчик.
func NewPlatformRoutesHandlers(db *storage.DB) *PlatformRoutesHandlers {
	return &PlatformRoutesHandlers{db: db}
}

type platformRouteRow struct {
	ID             string
	OperatorID     sql.NullString
	OperatorName   sql.NullString
	ChannelType    string
	ProviderID     string
	ProviderName   string
	LegalEntityID  sql.NullString
	LegalEntityName sql.NullString
	LegalEntityINN  sql.NullString
	Priority       int
	Active         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r platformRouteRow) toJSON() map[string]interface{} {
	m := map[string]interface{}{
		"id":           r.ID,
		"channel_type": r.ChannelType,
		"provider_id":  r.ProviderID,
		"provider_name": r.ProviderName,
		"priority":     r.Priority,
		"active":       r.Active,
		"created_at":   r.CreatedAt,
		"updated_at":   r.UpdatedAt,
	}
	if r.OperatorID.Valid {
		m["operator_id"] = r.OperatorID.String
		m["operator_name"] = r.OperatorName.String
	} else {
		m["operator_id"] = nil
		m["operator_name"] = "All Networks"
	}
	if r.LegalEntityID.Valid {
		m["legal_entity_id"] = r.LegalEntityID.String
		m["legal_entity_name"] = r.LegalEntityName.String
		m["legal_entity_inn"] = r.LegalEntityINN.String
	}
	return m
}

const platformRoutesListQuery = `
	SELECT
		pr.id::text,
		pr.operator_id::text,
		o.name,
		pr.channel_type,
		pr.provider_id::text,
		p.name,
		pr.legal_entity_id::text,
		le.name,
		le.inn,
		pr.priority,
		pr.active,
		pr.created_at,
		pr.updated_at
	FROM platform_routes pr
	LEFT JOIN operators o  ON o.id = pr.operator_id
	JOIN  providers p      ON p.id = pr.provider_id
	LEFT JOIN legal_entities le ON le.id = pr.legal_entity_id
	ORDER BY pr.priority ASC, pr.created_at ASC
	LIMIT $1 OFFSET $2
`

// ListPlatformRoutes обрабатывает GET /admin/v1/platform-routes
func (h *PlatformRoutesHandlers) ListPlatformRoutes(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 200)
	offset := parseIntParam(r, "offset", 0)

	rows, err := h.db.QueryContext(r.Context(), platformRoutesListQuery, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка списка")
		respondError(w, shared.ErrInternal("Ошибка получения маршрутов"))
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var row platformRouteRow
		if err := rows.Scan(
			&row.ID, &row.OperatorID, &row.OperatorName,
			&row.ChannelType,
			&row.ProviderID, &row.ProviderName,
			&row.LegalEntityID, &row.LegalEntityName, &row.LegalEntityINN,
			&row.Priority, &row.Active, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			continue
		}
		items = append(items, row.toJSON())
	}

	var total int64
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM platform_routes`).Scan(&total)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"routes": items,
		"total":  total,
	})
}

type createPlatformRouteRequest struct {
	OperatorID    string `json:"operator_id"`     // "" = All Networks
	ChannelType   string `json:"channel_type"`
	ProviderID    string `json:"provider_id"`
	LegalEntityID string `json:"legal_entity_id"` // required when operator_id == ""
	Priority      int    `json:"priority"`
}

// CreatePlatformRoute обрабатывает POST /admin/v1/platform-routes
func (h *PlatformRoutesHandlers) CreatePlatformRoute(w http.ResponseWriter, r *http.Request) {
	var req createPlatformRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.ProviderID == "" {
		respondError(w, shared.ErrInvalidInput("provider_id обязателен"))
		return
	}
	if req.ChannelType == "" {
		req.ChannelType = "sms"
	}
	if req.OperatorID == "" && req.LegalEntityID == "" {
		respondError(w, shared.ErrInvalidInput("При выборе All Networks необходимо указать legal_entity_id"))
		return
	}

	var operatorID, legalEntityID interface{}
	if req.OperatorID != "" {
		operatorID = req.OperatorID
	}
	if req.LegalEntityID != "" {
		legalEntityID = req.LegalEntityID
	}

	var id string
	var createdAt time.Time
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO platform_routes (operator_id, channel_type, provider_id, legal_entity_id, priority)
		VALUES ($1::uuid, $2, $3::uuid, $4::uuid, $5)
		RETURNING id::text, created_at
	`, operatorID, req.ChannelType, req.ProviderID, legalEntityID, req.Priority).Scan(&id, &createdAt)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка создания")
		respondError(w, shared.ErrInternal("Ошибка создания маршрута"))
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "created_at": createdAt})
}

// UpdatePlatformRoute обрабатывает PUT /admin/v1/platform-routes/{id}
func (h *PlatformRoutesHandlers) UpdatePlatformRoute(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		OperatorID    *string `json:"operator_id"`
		ChannelType   string  `json:"channel_type"`
		ProviderID    string  `json:"provider_id"`
		LegalEntityID *string `json:"legal_entity_id"`
		Priority      *int    `json:"priority"`
		Active        *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}

	res, err := h.db.ExecContext(r.Context(), `
		UPDATE platform_routes SET
			operator_id     = CASE WHEN $2::text = '' THEN NULL ELSE $2::uuid END,
			channel_type    = COALESCE(NULLIF($3, ''), channel_type),
			provider_id     = COALESCE(NULLIF($4, '')::uuid, provider_id),
			legal_entity_id = CASE WHEN $5::text = '' THEN NULL ELSE $5::uuid END,
			priority        = COALESCE($6, priority),
			active          = COALESCE($7, active),
			updated_at      = now()
		WHERE id = $1::uuid
	`, id,
		nullableString(req.OperatorID),
		req.ChannelType,
		req.ProviderID,
		nullableString(req.LegalEntityID),
		req.Priority,
		req.Active,
	)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка обновления")
		respondError(w, shared.ErrInternal("Ошибка обновления"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Маршрут не найден"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"updated": true})
}

// DeletePlatformRoute обрабатывает DELETE /admin/v1/platform-routes/{id}
func (h *PlatformRoutesHandlers) DeletePlatformRoute(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	res, err := h.db.ExecContext(r.Context(), `DELETE FROM platform_routes WHERE id = $1::uuid`, id)
	if err != nil {
		log.Error().Err(err).Msg("platform_routes: ошибка удаления")
		respondError(w, shared.ErrInternal("Ошибка удаления"))
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondError(w, shared.ErrNotFound("Маршрут не найден"))
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// ReorderPlatformRoutes обрабатывает PUT /admin/v1/platform-routes/reorder
// Body: {"items": [{"id": "...", "priority": 0}, ...]}
func (h *PlatformRoutesHandlers) ReorderPlatformRoutes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			ID       string `json:"id"`
			Priority int    `json:"priority"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if len(req.Items) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{"updated": 0})
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		respondError(w, shared.ErrInternal("Ошибка транзакции"))
		return
	}
	defer tx.Rollback()

	for _, item := range req.Items {
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE platform_routes SET priority = $2, updated_at = now() WHERE id = $1::uuid`,
			item.ID, item.Priority,
		); err != nil {
			log.Error().Err(err).Str("id", item.ID).Msg("platform_routes: ошибка reorder")
			respondError(w, shared.ErrInternal("Ошибка переупорядочивания"))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		respondError(w, shared.ErrInternal("Ошибка сохранения порядка"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"updated": len(req.Items)})
}

// nullableString преобразует *string в interface{} для SQL.
// nil pointer → NULL, пустая строка → NULL, иначе значение.
func nullableString(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}
```

- [ ] **Step 2: Убедиться, что `parseIntParam` и `respondJSON`/`respondError` уже существуют**

```bash
grep -n "func parseIntParam\|func respondJSON\|func respondError" internal/gateway/admin/handlers/*.go | head -20
```

Ожидаемый результат: все три функции найдены в каком-либо файле в пакете `handlers`.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/admin/handlers/platform_routes.go
git commit -m "feat(admin): add PlatformRoutesHandlers for operator-based routing"
```

---

## Task 3: Backend — ConnectionsHandlers

**Files:**
- Create: `internal/gateway/admin/handlers/connections.go`

- [ ] **Step 1: Создать хендлер**

```go
// internal/gateway/admin/handlers/connections.go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/storage"
	providerv1 "github.com/smpp-server/smpp-server/api/gen/provider/v1"
)

// ConnectionsHandlers управляет SMPP-подключениями (runtime view).
type ConnectionsHandlers struct {
	db             *storage.DB
	providerClient providerv1.ProviderServiceClient
	redis          *redis.Client
}

// NewConnectionsHandlers создаёт обработчик.
func NewConnectionsHandlers(db *storage.DB, providerClient providerv1.ProviderServiceClient, rdb *redis.Client) *ConnectionsHandlers {
	return &ConnectionsHandlers{db: db, providerClient: providerClient, redis: rdb}
}

// connectionInfo — агрегированные данные о подключении.
type connectionInfo struct {
	ProviderID        string    `json:"provider_id"`
	Name              string    `json:"name"`
	Host              string    `json:"host"`
	Port              int32     `json:"port"`
	SystemID          string    `json:"system_id"`
	BindType          int32     `json:"bind_type"`
	MaxConnections    int32     `json:"max_connections"`
	Status            string    `json:"status"`
	ActiveConnections int32     `json:"active_connections"`
	SuccessRate       float64   `json:"success_rate"`
	MessagesSent24h   int64     `json:"messages_sent_24h"`
	MessagesFailed24h int64     `json:"messages_failed_24h"`
	LastSuccess       string    `json:"last_success,omitempty"`
	LastFailure       string    `json:"last_failure,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	Active            bool      `json:"active"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ListConnections обрабатывает GET /admin/v1/connections
func (h *ConnectionsHandlers) ListConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Получаем список провайдеров из БД напрямую
	rows, err := h.db.QueryContext(ctx, `
		SELECT id::text, name, host, port, system_id, bind_type, max_connections, active, updated_at
		FROM providers
		ORDER BY name
	`)
	if err != nil {
		log.Error().Err(err).Msg("connections: ошибка списка провайдеров")
		respondError(w, shared.ErrInternal("Ошибка получения подключений"))
		return
	}
	defer rows.Close()

	type providerRow struct {
		ID             string
		Name           string
		Host           string
		Port           int32
		SystemID       string
		BindType       int32
		MaxConnections int32
		Active         bool
		UpdatedAt      time.Time
	}

	var providers []providerRow
	for rows.Next() {
		var p providerRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Host, &p.Port, &p.SystemID, &p.BindType, &p.MaxConnections, &p.Active, &p.UpdatedAt); err != nil {
			continue
		}
		providers = append(providers, p)
	}

	// Для каждого провайдера получаем health
	connections := make([]connectionInfo, 0, len(providers))
	for _, p := range providers {
		ci := connectionInfo{
			ProviderID:     p.ID,
			Name:           p.Name,
			Host:           p.Host,
			Port:           p.Port,
			SystemID:       p.SystemID,
			BindType:       p.BindType,
			MaxConnections: p.MaxConnections,
			Active:         p.Active,
			UpdatedAt:      p.UpdatedAt,
			Status:         "unknown",
		}

		// Запрашиваем health через gRPC
		healthCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		healthResp, err := h.providerClient.GetProviderHealth(healthCtx, &providerv1.GetProviderHealthRequest{ProviderId: p.ID})
		cancel()
		if err == nil && healthResp != nil {
			ci.Status = healthResp.Status
			ci.ActiveConnections = int32(healthResp.ActiveConnections)
			ci.SuccessRate = healthResp.SuccessRate
			ci.MessagesSent24h = healthResp.MessagesSent_24H
			ci.MessagesFailed24h = healthResp.MessagesFailed_24H
			if healthResp.LastSuccess != "" {
				ci.LastSuccess = healthResp.LastSuccess
			}
			if healthResp.LastFailure != "" {
				ci.LastFailure = healthResp.LastFailure
			}
			if healthResp.LastError != "" {
				ci.LastError = healthResp.LastError
			}
		}

		connections = append(connections, ci)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"connections": connections,
		"total":       len(connections),
	})
}

// GetConnection обрабатывает GET /admin/v1/connections/{id}
func (h *ConnectionsHandlers) GetConnection(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var ci connectionInfo
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id::text, name, host, port, system_id, bind_type, max_connections, active, updated_at
		FROM providers WHERE id = $1::uuid
	`, id).Scan(&ci.ProviderID, &ci.Name, &ci.Host, &ci.Port, &ci.SystemID, &ci.BindType, &ci.MaxConnections, &ci.Active, &ci.UpdatedAt)
	if err != nil {
		respondError(w, shared.ErrNotFound("Подключение не найдено"))
		return
	}
	ci.Status = "unknown"

	healthCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	healthResp, err := h.providerClient.GetProviderHealth(healthCtx, &providerv1.GetProviderHealthRequest{ProviderId: id})
	if err == nil && healthResp != nil {
		ci.Status = healthResp.Status
		ci.ActiveConnections = int32(healthResp.ActiveConnections)
		ci.SuccessRate = healthResp.SuccessRate
		ci.MessagesSent24h = healthResp.MessagesSent_24H
		ci.MessagesFailed24h = healthResp.MessagesFailed_24H
		ci.LastSuccess = healthResp.LastSuccess
		ci.LastFailure = healthResp.LastFailure
		ci.LastError = healthResp.LastError
	}

	respondJSON(w, http.StatusOK, ci)
}

// ReconnectConnection обрабатывает POST /admin/v1/connections/{id}/reconnect
func (h *ConnectionsHandlers) ReconnectConnection(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	h.publishConnectionCommand(r.Context(), id, "reconnect", w)
}

// StopConnection обрабатывает POST /admin/v1/connections/{id}/stop
func (h *ConnectionsHandlers) StopConnection(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	h.publishConnectionCommand(r.Context(), id, "stop", w)
}

func (h *ConnectionsHandlers) publishConnectionCommand(ctx context.Context, providerID, command string, w http.ResponseWriter) {
	msg, _ := json.Marshal(map[string]string{
		"provider_id": providerID,
		"command":     command,
	})
	if err := h.redis.Publish(ctx, "smpp:admin:commands", msg).Err(); err != nil {
		log.Error().Err(err).Str("provider_id", providerID).Str("command", command).Msg("connections: ошибка публикации команды")
		respondError(w, shared.ErrInternal("Ошибка отправки команды"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"queued": true, "provider_id": providerID, "command": command})
}
```

- [ ] **Step 2: Проверить импорты proto-пакета**

```bash
grep -r "GetProviderHealthRequest\|ProviderServiceClient" internal/gateway/admin/handlers/providers.go | head -5
```

Ожидаемый результат: `ProviderServiceClient` и `GetProviderHealthRequest` используются в `providers.go`. Убедитесь, что путь к proto-пакету (`api/gen/provider/v1`) совпадает с тем, что в `providers.go`.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/admin/handlers/connections.go
git commit -m "feat(admin): add ConnectionsHandlers for SMPP connection management"
```

---

## Task 4: Регистрация хендлеров в роутере и main.go

**Files:**
- Modify: `internal/gateway/admin/router/router.go`
- Modify: `cmd/admin-gateway/main.go`

- [ ] **Step 1: Добавить хендлеры в сигнатуру SetupRouter**

В `internal/gateway/admin/router/router.go`, изменить сигнатуру функции `SetupRouter` — добавить два новых параметра перед `healthChecker`:

```go
// Добавить параметры:
platformRoutesHandlers *handlers.PlatformRoutesHandlers,
connectionsHandlers *handlers.ConnectionsHandlers,
```

- [ ] **Step 2: Добавить эндпоинты в тело SetupRouter**

Добавить после блока `// Operator Templates endpoints`:

```go
// Platform Routes endpoints (новая модель маршрутизации)
platformRoutes := adminV1.PathPrefix("/platform-routes").Subrouter()
platformRoutes.HandleFunc("/reorder", platformRoutesHandlers.ReorderPlatformRoutes).Methods("PUT")
platformRoutes.HandleFunc("", platformRoutesHandlers.ListPlatformRoutes).Methods("GET")
platformRoutes.HandleFunc("", platformRoutesHandlers.CreatePlatformRoute).Methods("POST")
platformRoutes.HandleFunc("/{id}", platformRoutesHandlers.UpdatePlatformRoute).Methods("PUT")
platformRoutes.HandleFunc("/{id}", platformRoutesHandlers.DeletePlatformRoute).Methods("DELETE")

// Connections endpoints (SMPP connection management)
connections := adminV1.PathPrefix("/connections").Subrouter()
connections.HandleFunc("", connectionsHandlers.ListConnections).Methods("GET")
connections.HandleFunc("/{id}", connectionsHandlers.GetConnection).Methods("GET")
connections.HandleFunc("/{id}/reconnect", connectionsHandlers.ReconnectConnection).Methods("POST")
connections.HandleFunc("/{id}/stop", connectionsHandlers.StopConnection).Methods("POST")
```

- [ ] **Step 3: Обновить main.go — инициализировать Redis и новые хендлеры**

В `cmd/admin-gateway/main.go` добавить после `operatorTemplateHandlers := ...`:

```go
// Redis клиент для ConnectionsHandlers
redisClient := redis.NewClient(&redis.Options{
    Addr: getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
})
defer redisClient.Close()

platformRoutesHandlers := handlers.NewPlatformRoutesHandlers(adminDB)
connectionsHandlers := handlers.NewConnectionsHandlers(adminDB, serviceClients.ProviderClient, redisClient)
```

Добавить в начало файла импорт Redis:

```go
"github.com/redis/go-redis/v9"
```

- [ ] **Step 4: Передать новые хендлеры в SetupRouter**

В `cmd/admin-gateway/main.go`, в вызове `adminrouter.SetupRouter(...)` добавить перед `healthChecker`:

```go
platformRoutesHandlers,
connectionsHandlers,
```

- [ ] **Step 5: Скомпилировать**

```bash
cd c:/projects/sms
go build ./cmd/admin-gateway/...
```

Ожидаемый результат: компиляция проходит без ошибок.

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/admin/router/router.go cmd/admin-gateway/main.go
git commit -m "feat(admin): register platform-routes and connections endpoints"
```

---

## Task 5: Frontend — API типы и функции

**Files:**
- Modify: `portal-frontend/src/api/admin.ts`

- [ ] **Step 1: Добавить интерфейсы и API в admin.ts**

Найти в `admin.ts` блок `// ── Types ──` и добавить после существующих типов (после `RouteInfo`):

```typescript
export interface PlatformRoute {
  id: string;
  operator_id: string | null;       // null = All Networks
  operator_name: string;            // "All Networks" когда null
  channel_type: string;             // 'sms' | 'flash' | ...
  provider_id: string;
  provider_name: string;
  legal_entity_id?: string;
  legal_entity_name?: string;
  legal_entity_inn?: string;
  priority: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface ConnectionInfo {
  provider_id: string;
  name: string;
  host: string;
  port: number;
  system_id: string;
  bind_type: number;
  max_connections: number;
  status: string;                   // 'healthy' | 'degraded' | 'unhealthy' | 'unknown'
  active_connections: number;
  success_rate: number;
  messages_sent_24h: number;
  messages_failed_24h: number;
  last_success?: string;
  last_failure?: string;
  last_error?: string;
  active: boolean;
  updated_at: string;
}
```

- [ ] **Step 2: Добавить API-функции в admin.ts**

Добавить после `routesApi`:

```typescript
export const platformRoutesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminFetch<{ routes: PlatformRoute[]; total: number }>(`/platform-routes${qs(params || {})}`),
  create: (data: {
    operator_id?: string;
    channel_type: string;
    provider_id: string;
    legal_entity_id?: string;
    priority?: number;
  }) =>
    adminFetch<{ id: string; created_at: string }>('/platform-routes', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  update: (id: string, data: Partial<{
    operator_id: string;
    channel_type: string;
    provider_id: string;
    legal_entity_id: string;
    priority: number;
    active: boolean;
  }>) =>
    adminFetch<void>(`/platform-routes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/platform-routes/${id}`, { method: 'DELETE' }),
  reorder: (items: Array<{ id: string; priority: number }>) =>
    adminFetch<{ updated: number }>('/platform-routes/reorder', {
      method: 'PUT',
      body: JSON.stringify({ items }),
    }),
};

export const connectionsApi = {
  list: () => adminFetch<{ connections: ConnectionInfo[]; total: number }>('/connections'),
  get: (id: string) => adminFetch<ConnectionInfo>(`/connections/${id}`),
  reconnect: (id: string) =>
    adminFetch<{ queued: boolean }>(`/connections/${id}/reconnect`, { method: 'POST' }),
  stop: (id: string) =>
    adminFetch<{ queued: boolean }>(`/connections/${id}/stop`, { method: 'POST' }),
};
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/admin.ts
git commit -m "feat(admin-api): add PlatformRoute and ConnectionInfo types + API functions"
```

---

## Task 6: Frontend — новый RoutesPage (REWORK-4)

**Files:**
- Modify: `portal-frontend/src/pages/admin/RoutesPage.tsx`

- [ ] **Step 1: Заменить RoutesPage.tsx**

Полностью заменить содержимое файла `portal-frontend/src/pages/admin/RoutesPage.tsx`:

```tsx
import { useState, useEffect, useCallback, useRef } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Select } from '../../components/ui/Select';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { StatusBadge } from '../../components/ui/Badge';
import { useToast } from '../../components/ui/Toast';
import {
  platformRoutesApi,
  operatorsApi,
  providersApi,
  type PlatformRoute,
  type OperatorInfo,
  type ProviderInfo,
} from '../../api/admin';
import { legalEntitiesApi, type LegalEntity } from '../../api/admin';

const CHANNEL_TYPES = [
  { value: 'sms', label: 'SMS' },
  { value: 'flash', label: 'Flash' },
  { value: 'viber', label: 'Viber' },
  { value: 'whatsapp', label: 'WhatsApp' },
  { value: 'vk', label: 'VK' },
];

const ALL_NETWORKS_VALUE = '';

function BindTypeLabel({ type }: { type: number }) {
  return <span>{type === 1 ? 'TX' : type === 2 ? 'RX' : 'TRX'}</span>;
}

export function RoutesPage() {
  const toast = useToast();
  const [routes, setRoutes] = useState<PlatformRoute[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editRoute, setEditRoute] = useState<PlatformRoute | null>(null);
  const [deleteRoute, setDeleteRoute] = useState<PlatformRoute | null>(null);
  const [saving, setSaving] = useState(false);

  // Drag-and-drop state
  const dragItemRef = useRef<number | null>(null);
  const dragOverItemRef = useRef<number | null>(null);

  const [operators, setOperators] = useState<OperatorInfo[]>([]);
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [legalEntities, setLegalEntities] = useState<LegalEntity[]>([]);

  const [form, setForm] = useState({
    operator_id: ALL_NETWORKS_VALUE,
    channel_type: 'sms',
    provider_id: '',
    legal_entity_id: '',
    priority: 0,
  });

  const isAllNetworks = form.operator_id === ALL_NETWORKS_VALUE;

  const fetchRoutes = useCallback(async () => {
    setLoading(true);
    try {
      const res = await platformRoutesApi.list({ limit: 500 });
      setRoutes(res.routes || []);
      setTotal(res.total);
    } catch {
      toast.error('Не удалось загрузить маршруты');
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    fetchRoutes();
    providersApi.list({ limit: 500 }).then((r) => setProviders(r.providers || [])).catch(() => {});
    operatorsApi?.list?.({ limit: 500 }).then((r) => setOperators(r.operators || [])).catch(() => {});
    legalEntitiesApi?.list?.({ limit: 500 }).then((r) => setLegalEntities(r.legal_entities || [])).catch(() => {});
  }, [fetchRoutes]);

  const operatorOptions = [
    { value: ALL_NETWORKS_VALUE, label: '— All Networks —' },
    ...operators.map((o) => ({ value: o.operator_id, label: `${o.name} (MCC${o.mcc}/MNC${o.mnc})` })),
  ];
  const providerOptions = providers.map((p) => ({ value: p.provider_id, label: p.name }));
  const legalEntityOptions = legalEntities.map((le) => ({
    value: le.id,
    label: `${le.inn} — ${le.name}`,
  }));

  const openCreate = () => {
    setForm({ operator_id: ALL_NETWORKS_VALUE, channel_type: 'sms', provider_id: '', legal_entity_id: '', priority: routes.length });
    setEditRoute(null);
    setShowForm(true);
  };

  const openEdit = (route: PlatformRoute) => {
    setForm({
      operator_id: route.operator_id ?? ALL_NETWORKS_VALUE,
      channel_type: route.channel_type,
      provider_id: route.provider_id,
      legal_entity_id: route.legal_entity_id ?? '',
      priority: route.priority,
    });
    setEditRoute(route);
    setShowForm(true);
  };

  const handleSave = async () => {
    if (!form.provider_id) { toast.error('Выберите провайдера'); return; }
    if (isAllNetworks && !form.legal_entity_id) { toast.error('При All Networks необходимо указать юр. лицо'); return; }
    setSaving(true);
    try {
      const payload = {
        operator_id: form.operator_id || undefined,
        channel_type: form.channel_type,
        provider_id: form.provider_id,
        legal_entity_id: form.legal_entity_id || undefined,
        priority: Number(form.priority),
      };
      if (editRoute) {
        await platformRoutesApi.update(editRoute.id, payload);
        toast.success('Маршрут обновлён');
      } else {
        await platformRoutesApi.create(payload);
        toast.success('Маршрут создан');
      }
      setShowForm(false);
      fetchRoutes();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка сохранения');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteRoute) return;
    setSaving(true);
    try {
      await platformRoutesApi.delete(deleteRoute.id);
      toast.success('Маршрут удалён');
      setDeleteRoute(null);
      fetchRoutes();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка удаления');
    } finally {
      setSaving(false);
    }
  };

  // Drag-and-drop handlers
  const handleDragStart = (index: number) => { dragItemRef.current = index; };
  const handleDragEnter = (index: number) => { dragOverItemRef.current = index; };

  const handleDragEnd = async () => {
    const dragItem = dragItemRef.current;
    const dragOver = dragOverItemRef.current;
    if (dragItem === null || dragOver === null || dragItem === dragOver) {
      dragItemRef.current = null;
      dragOverItemRef.current = null;
      return;
    }

    const newRoutes = [...routes];
    const [removed] = newRoutes.splice(dragItem, 1);
    newRoutes.splice(dragOver, 0, removed);

    // Переназначаем приоритеты по индексу
    const updated = newRoutes.map((r, i) => ({ ...r, priority: i }));
    setRoutes(updated);
    dragItemRef.current = null;
    dragOverItemRef.current = null;

    try {
      await platformRoutesApi.reorder(updated.map((r) => ({ id: r.id, priority: r.priority })));
    } catch {
      toast.error('Ошибка сохранения порядка');
      fetchRoutes(); // откатываемся
    }
  };

  const channelLabel = (type: string) => CHANNEL_TYPES.find((c) => c.value === type)?.label ?? type;

  const columns: Column<PlatformRoute>[] = [
    {
      key: 'priority',
      header: '⠿',
      render: () => (
        <span className="text-gray-400 cursor-grab select-none">⠿</span>
      ),
    },
    {
      key: 'operator_name',
      header: 'Оператор',
      render: (r) =>
        r.operator_id ? (
          <span>{r.operator_name}</span>
        ) : (
          <span className="font-medium text-blue-600">All Networks</span>
        ),
    },
    {
      key: 'channel_type',
      header: 'Канал',
      render: (r) => <span className="text-sm">{channelLabel(r.channel_type)}</span>,
    },
    { key: 'provider_name', header: 'Провайдер' },
    {
      key: 'legal_entity_inn',
      header: 'Юр. лицо',
      render: (r) =>
        r.legal_entity_id ? (
          <span className="text-sm text-gray-600">{r.legal_entity_inn} — {r.legal_entity_name}</span>
        ) : (
          <span className="text-gray-400">—</span>
        ),
    },
    {
      key: 'active',
      header: 'Статус',
      render: (r) => <StatusBadge status={r.active ? 'active' : 'inactive'} />,
    },
  ];

  return (
    <>
      <PageHeader
        title="Маршрутизация"
        subtitle={`${total} маршрутов`}
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Маршрутизация' }]}
        actions={<Button onClick={openCreate}>Добавить маршрут</Button>}
      />

      {/* Drag-and-drop таблица */}
      <div className="overflow-x-auto rounded-lg border border-gray-200">
        <table className="w-full text-sm">
          <thead className="bg-gray-50">
            <tr>
              {columns.map((col) => (
                <th key={String(col.key)} className="px-4 py-3 text-left font-medium text-gray-600">
                  {col.header}
                </th>
              ))}
              <th className="px-4 py-3 text-left font-medium text-gray-600">Действия</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={columns.length + 1} className="py-8 text-center text-gray-400">
                  Загрузка...
                </td>
              </tr>
            ) : routes.length === 0 ? (
              <tr>
                <td colSpan={columns.length + 1} className="py-8 text-center text-gray-400">
                  Маршруты не настроены
                </td>
              </tr>
            ) : (
              routes.map((route, index) => (
                <tr
                  key={route.id}
                  draggable
                  onDragStart={() => handleDragStart(index)}
                  onDragEnter={() => handleDragEnter(index)}
                  onDragEnd={handleDragEnd}
                  onDragOver={(e) => e.preventDefault()}
                  className="border-t border-gray-100 hover:bg-gray-50 cursor-grab active:cursor-grabbing"
                >
                  {columns.map((col) => (
                    <td key={String(col.key)} className="px-4 py-3">
                      {col.render ? col.render(route) : String(route[col.key] ?? '')}
                    </td>
                  ))}
                  <td className="px-4 py-3">
                    <div className="flex gap-1">
                      <Button size="sm" variant="ghost" onClick={() => openEdit(route)}>
                        Изменить
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => setDeleteRoute(route)}>
                        Удалить
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Форма создания/редактирования */}
      <Modal
        open={showForm}
        onClose={() => { setShowForm(false); setEditRoute(null); }}
        title={editRoute ? 'Редактирование маршрута' : 'Добавить маршрут'}
      >
        <div className="space-y-4">
          <Select
            label="Оператор"
            options={operatorOptions}
            value={form.operator_id}
            onChange={(v) => setForm({ ...form, operator_id: v, legal_entity_id: v ? '' : form.legal_entity_id })}
          />

          {isAllNetworks && (
            <Select
              label="Юридическое лицо *"
              options={legalEntityOptions}
              value={form.legal_entity_id}
              onChange={(v) => setForm({ ...form, legal_entity_id: v })}
              placeholder="Выберите юр. лицо..."
            />
          )}

          <Select
            label="Канал"
            options={CHANNEL_TYPES}
            value={form.channel_type}
            onChange={(v) => setForm({ ...form, channel_type: v })}
          />

          <Select
            label="Провайдер *"
            options={providerOptions}
            value={form.provider_id}
            onChange={(v) => setForm({ ...form, provider_id: v })}
            placeholder="Выберите провайдера..."
          />

          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={() => { setShowForm(false); setEditRoute(null); }}>
              Отмена
            </Button>
            <Button
              onClick={handleSave}
              disabled={saving || !form.provider_id || (isAllNetworks && !form.legal_entity_id)}
            >
              {saving ? 'Сохранение...' : 'Сохранить'}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!deleteRoute}
        onConfirm={handleDelete}
        onCancel={() => setDeleteRoute(null)}
        title="Удаление маршрута"
        description={`Удалить маршрут "${deleteRoute?.operator_name} → ${deleteRoute?.provider_name}"?`}
        confirmLabel="Удалить"
        variant="danger"
        loading={saving}
      />
    </>
  );
}
```

- [ ] **Step 2: Добавить `legalEntitiesApi` в admin.ts (если не существует)**

Проверить наличие:
```bash
grep -n "legalEntitiesApi\|LegalEntity" portal-frontend/src/api/admin.ts | head -10
```

Если `legalEntitiesApi` не найден — добавить в `admin.ts` после `connectionsApi`:

```typescript
export interface LegalEntity {
  id: string;
  inn: string;
  name: string;
  full_name?: string;
  address?: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export const legalEntitiesApi = {
  list: (params?: { limit?: number; offset?: number }) =>
    adminFetch<{ legal_entities: LegalEntity[]; total: number }>(`/legal-entities${qs(params || {})}`),
  get: (id: string) => adminFetch<LegalEntity>(`/legal-entities/${id}`),
  create: (data: { inn: string; name: string; full_name?: string; address?: string }) =>
    adminFetch<LegalEntity>('/legal-entities', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<LegalEntity>) =>
    adminFetch<LegalEntity>(`/legal-entities/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => adminFetch<void>(`/legal-entities/${id}`, { method: 'DELETE' }),
};
```

Также добавить `operatorsApi`, если отсутствует:

```typescript
export const operatorsApi = {
  list: (params?: { country_id?: string; active_only?: boolean; limit?: number; offset?: number }) =>
    adminFetch<{ operators: OperatorInfo[]; total: number }>(`/operators${qs(params || {})}`),
  get: (id: string) => adminFetch<{ operator: OperatorInfo }>(`/operators/${id}`),
};
```

- [ ] **Step 3: Проверить компиляцию TypeScript**

```bash
cd portal-frontend && npx tsc --noEmit 2>&1 | head -30
```

Ожидаемый результат: нет ошибок TS, связанных с новыми файлами.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/admin/RoutesPage.tsx portal-frontend/src/api/admin.ts
git commit -m "feat(admin-ui): replace RoutesPage with operator+channel+provider model + drag-and-drop reorder"
```

---

## Task 7: Frontend — ConnectionsPage (NEW-2)

**Files:**
- Create: `portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx`

- [ ] **Step 1: Создать директорию и файл**

```bash
mkdir -p portal-frontend/src/pages/admin/connections
```

- [ ] **Step 2: Написать ConnectionsPage.tsx**

```tsx
// portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx
import { useState, useEffect, useCallback, useRef } from 'react';
import { PageHeader } from '../../../components/layout/PageHeader';
import { Button } from '../../../components/ui/Button';
import { StatusBadge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import { connectionsApi, type ConnectionInfo } from '../../../api/admin';

const REFRESH_INTERVAL_MS = 10_000;

function statusBadge(status: string) {
  if (status === 'healthy') return <StatusBadge status="active" label="Подключён" />;
  if (status === 'degraded') return <StatusBadge status="pending" label="Деградация" />;
  if (status === 'unhealthy') return <StatusBadge status="inactive" label="Отключён" />;
  return <StatusBadge status="inactive" label="Неизвестно" />;
}

function bindTypeLabel(type: number) {
  return type === 1 ? 'TX' : type === 2 ? 'RX' : 'TRX';
}

export function ConnectionsPage() {
  const toast = useToast();
  const [connections, setConnections] = useState<ConnectionInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [actioning, setActioning] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchConnections = useCallback(async () => {
    try {
      const res = await connectionsApi.list();
      setConnections(res.connections || []);
      setTotal(res.total);
    } catch {
      // silent — don't spam toasts on auto-refresh
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchConnections();
    timerRef.current = setInterval(fetchConnections, REFRESH_INTERVAL_MS);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [fetchConnections]);

  const handleReconnect = async (conn: ConnectionInfo) => {
    setActioning(conn.provider_id);
    try {
      await connectionsApi.reconnect(conn.provider_id);
      toast.success(`Команда reconnect отправлена для ${conn.name}`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setActioning(null);
    }
  };

  const handleStop = async (conn: ConnectionInfo) => {
    setActioning(conn.provider_id);
    try {
      await connectionsApi.stop(conn.provider_id);
      toast.success(`Команда stop отправлена для ${conn.name}`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка');
    } finally {
      setActioning(null);
    }
  };

  return (
    <>
      <PageHeader
        title="Список подключений"
        subtitle={`${total} SMPP-подключений · автообновление 10 с`}
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Подключения' }]}
        actions={
          <Button variant="secondary" onClick={fetchConnections} disabled={loading}>
            {loading ? 'Обновление...' : 'Обновить'}
          </Button>
        }
      />

      <div className="overflow-x-auto rounded-lg border border-gray-200">
        <table className="w-full text-sm">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Провайдер</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Host:Port</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">System ID</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Bind</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Статус</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Сессий</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Успех %</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Отправлено 24ч</th>
              <th className="px-4 py-3 text-left font-medium text-gray-600">Действия</th>
            </tr>
          </thead>
          <tbody>
            {loading && connections.length === 0 ? (
              <tr>
                <td colSpan={9} className="py-8 text-center text-gray-400">
                  Загрузка...
                </td>
              </tr>
            ) : connections.length === 0 ? (
              <tr>
                <td colSpan={9} className="py-8 text-center text-gray-400">
                  Провайдеры не настроены
                </td>
              </tr>
            ) : (
              connections.map((conn) => (
                <tr key={conn.provider_id} className="border-t border-gray-100 hover:bg-gray-50">
                  <td className="px-4 py-3 font-medium">{conn.name}</td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-600">
                    {conn.host}:{conn.port}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">{conn.system_id}</td>
                  <td className="px-4 py-3">{bindTypeLabel(conn.bind_type)}</td>
                  <td className="px-4 py-3">{statusBadge(conn.status)}</td>
                  <td className="px-4 py-3">
                    <span className="font-medium">{conn.active_connections}</span>
                    <span className="text-gray-400">/{conn.max_connections}</span>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={
                        conn.success_rate >= 95
                          ? 'text-green-600 font-medium'
                          : conn.success_rate >= 80
                          ? 'text-yellow-600 font-medium'
                          : 'text-red-600 font-medium'
                      }
                    >
                      {conn.success_rate.toFixed(1)}%
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-green-600">{conn.messages_sent_24h.toLocaleString()}</span>
                    {conn.messages_failed_24h > 0 && (
                      <span className="text-red-500 ml-1">
                        / {conn.messages_failed_24h.toLocaleString()} ошибок
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleReconnect(conn)}
                        disabled={actioning === conn.provider_id}
                      >
                        Реконнект
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleStop(conn)}
                        disabled={actioning === conn.provider_id || !conn.active}
                        className="text-red-600 hover:text-red-700"
                      >
                        Стоп
                      </Button>
                    </div>
                    {conn.last_error && (
                      <p className="text-xs text-red-500 mt-1 max-w-[200px] truncate" title={conn.last_error}>
                        {conn.last_error}
                      </p>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}
```

- [ ] **Step 3: Проверить TypeScript**

```bash
cd portal-frontend && npx tsc --noEmit 2>&1 | grep "connections" | head -20
```

Ожидаемый результат: нет ошибок для нового файла.

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/pages/admin/connections/ConnectionsPage.tsx
git commit -m "feat(admin-ui): add ConnectionsPage for SMPP connection management"
```

---

## Task 8: Sidebar + App Router

**Files:**
- Modify: `portal-frontend/src/components/layout/AdminSidebar.tsx`
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Читать текущий AdminSidebar.tsx**

Прочитать файл и найти блок "Маршрутизация" или "Инфраструктура":

```bash
grep -n "routes\|connections\|Маршрут\|Подключен" portal-frontend/src/components/layout/AdminSidebar.tsx | head -20
```

- [ ] **Step 2: Добавить "Список подключений" в сайдбар**

Найти в `AdminSidebar.tsx` пункт для `/admin/routes` и добавить после него пункт для `/admin/connections`:

```tsx
// Добавить в группу "Инфраструктура" или "Маршрутизация":
{ label: 'Подключения', href: '/admin/connections', icon: '⚡', resource: 'providers' },
```

(Точный JSX — взять по образцу соседних пунктов в файле.)

- [ ] **Step 3: Читать текущий App.tsx**

```bash
grep -n "RoutesPage\|admin/routes\|admin/connections\|lazy\|import" portal-frontend/src/App.tsx | head -40
```

- [ ] **Step 4: Добавить маршруты в App.tsx**

Найти импорт `RoutesPage` и добавить рядом:

```tsx
// Если App.tsx использует lazy imports:
const ConnectionsPage = lazy(() =>
  import('./pages/admin/connections/ConnectionsPage').then((m) => ({ default: m.ConnectionsPage }))
);

// Если прямые импорты:
import { ConnectionsPage } from './pages/admin/connections/ConnectionsPage';
```

Добавить маршрут рядом с `/admin/routes`:

```tsx
<Route path="/admin/connections" element={<ConnectionsPage />} />
```

- [ ] **Step 5: Проверить компиляцию и запустить dev-сервер**

```bash
cd portal-frontend && npx tsc --noEmit 2>&1 | head -20
```

```bash
cd portal-frontend && npm run dev
```

Ожидаемый результат: приложение запускается, `/admin/routes` открывает новую страницу маршрутизации, `/admin/connections` открывает страницу подключений.

- [ ] **Step 6: Финальный commit**

```bash
git add portal-frontend/src/components/layout/AdminSidebar.tsx portal-frontend/src/App.tsx
git commit -m "feat(admin-ui): wire Phase 4 routes — /admin/routes (new model) + /admin/connections"
```

---

## Spec Coverage Check

| Требование из спека | Реализовано в |
|---------------------|---------------|
| NEW-2: `GET /admin/v1/connections` | Task 3 + Task 4 |
| NEW-2: `POST /admin/v1/connections/{id}/reconnect` | Task 3 + Task 4 |
| NEW-2: `POST /admin/v1/connections/{id}/stop` | Task 3 + Task 4 |
| NEW-2: ConnectionsPage с авто-обновлением | Task 7 |
| NEW-2: Статус, сессии, TPS/SuccessRate, действия | Task 7 |
| REWORK-4: Новая таблица `platform_routes` | Task 1 |
| REWORK-4: Operator или "All Networks" | Task 2, Task 6 |
| REWORK-4: channel_type | Task 1, Task 2, Task 6 |
| REWORK-4: legal_entity_id при All Networks | Task 1, Task 2, Task 6 |
| REWORK-4: Drag-and-drop reorder | Task 6 |
| REWORK-4: `PUT /admin/v1/platform-routes/reorder` | Task 2, Task 4 |
| REWORK-4: CRUD endpoints | Task 2, Task 4 |
| Sidebar: новый пункт "Подключения" | Task 8 |

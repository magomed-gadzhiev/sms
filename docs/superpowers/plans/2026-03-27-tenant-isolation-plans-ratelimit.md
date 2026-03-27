# Tenant Isolation + Subscription Plans + Rate-Limiting — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить модель подписочных планов (Starter/Business/Pro), привязать к клиентам, включить rate-limiting на основе плана, добавить PostgreSQL RLS как safety net для изоляции данных между тенантами.

**Architecture:** Новая таблица `subscription_plans` определяет лимиты и квоты. Поле `plan_id` в таблице `clients` привязывает клиента к плану. Rate-limiting middleware (уже существует, но отключён) переключается на чтение лимитов из плана. RLS-политики на таблицах `messages`, `transactions`, `accounts` обеспечивают изоляцию на уровне БД. Трекинг использования через Redis-счётчики (ежемесячный объём SMS).

**Tech Stack:** Go 1.24.0, PostgreSQL 15+ (RLS), Redis 7+ (rate-limit counters), pgx/v5, go-redis/v9, testify

---

## File Structure

### New Files
- `migrations/000030_create_subscription_plans.up.sql` — таблица планов + привязка к клиентам
- `migrations/000030_create_subscription_plans.down.sql` — откат
- `migrations/000031_add_rls_policies.up.sql` — RLS-политики
- `migrations/000031_add_rls_policies.down.sql` — откат RLS
- `internal/services/client/domain/plan.go` — доменная модель плана
- `internal/services/client/infrastructure/repository/plan_repository.go` — репозиторий планов
- `internal/services/client/application/plan_service.go` — сервис управления планами
- `internal/middleware/quota.go` — middleware проверки квот (месячный лимит SMS)
- `internal/middleware/ratelimit.go` — обновлённый rate-limiter на базе плана
- `tests/integration/plan_test.go` — интеграционные тесты планов
- `tests/integration/ratelimit_test.go` — интеграционные тесты rate-limiting
- `tests/integration/rls_test.go` — тесты RLS-изоляции

### Modified Files
- `internal/services/client/domain/client.go` — добавить PlanID к Client
- `internal/services/client/application/client_service.go` — логика назначения плана
- `internal/services/client/infrastructure/repository/client_repository.go` — запросы с plan_id
- `internal/gateway/client/middleware/auth.go` — включить auth, прокинуть plan limits в контекст
- `internal/api/middleware/auth.go` — включить auth
- `internal/api/middleware/ratelimit.go` — включить и обновить rate-limiting
- `api/proto/client/client.proto` — добавить Plan message и методы

---

## Task 1: Миграция — таблица subscription_plans + привязка к клиентам

**Files:**
- Create: `migrations/000030_create_subscription_plans.up.sql`
- Create: `migrations/000030_create_subscription_plans.down.sql`

- [ ] **Step 1: Создать up-миграцию**

```sql
-- migrations/000030_create_subscription_plans.up.sql

CREATE TABLE subscription_plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    monthly_price_rub NUMERIC(10, 2) NOT NULL DEFAULT 0,
    max_sms_per_month INTEGER NOT NULL DEFAULT 50000,
    max_smpp_connections INTEGER NOT NULL DEFAULT 1,
    max_users INTEGER NOT NULL DEFAULT 1,
    rate_limit_per_second INTEGER NOT NULL DEFAULT 10,
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 100,
    rate_limit_per_hour INTEGER NOT NULL DEFAULT 1000,
    rate_limit_per_day INTEGER NOT NULL DEFAULT 10000,
    features JSONB NOT NULL DEFAULT '{}',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Seed default plans
INSERT INTO subscription_plans (name, display_name, monthly_price_rub, max_sms_per_month, max_smpp_connections, max_users, rate_limit_per_second, rate_limit_per_minute, rate_limit_per_hour, rate_limit_per_day, features) VALUES
('free', 'Free / Trial', 0, 1000, 1, 1, 5, 50, 500, 1000, '{"analytics": false, "webhooks": false, "hlr": false, "smart_routing": false, "sub_accounts": false, "white_label": false}'),
('starter', 'Starter', 5000, 50000, 1, 1, 10, 100, 1000, 10000, '{"analytics": true, "webhooks": false, "hlr": false, "smart_routing": false, "sub_accounts": false, "white_label": false}'),
('business', 'Business', 15000, 300000, 5, 5, 50, 500, 5000, 50000, '{"analytics": true, "webhooks": true, "hlr": true, "smart_routing": true, "sub_accounts": false, "white_label": false}'),
('pro', 'Pro', 40000, 1500000, 100, 50, 200, 2000, 20000, 200000, '{"analytics": true, "webhooks": true, "hlr": true, "smart_routing": true, "sub_accounts": true, "white_label": false}');

-- Add plan_id to clients
ALTER TABLE clients ADD COLUMN plan_id UUID REFERENCES subscription_plans(id);

-- Set default plan (starter) for existing clients
UPDATE clients SET plan_id = (SELECT id FROM subscription_plans WHERE name = 'starter');

-- Make plan_id NOT NULL after backfill
ALTER TABLE clients ALTER COLUMN plan_id SET NOT NULL;

-- Add monthly usage tracking
ALTER TABLE clients ADD COLUMN monthly_sms_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE clients ADD COLUMN monthly_sms_reset_at TIMESTAMP WITH TIME ZONE DEFAULT date_trunc('month', NOW()) + INTERVAL '1 month';

CREATE INDEX idx_clients_plan_id ON clients(plan_id);
CREATE INDEX idx_subscription_plans_name ON subscription_plans(name);
CREATE INDEX idx_subscription_plans_active ON subscription_plans(active);
```

- [ ] **Step 2: Создать down-миграцию**

```sql
-- migrations/000030_create_subscription_plans.down.sql

ALTER TABLE clients DROP COLUMN IF EXISTS monthly_sms_reset_at;
ALTER TABLE clients DROP COLUMN IF EXISTS monthly_sms_count;
ALTER TABLE clients DROP COLUMN IF EXISTS plan_id;
DROP TABLE IF EXISTS subscription_plans;
```

- [ ] **Step 3: Применить миграцию локально**

Run: `cd /home/magomed/projects/sms && go run cmd/migrate/main.go up`
Expected: Migration 000030 applied successfully

- [ ] **Step 4: Commit**

```bash
git add migrations/000030_create_subscription_plans.up.sql migrations/000030_create_subscription_plans.down.sql
git commit -m "feat(billing): add subscription_plans table and link to clients"
```

---

## Task 2: Миграция — RLS-политики для изоляции данных

**Files:**
- Create: `migrations/000031_add_rls_policies.up.sql`
- Create: `migrations/000031_add_rls_policies.down.sql`

- [ ] **Step 1: Создать up-миграцию с RLS-политиками**

```sql
-- migrations/000031_add_rls_policies.up.sql

-- Enable RLS on tables with client data
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE client_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE pricing_rules ENABLE ROW LEVEL SECURITY;

-- Policy: messages — клиент видит только свои сообщения
CREATE POLICY messages_tenant_isolation ON messages
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: accounts — клиент видит только свой аккаунт
CREATE POLICY accounts_tenant_isolation ON accounts
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: transactions — клиент видит только свои транзакции
CREATE POLICY transactions_tenant_isolation ON transactions
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: client_configs — клиент видит только свои конфиги
CREATE POLICY client_configs_tenant_isolation ON client_configs
    USING (client_id = current_setting('app.current_client_id', true)::uuid);

-- Policy: pricing_rules — клиент видит свои правила + глобальные (client_id IS NULL)
CREATE POLICY pricing_rules_tenant_isolation ON pricing_rules
    USING (client_id = current_setting('app.current_client_id', true)::uuid OR client_id IS NULL);

-- Сервисный пользователь обходит RLS (для admin, миграций, pipeline worker)
-- Главный пользователь БД (sms_admin) должен быть superuser или владельцем таблиц
-- Приложение подключается через пользователя sms_app с ограниченными правами

-- Создаём роль для приложения если не существует
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'sms_app') THEN
        CREATE ROLE sms_app LOGIN PASSWORD 'sms_app_password';
    END IF;
END
$$;

-- Даём права на таблицы
GRANT SELECT, INSERT, UPDATE, DELETE ON messages TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON accounts TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON transactions TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON client_configs TO sms_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON pricing_rules TO sms_app;
GRANT SELECT ON subscription_plans TO sms_app;
GRANT SELECT, UPDATE ON clients TO sms_app;

-- RLS применяется к sms_app, но НЕ к владельцу таблиц (суперюзер)
-- Это значит: pipeline worker и admin service подключаются как суперюзер,
-- а client-facing gateways — через sms_app
```

- [ ] **Step 2: Создать down-миграцию**

```sql
-- migrations/000031_add_rls_policies.down.sql

DROP POLICY IF EXISTS messages_tenant_isolation ON messages;
DROP POLICY IF EXISTS accounts_tenant_isolation ON accounts;
DROP POLICY IF EXISTS transactions_tenant_isolation ON transactions;
DROP POLICY IF EXISTS client_configs_tenant_isolation ON client_configs;
DROP POLICY IF EXISTS pricing_rules_tenant_isolation ON pricing_rules;

ALTER TABLE messages DISABLE ROW LEVEL SECURITY;
ALTER TABLE accounts DISABLE ROW LEVEL SECURITY;
ALTER TABLE transactions DISABLE ROW LEVEL SECURITY;
ALTER TABLE client_configs DISABLE ROW LEVEL SECURITY;
ALTER TABLE pricing_rules DISABLE ROW LEVEL SECURITY;
```

- [ ] **Step 3: Применить миграцию**

Run: `cd /home/magomed/projects/sms && go run cmd/migrate/main.go up`
Expected: Migration 000031 applied successfully

- [ ] **Step 4: Commit**

```bash
git add migrations/000031_add_rls_policies.up.sql migrations/000031_add_rls_policies.down.sql
git commit -m "feat(security): add PostgreSQL RLS policies for tenant data isolation"
```

---

## Task 3: Доменная модель плана

**Files:**
- Create: `internal/services/client/domain/plan.go`
- Modify: `internal/services/client/domain/client.go`

- [ ] **Step 1: Создать доменную модель плана**

```go
// internal/services/client/domain/plan.go
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Plan struct {
	ID                 uuid.UUID
	Name               string
	DisplayName        string
	MonthlyPriceRub    float64
	MaxSMSPerMonth     int
	MaxSMPPConnections int
	MaxUsers           int
	RateLimitPerSecond int
	RateLimitPerMinute int
	RateLimitPerHour   int
	RateLimitPerDay    int
	Features           PlanFeatures
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PlanFeatures struct {
	Analytics   bool `json:"analytics"`
	Webhooks    bool `json:"webhooks"`
	HLR         bool `json:"hlr"`
	SmartRouting bool `json:"smart_routing"`
	SubAccounts bool `json:"sub_accounts"`
	WhiteLabel  bool `json:"white_label"`
}

func (p *Plan) HasFeature(feature string) bool {
	switch feature {
	case "analytics":
		return p.Features.Analytics
	case "webhooks":
		return p.Features.Webhooks
	case "hlr":
		return p.Features.HLR
	case "smart_routing":
		return p.Features.SmartRouting
	case "sub_accounts":
		return p.Features.SubAccounts
	case "white_label":
		return p.Features.WhiteLabel
	default:
		return false
	}
}

func (p *Plan) UnmarshalFeatures(data []byte) error {
	return json.Unmarshal(data, &p.Features)
}

func (p *Plan) MarshalFeatures() ([]byte, error) {
	return json.Marshal(p.Features)
}
```

- [ ] **Step 2: Добавить PlanID в модель Client**

В файле `internal/services/client/domain/client.go` добавить поля:

```go
// Добавить к структуре Client после поля MaxSubAccounts:
	PlanID           uuid.UUID
	MonthlySMSCount  int
	MonthlySMSResetAt time.Time
	Plan             *Plan // загружается через JOIN
```

- [ ] **Step 3: Добавить методы проверки квот к Client**

В файле `internal/services/client/domain/client.go` добавить методы:

```go
func (c *Client) IsWithinMonthlyQuota(additionalMessages int) bool {
	if c.Plan == nil {
		return true
	}
	return c.MonthlySMSCount+additionalMessages <= c.Plan.MaxSMSPerMonth
}

func (c *Client) RemainingMonthlyQuota() int {
	if c.Plan == nil {
		return 0
	}
	remaining := c.Plan.MaxSMSPerMonth - c.MonthlySMSCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (c *Client) NeedsMonthlyReset() bool {
	return time.Now().After(c.MonthlySMSResetAt)
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/client/domain/plan.go internal/services/client/domain/client.go
git commit -m "feat(client): add Plan domain model and quota methods to Client"
```

---

## Task 4: Репозиторий планов

**Files:**
- Create: `internal/services/client/infrastructure/repository/plan_repository.go`

- [ ] **Step 1: Создать репозиторий планов**

```go
// internal/services/client/infrastructure/repository/plan_repository.go
package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sms/internal/services/client/domain"
)

type PlanRepository struct {
	db *pgxpool.Pool
}

func NewPlanRepository(db *pgxpool.Pool) *PlanRepository {
	return &PlanRepository{db: db}
}

func (r *PlanRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Plan, error) {
	query := `SELECT id, name, display_name, monthly_price_rub, max_sms_per_month,
		max_smpp_connections, max_users, rate_limit_per_second, rate_limit_per_minute,
		rate_limit_per_hour, rate_limit_per_day, features, active, created_at, updated_at
		FROM subscription_plans WHERE id = $1`

	plan := &domain.Plan{}
	var featuresJSON []byte
	err := r.db.QueryRow(ctx, query, id).Scan(
		&plan.ID, &plan.Name, &plan.DisplayName, &plan.MonthlyPriceRub,
		&plan.MaxSMSPerMonth, &plan.MaxSMPPConnections, &plan.MaxUsers,
		&plan.RateLimitPerSecond, &plan.RateLimitPerMinute,
		&plan.RateLimitPerHour, &plan.RateLimitPerDay,
		&featuresJSON, &plan.Active, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get plan by id: %w", err)
	}
	if err := json.Unmarshal(featuresJSON, &plan.Features); err != nil {
		return nil, fmt.Errorf("unmarshal plan features: %w", err)
	}
	return plan, nil
}

func (r *PlanRepository) GetByName(ctx context.Context, name string) (*domain.Plan, error) {
	query := `SELECT id, name, display_name, monthly_price_rub, max_sms_per_month,
		max_smpp_connections, max_users, rate_limit_per_second, rate_limit_per_minute,
		rate_limit_per_hour, rate_limit_per_day, features, active, created_at, updated_at
		FROM subscription_plans WHERE name = $1`

	plan := &domain.Plan{}
	var featuresJSON []byte
	err := r.db.QueryRow(ctx, query, name).Scan(
		&plan.ID, &plan.Name, &plan.DisplayName, &plan.MonthlyPriceRub,
		&plan.MaxSMSPerMonth, &plan.MaxSMPPConnections, &plan.MaxUsers,
		&plan.RateLimitPerSecond, &plan.RateLimitPerMinute,
		&plan.RateLimitPerHour, &plan.RateLimitPerDay,
		&featuresJSON, &plan.Active, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get plan by name: %w", err)
	}
	if err := json.Unmarshal(featuresJSON, &plan.Features); err != nil {
		return nil, fmt.Errorf("unmarshal plan features: %w", err)
	}
	return plan, nil
}

func (r *PlanRepository) ListActive(ctx context.Context) ([]*domain.Plan, error) {
	query := `SELECT id, name, display_name, monthly_price_rub, max_sms_per_month,
		max_smpp_connections, max_users, rate_limit_per_second, rate_limit_per_minute,
		rate_limit_per_hour, rate_limit_per_day, features, active, created_at, updated_at
		FROM subscription_plans WHERE active = true ORDER BY monthly_price_rub ASC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list active plans: %w", err)
	}
	defer rows.Close()

	var plans []*domain.Plan
	for rows.Next() {
		plan := &domain.Plan{}
		var featuresJSON []byte
		err := rows.Scan(
			&plan.ID, &plan.Name, &plan.DisplayName, &plan.MonthlyPriceRub,
			&plan.MaxSMSPerMonth, &plan.MaxSMPPConnections, &plan.MaxUsers,
			&plan.RateLimitPerSecond, &plan.RateLimitPerMinute,
			&plan.RateLimitPerHour, &plan.RateLimitPerDay,
			&featuresJSON, &plan.Active, &plan.CreatedAt, &plan.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		if err := json.Unmarshal(featuresJSON, &plan.Features); err != nil {
			return nil, fmt.Errorf("unmarshal plan features: %w", err)
		}
		plans = append(plans, plan)
	}
	return plans, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/client/infrastructure/repository/plan_repository.go
git commit -m "feat(client): add PlanRepository for subscription plans CRUD"
```

---

## Task 5: Обновить Client Repository — загрузка плана через JOIN

**Files:**
- Modify: `internal/services/client/infrastructure/repository/client_repository.go`

- [ ] **Step 1: Обновить GetByID для загрузки плана**

В файле `client_repository.go` найти метод `GetByID` и обновить SQL-запрос, чтобы он включал JOIN с `subscription_plans`:

```go
func (r *ClientRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Client, error) {
	query := `SELECT c.id, c.name, c.email, c.contact_person, c.phone, c.active,
		c.metadata, c.created_at, c.updated_at, c.parent_client_id, c.is_reseller,
		c.max_sub_accounts, c.plan_id, c.monthly_sms_count, c.monthly_sms_reset_at,
		p.id, p.name, p.display_name, p.monthly_price_rub, p.max_sms_per_month,
		p.max_smpp_connections, p.max_users, p.rate_limit_per_second, p.rate_limit_per_minute,
		p.rate_limit_per_hour, p.rate_limit_per_day, p.features, p.active
		FROM clients c
		JOIN subscription_plans p ON c.plan_id = p.id
		WHERE c.id = $1`

	client := &domain.Client{}
	plan := &domain.Plan{}
	var featuresJSON []byte
	err := r.db.QueryRow(ctx, query, id).Scan(
		&client.ID, &client.Name, &client.Email, &client.ContactPerson,
		&client.Phone, &client.Active, &client.Metadata, &client.CreatedAt,
		&client.UpdatedAt, &client.ParentClientID, &client.IsReseller,
		&client.MaxSubAccounts, &client.PlanID, &client.MonthlySMSCount,
		&client.MonthlySMSResetAt,
		&plan.ID, &plan.Name, &plan.DisplayName, &plan.MonthlyPriceRub,
		&plan.MaxSMSPerMonth, &plan.MaxSMPPConnections, &plan.MaxUsers,
		&plan.RateLimitPerSecond, &plan.RateLimitPerMinute,
		&plan.RateLimitPerHour, &plan.RateLimitPerDay,
		&featuresJSON, &plan.Active,
	)
	if err != nil {
		return nil, fmt.Errorf("get client by id: %w", err)
	}
	if err := json.Unmarshal(featuresJSON, &plan.Features); err != nil {
		return nil, fmt.Errorf("unmarshal plan features: %w", err)
	}
	client.Plan = plan
	return client, nil
}
```

- [ ] **Step 2: Добавить метод IncrementMonthlySMSCount**

```go
func (r *ClientRepository) IncrementMonthlySMSCount(ctx context.Context, clientID uuid.UUID, count int) error {
	query := `UPDATE clients
		SET monthly_sms_count = CASE
			WHEN monthly_sms_reset_at <= NOW() THEN $2
			ELSE monthly_sms_count + $2
		END,
		monthly_sms_reset_at = CASE
			WHEN monthly_sms_reset_at <= NOW() THEN date_trunc('month', NOW()) + INTERVAL '1 month'
			ELSE monthly_sms_reset_at
		END,
		updated_at = NOW()
		WHERE id = $1`

	_, err := r.db.Exec(ctx, query, clientID, count)
	if err != nil {
		return fmt.Errorf("increment monthly sms count: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Добавить метод AssignPlan**

```go
func (r *ClientRepository) AssignPlan(ctx context.Context, clientID uuid.UUID, planID uuid.UUID) error {
	query := `UPDATE clients SET plan_id = $2, updated_at = NOW() WHERE id = $1`
	result, err := r.db.Exec(ctx, query, clientID, planID)
	if err != nil {
		return fmt.Errorf("assign plan: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("client not found: %s", clientID)
	}
	return nil
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/client/infrastructure/repository/client_repository.go
git commit -m "feat(client): load plan via JOIN, add plan assignment and usage tracking"
```

---

## Task 6: Rate-Limiting middleware — включить и привязать к плану

**Files:**
- Modify: `internal/api/middleware/ratelimit.go`

- [ ] **Step 1: Переписать rate-limit middleware**

Заменить содержимое `internal/api/middleware/ratelimit.go`:

```go
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type RateLimitConfig struct {
	PerSecond int
	PerMinute int
	PerHour   int
	PerDay    int
}

func RateLimitMiddleware(redisClient *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := GetClientID(r.Context())
			if clientID.String() == "00000000-0000-0000-0000-000000000000" {
				next.ServeHTTP(w, r)
				return
			}

			client := GetClient(r.Context())
			if client == nil || client.Plan == nil {
				next.ServeHTTP(w, r)
				return
			}

			limits := RateLimitConfig{
				PerSecond: client.Plan.RateLimitPerSecond,
				PerMinute: client.Plan.RateLimitPerMinute,
				PerHour:   client.Plan.RateLimitPerHour,
				PerDay:    client.Plan.RateLimitPerDay,
			}

			ctx := r.Context()
			cid := clientID.String()

			// Check all windows
			windows := []struct {
				suffix string
				limit  int
				window time.Duration
			}{
				{"sec", limits.PerSecond, time.Second},
				{"min", limits.PerMinute, time.Minute},
				{"hour", limits.PerHour, time.Hour},
				{"day", limits.PerDay, 24 * time.Hour},
			}

			for _, w := range windows {
				if w.limit <= 0 {
					continue
				}
				exceeded, err := checkWindow(ctx, redisClient, cid, w.suffix, w.limit, w.window)
				if err != nil {
					log.Error().Err(err).Str("client_id", cid).Msg("rate limit check failed")
					// Fail open — don't block on Redis errors
					break
				}
				if exceeded {
					http.Error(w, `{"error":"rate_limit_exceeded","window":"`+w.suffix+`"}`, http.StatusTooManyRequests)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func checkWindow(ctx context.Context, rdb *redis.Client, clientID, suffix string, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("rl:%s:%s", clientID, suffix)

	pipe := rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := incrCmd.Val()
	return count > int64(limit), nil
}
```

- [ ] **Step 2: Убедиться что `GetClient` возвращает клиента с Plan**

В `internal/api/middleware/auth.go` нужно обновить `GetClient` — он должен возвращать `*domain.Client` (с полем Plan), а не `*shared.Client`. Это будет сделано в Task 7.

- [ ] **Step 3: Commit**

```bash
git add internal/api/middleware/ratelimit.go
git commit -m "feat(middleware): rewrite rate-limit middleware to use plan-based limits"
```

---

## Task 7: Auth middleware — включить и загружать клиента с планом

**Files:**
- Modify: `internal/api/middleware/auth.go`
- Modify: `internal/gateway/client/middleware/auth.go`

- [ ] **Step 1: Обновить API auth middleware**

В `internal/api/middleware/auth.go` убрать заглушку и включить реальную авторизацию. Middleware должен:
1. Извлечь API-ключ из заголовка `Authorization`
2. Валидировать через Auth Service (gRPC)
3. Загрузить клиента с планом через Client Service (gRPC)
4. Положить в контекст: ClientID, Client (с Plan)

```go
// Обновить тип контекста для Client — он теперь содержит Plan
// Ключевое изменение: вместо shared.Client используем структуру с Plan

type ClientContext struct {
	ID                uuid.UUID
	Plan              *PlanContext
	MonthlySMSCount   int
	MonthlySMSResetAt time.Time
	Active            bool
}

type PlanContext struct {
	Name               string
	MaxSMSPerMonth     int
	MaxSMPPConnections int
	RateLimitPerSecond int
	RateLimitPerMinute int
	RateLimitPerHour   int
	RateLimitPerDay    int
	Features           map[string]bool
}
```

Конкретные изменения зависят от текущей реализации auth middleware — нужно:
- Убрать строку `// LOAD TESTING MODE` и заглушку с dummy UUID
- Раскомментировать или написать реальную валидацию
- Добавить загрузку Plan из Client Service gRPC response

- [ ] **Step 2: Аналогично обновить Client Gateway auth middleware**

В `internal/gateway/client/middleware/auth.go` — та же логика: убрать заглушку, включить реальную авторизацию с загрузкой плана.

- [ ] **Step 3: Commit**

```bash
git add internal/api/middleware/auth.go internal/gateway/client/middleware/auth.go
git commit -m "feat(auth): enable real auth middleware with plan context loading"
```

---

## Task 8: Quota middleware — проверка месячного лимита SMS

**Files:**
- Create: `internal/middleware/quota.go`

- [ ] **Step 1: Создать middleware проверки квот**

```go
// internal/middleware/quota.go
package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func QuotaMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := GetClient(r.Context())
			if client == nil || client.Plan == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Auto-reset monthly counter
			monthlySMSCount := client.MonthlySMSCount
			if time.Now().After(client.MonthlySMSResetAt) {
				monthlySMSCount = 0
			}

			if monthlySMSCount >= client.Plan.MaxSMSPerMonth {
				log.Warn().
					Str("client_id", client.ID.String()).
					Int("count", monthlySMSCount).
					Int("limit", client.Plan.MaxSMSPerMonth).
					Msg("monthly SMS quota exceeded")

				http.Error(w, `{"error":"monthly_quota_exceeded","limit":`+
					fmt.Sprintf("%d", client.Plan.MaxSMSPerMonth)+
					`,"used":`+fmt.Sprintf("%d", monthlySMSCount)+`}`,
					http.StatusPaymentRequired)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/middleware/quota.go
git commit -m "feat(middleware): add monthly SMS quota enforcement middleware"
```

---

## Task 9: Обновить proto и gRPC — Plan в ClientInfo

**Files:**
- Modify: `api/proto/client/client.proto`

- [ ] **Step 1: Добавить Plan message и поля в proto**

В `api/proto/client/client.proto` добавить:

```protobuf
message SubscriptionPlan {
  string id = 1;
  string name = 2;
  string display_name = 3;
  double monthly_price_rub = 4;
  int32 max_sms_per_month = 5;
  int32 max_smpp_connections = 6;
  int32 max_users = 7;
  RateLimits rate_limits = 8;
  map<string, bool> features = 9;
  bool active = 10;
}

// Добавить в ClientInfo:
//   string plan_id = 14;
//   SubscriptionPlan plan = 15;
//   int32 monthly_sms_count = 16;
//   string monthly_sms_reset_at = 17;

// Добавить в service ClientService:
//   rpc ListPlans(ListPlansRequest) returns (ListPlansResponse);
//   rpc AssignPlan(AssignPlanRequest) returns (AssignPlanResponse);

message ListPlansRequest {}
message ListPlansResponse {
  repeated SubscriptionPlan plans = 1;
}

message AssignPlanRequest {
  string client_id = 1;
  string plan_id = 2;
}

message AssignPlanResponse {
  bool success = 1;
}
```

- [ ] **Step 2: Сгенерировать Go-код из proto**

Run: `cd /home/magomed/projects/sms && protoc --go_out=. --go-grpc_out=. api/proto/client/client.proto`
Expected: Generated files updated in `api/proto/client/`

- [ ] **Step 3: Commit**

```bash
git add api/proto/client/
git commit -m "feat(proto): add SubscriptionPlan to client proto, add ListPlans and AssignPlan RPCs"
```

---

## Task 10: Подключить middleware к роутеру

**Files:**
- Modify: файл, где настраиваются HTTP routes для client gateway (найти по паттерну `r.Use(` или `router.Use(`)

- [ ] **Step 1: Найти файл настройки роутера**

Run: `grep -rn "router.Use\|r.Use\|mux.Use" /home/magomed/projects/sms/internal/gateway/client/ /home/magomed/projects/sms/cmd/`

- [ ] **Step 2: Добавить middleware в цепочку**

Порядок middleware должен быть:
1. `AuthMiddleware` — аутентификация, загрузка клиента с планом
2. `RateLimitMiddleware` — проверка rate limits по плану
3. `QuotaMiddleware` — проверка месячной квоты
4. Handler

```go
// Пример подключения:
router.Use(middleware.AuthMiddleware(authClient, clientClient))
router.Use(middleware.RateLimitMiddleware(redisClient))
router.Use(middleware.QuotaMiddleware())
```

- [ ] **Step 3: Commit**

```bash
git add cmd/ internal/gateway/
git commit -m "feat(gateway): wire auth, rate-limit, and quota middleware into client gateway"
```

---

## Task 11: Redis-трекинг использования в реальном времени

**Files:**
- Create: `internal/services/client/infrastructure/usage_tracker.go`

- [ ] **Step 1: Создать трекер использования на Redis**

```go
// internal/services/client/infrastructure/usage_tracker.go
package infrastructure

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type UsageTracker struct {
	redis *redis.Client
}

func NewUsageTracker(redis *redis.Client) *UsageTracker {
	return &UsageTracker{redis: redis}
}

// IncrementSMSCount увеличивает счётчик SMS за текущий месяц.
// Возвращает новое значение счётчика.
func (t *UsageTracker) IncrementSMSCount(ctx context.Context, clientID uuid.UUID, count int) (int64, error) {
	key := t.monthlyKey(clientID)
	pipe := t.redis.Pipeline()
	incrCmd := pipe.IncrBy(ctx, key, int64(count))
	// TTL = до конца месяца + 1 день (safety margin)
	pipe.ExpireNX(ctx, key, t.timeUntilEndOfMonth()+24*time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("increment sms count: %w", err)
	}
	return incrCmd.Val(), nil
}

// GetMonthlySMSCount возвращает текущий счётчик SMS за месяц.
func (t *UsageTracker) GetMonthlySMSCount(ctx context.Context, clientID uuid.UUID) (int, error) {
	key := t.monthlyKey(clientID)
	val, err := t.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get monthly sms count: %w", err)
	}
	count, _ := strconv.Atoi(val)
	return count, nil
}

func (t *UsageTracker) monthlyKey(clientID uuid.UUID) string {
	now := time.Now()
	return fmt.Sprintf("usage:%s:sms:%d-%02d", clientID.String(), now.Year(), now.Month())
}

func (t *UsageTracker) timeUntilEndOfMonth() time.Duration {
	now := time.Now()
	endOfMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	return endOfMonth.Sub(now)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/client/infrastructure/usage_tracker.go
git commit -m "feat(client): add Redis-based monthly SMS usage tracker"
```

---

## Task 12: Интеграционный тест — RLS-изоляция

**Files:**
- Create: `tests/integration/rls_test.go`

- [ ] **Step 1: Написать тест RLS-изоляции**

```go
// tests/integration/rls_test.go
package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRLSIsolation_MessagesTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := getTestDB(t) // helper to connect to test DB as sms_app role
	ctx := context.Background()

	clientA := uuid.New()
	clientB := uuid.New()

	// Insert messages for two different clients (as superuser)
	adminDB := getAdminDB(t)
	_, err := adminDB.Exec(ctx, `INSERT INTO messages (id, client_id, source, destination, body, status)
		VALUES ($1, $2, 'sender', '+79001234567', 'hello from A', 'pending')`, uuid.New(), clientA)
	require.NoError(t, err)

	_, err = adminDB.Exec(ctx, `INSERT INTO messages (id, client_id, source, destination, body, status)
		VALUES ($1, $2, 'sender', '+79001234568', 'hello from B', 'pending')`, uuid.New(), clientB)
	require.NoError(t, err)

	// Set RLS context to client A
	_, err = db.Exec(ctx, "SET app.current_client_id = $1", clientA.String())
	require.NoError(t, err)

	// Client A should only see their own messages
	var count int
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM messages").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Client A should see only 1 message")

	// Set RLS context to client B
	_, err = db.Exec(ctx, "SET app.current_client_id = $1", clientB.String())
	require.NoError(t, err)

	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM messages").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Client B should see only 1 message")
}
```

- [ ] **Step 2: Запустить тест**

Run: `cd /home/magomed/projects/sms && go test ./tests/integration/ -run TestRLSIsolation -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add tests/integration/rls_test.go
git commit -m "test(integration): add RLS tenant isolation test for messages table"
```

---

## Task 13: Интеграционный тест — Rate Limiting

**Files:**
- Create: `tests/integration/ratelimit_test.go`

- [ ] **Step 1: Написать тест rate limiting**

```go
// tests/integration/ratelimit_test.go
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sms/internal/api/middleware"
)

func TestRateLimit_ExceedsPerSecondLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	rdb := getTestRedis(t)
	ctx := context.Background()
	clientID := uuid.New().String()

	// Clean up
	defer rdb.FlushDB(ctx)

	limit := 5
	for i := 0; i < limit; i++ {
		exceeded, err := middleware.CheckWindow(ctx, rdb, clientID, "sec", limit, time.Second)
		require.NoError(t, err)
		assert.False(t, exceeded, "request %d should not be rate limited", i+1)
	}

	// Next request should be rate limited
	exceeded, err := middleware.CheckWindow(ctx, rdb, clientID, "sec", limit, time.Second)
	require.NoError(t, err)
	assert.True(t, exceeded, "request %d should be rate limited", limit+1)

	// After 1 second, should be allowed again
	time.Sleep(1100 * time.Millisecond)
	exceeded, err = middleware.CheckWindow(ctx, rdb, clientID, "sec", limit, time.Second)
	require.NoError(t, err)
	assert.False(t, exceeded, "request after window reset should not be limited")
}
```

- [ ] **Step 2: Запустить тест**

Run: `cd /home/magomed/projects/sms && go test ./tests/integration/ -run TestRateLimit -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add tests/integration/ratelimit_test.go
git commit -m "test(integration): add rate limiting integration tests"
```

---

## Summary

| Task | Описание | Файлы |
|------|----------|-------|
| 1 | Миграция: таблица subscription_plans | 2 файла миграции |
| 2 | Миграция: RLS-политики | 2 файла миграции |
| 3 | Доменная модель Plan + обновление Client | 2 файла |
| 4 | Репозиторий планов | 1 файл |
| 5 | Обновление Client Repository (JOIN + usage) | 1 файл |
| 6 | Rate-limiting middleware (plan-aware) | 1 файл |
| 7 | Auth middleware (включить + план в контекст) | 2 файла |
| 8 | Quota middleware (месячный лимит) | 1 файл |
| 9 | Proto: SubscriptionPlan + RPC | proto файлы |
| 10 | Подключение middleware к роутеру | cmd + gateway |
| 11 | Redis usage tracker | 1 файл |
| 12 | Интеграционный тест RLS | 1 файл |
| 13 | Интеграционный тест Rate Limiting | 1 файл |

# Aggregator Routing — Plan 4: Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть production-readiness блокеры aggregator routing'а: Redis hardening, retry для committed-but-unmaterialized SRA-rows, frontend rendering warnings, аудит UI для route-set/provider-set/assignment операций.

**Architecture:** Изменения распределены по 4 независимым областям: (1) AUTH на Redis-клиентах через единый ENV `REDIS_PASSWORD` + миграция SRA для retry-state; (2) экстракция дублирующейся материализационной логики в helper и вызов того же helper'а из нового cron-ретраера; (3) UI рендер warnings из PutOne/Bulk responses; (4) audit-write при mutation в network-handler'ах + новый GET endpoint с фильтром по `resource_type` + страница «История изменений».

**Tech Stack:** Go 1.24 (gorilla/mux, redis/go-redis/v9, jackc/pgx/v5, rs/zerolog, prometheus), TypeScript 5.7 + React 19 (Vite, Tailwind, Radix), PostgreSQL 15 partitioned `audit_log`.

**Spec reference:** `docs/superpowers/specs/2026-05-04-aggregator-routing-management-design.md` §6.7 (audit-log UI), §6.4 (materialize partial-failure recovery).

**Memory:**
- `project_redis_hijack_2026_05_04` — sandbox Redis hijack обоснование Section 1.
- `feedback_review_gate_no_skip` — review-gate non-negotiable per task.
- `feedback_git_status_precommit` — никогда `git add .`.
- `feedback_verify_before_asserting` — read source before writing assertions.
- `project_server_is_sandbox` — миграции/деплой свободны на sandbox.

---

## Section 1 — Redis hardening (D13)

**Контекст.** Sandbox Redis 2026-05-04 был hijack'нут внешним хостом, hot-fix `REPLICAOF NO ONE`. Сейчас:
- `cmd/portal-gateway/main.go:109` и `cmd/services/auth-service/main.go:125` уже читают `REDIS_PASSWORD`.
- `internal/shared/cache/cache.go:42,82` использует `cfg.Password` из `RedisConfig`.
- Остальные 9 main'ов (`admin-gateway`, `api`, `client-gateway`, `pipeline-worker`, `cascade-service`, `link-service`, `network-analytics-service`, `routing-service`, `worker`) подключаются БЕЗ пароля.

Цель: единая точка чтения `REDIS_ADDR` + `REDIS_PASSWORD` + `REDIS_DB` + firewall-only-localhost на sandbox.

### Task 1: Redis-config helper в `internal/config`

**Files:**
- Create: `internal/config/redis_env.go`
- Create: `internal/config/redis_env_test.go`

- [ ] **Step 1: Написать failing test**

`internal/config/redis_env_test.go`:

```go
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedisOptionsFromEnv_Defaults(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "localhost:6379", opts.Addr)
	assert.Empty(t, opts.Password)
	assert.Equal(t, 0, opts.DB)
}

func TestRedisOptionsFromEnv_FromVars(t *testing.T) {
	t.Setenv("REDIS_ADDR", "redis.internal:6380")
	t.Setenv("REDIS_PASSWORD", "s3cret")
	t.Setenv("REDIS_DB", "3")

	opts := RedisOptionsFromEnv()
	assert.Equal(t, "redis.internal:6380", opts.Addr)
	assert.Equal(t, "s3cret", opts.Password)
	assert.Equal(t, 3, opts.DB)
}

func TestRedisOptionsFromEnv_InvalidDB_Defaults(t *testing.T) {
	t.Setenv("REDIS_DB", "not-a-number")
	opts := RedisOptionsFromEnv()
	assert.Equal(t, 0, opts.DB)
}
```

- [ ] **Step 2: Запустить test для подтверждения FAIL**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/config/..."
```

Expected: FAIL `undefined: RedisOptionsFromEnv`.

- [ ] **Step 3: Реализация**

`internal/config/redis_env.go`:

```go
package config

import (
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// RedisOptionsFromEnv reads REDIS_ADDR / REDIS_PASSWORD / REDIS_DB and returns
// a populated *redis.Options. Empty REDIS_ADDR defaults to "localhost:6379".
// Empty/invalid REDIS_DB defaults to 0. REDIS_PASSWORD must be set in any
// environment that exposes Redis to a non-loopback interface (see
// project_redis_hijack_2026_05_04).
func RedisOptionsFromEnv() *redis.Options {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	db := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			db = parsed
		}
	}
	return &redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	}
}
```

- [ ] **Step 4: Запустить test для PASS**

Та же команда. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/redis_env.go internal/config/redis_env_test.go
git commit -m "feat(config): add RedisOptionsFromEnv helper for unified REDIS_ADDR/PASSWORD/DB reading

Plan 4 Task 1 — single source of truth для Redis-аутентификации, готовится миграция всех main'ов на этот helper для закрытия sandbox-hijack-вектора."
```

### Task 2: Перевести все main'ы на `RedisOptionsFromEnv`

**Files:** modify
- `cmd/admin-gateway/main.go:147` (replace `redis.NewClient(&redis.Options{Addr: ...})` block)
- `cmd/api/main.go:66`
- `cmd/client-gateway/main.go:117`
- `cmd/pipeline-worker/main.go:102`
- `cmd/portal-gateway/main.go:107-122` (заменить ручное чтение env)
- `cmd/services/auth-service/main.go:123` (заменить ручное чтение env)
- `cmd/services/cascade-service/main.go:71`
- `cmd/services/link-service/main.go:49`
- `cmd/services/network-analytics-service/main.go:51` (тут уже есть `redisOpts` — использовать helper)
- `cmd/services/routing-service/main.go:99`
- `cmd/worker/main.go:178`

**НЕ трогать**: `internal/shared/cache/cache.go` (использует `cfg.Password` из YAML config — корректный путь для сервисов на viper-config); тестовые файлы с `miniredis`.

- [ ] **Step 1: Verify — прочитать каждый main и убедиться, что замена корректна**

Прочитать каждый из 11 файлов в области изменения. Убедиться что:
- Импорт `github.com/smpp-server/smpp-server/internal/config` уже есть или добавлен
- `redis.NewClient(...)` принимает `*redis.Options` (а не embedded literal)
- Никакие custom-fields (PoolSize, MinIdleConns, и т.п.) не теряются — у этих 11 main'ов их нет

- [ ] **Step 2: Replace в каждом main**

Pattern (admin-gateway):

```go
redisDSN := config.EnvOrDefault("REDIS_ADDR", "localhost:6379")
redisClient := redis.NewClient(&redis.Options{
    Addr: redisDSN,
})
```

→

```go
redisClient := redis.NewClient(config.RedisOptionsFromEnv())
```

Для `portal-gateway` (lines 107-124) убрать также:
- `redisAddr := config.EnvOrDefault("REDIS_ADDR", ...)`
- `redisPassword := config.EnvOrDefault("REDIS_PASSWORD", ...)`
- `redisDB` парсинг
- `redis.Options{Addr/Password/DB}` literal

Заменить на одну строку `redisClient := redis.NewClient(config.RedisOptionsFromEnv())`. Сохранить `defer redisClient.Close()` и `logger.Info().Msg(...)`.

Для `auth-service` (line 123-127) — аналогично, заменить literal с `os.Getenv("REDIS_PASSWORD")` на helper.

- [ ] **Step 3: Build на сервере**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T dev go build -buildvcs=false ./cmd/..."
```

Expected: zero errors.

- [ ] **Step 4: Existing-tests на сервере**

```bash
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/config/... ./internal/shared/cache/..."
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/admin-gateway/main.go cmd/api/main.go cmd/client-gateway/main.go cmd/pipeline-worker/main.go cmd/portal-gateway/main.go cmd/services/auth-service/main.go cmd/services/cascade-service/main.go cmd/services/link-service/main.go cmd/services/network-analytics-service/main.go cmd/services/routing-service/main.go cmd/worker/main.go
git commit -m "feat(redis): unify all services on config.RedisOptionsFromEnv

Plan 4 Task 2 — устраняет 9 main'ов, которые подключались к Redis без пароля.
Все service binaries теперь читают REDIS_PASSWORD из env. Подготовка к Task 3 (включение requirepass на sandbox)."
```

### Task 3: Включить requirepass на sandbox + ENV в docker-compose + smoke

**Files:** modify
- `deployments/docker-compose.yml` — добавить `command: redis-server --requirepass ${REDIS_PASSWORD}` к redis service; пробросить `REDIS_PASSWORD` ко всем app-сервисам как env var
- `deployments/.env.example` (или создать если нет) — задокументировать `REDIS_PASSWORD=changeme`

- [ ] **Step 1: Прочитать docker-compose.yml**

```bash
grep -n "redis\|REDIS" deployments/docker-compose.yml
```

Понять текущую структуру — найти redis service block, найти все services с REDIS_ADDR в env. Зафиксировать список.

- [ ] **Step 2: Сгенерировать сильный пароль**

На server-side (НЕ в репо):

```bash
./scripts/server.sh exec "openssl rand -base64 32"
```

Скопировать результат — это `REDIS_PASSWORD`. Записать в /opt/sms/.env на сервере как `REDIS_PASSWORD=<...>`. **НЕ коммитить пароль в git.**

- [ ] **Step 3: Modify docker-compose.yml**

Найти redis service, добавить:
```yaml
  redis:
    image: redis:7-alpine
    command: ["redis-server", "--requirepass", "${REDIS_PASSWORD:?REDIS_PASSWORD must be set}"]
    # ... existing config
```

К каждому app-сервису, у которого уже есть `REDIS_ADDR`, добавить:
```yaml
      REDIS_PASSWORD: ${REDIS_PASSWORD:?REDIS_PASSWORD must be set}
```

Список app-сервисов проверить grep'ом из Step 1.

- [ ] **Step 4: Modify .env.example**

Если файл существует — добавить строку `REDIS_PASSWORD=changeme-generate-via-openssl-rand-base64-32`. Если нет — создать с одной этой строкой плюс комментарием.

- [ ] **Step 5: Push, deploy, smoke**

```bash
git add deployments/docker-compose.yml deployments/.env.example
git commit -m "feat(deploy): require REDIS_PASSWORD via docker-compose interpolation

Plan 4 Task 3 — docker-compose теперь fail-fast если REDIS_PASSWORD не установлен.
На sandbox-server файл .env обновлён вручную с openssl-rand-сгенерированным паролем."

git push origin master
./scripts/server.sh deploy
```

После деплоя:

```bash
./scripts/server.sh exec "docker compose -f /opt/sms/deployments/docker-compose.yml ps redis"
./scripts/server.sh exec "docker compose -f /opt/sms/deployments/docker-compose.yml exec -T redis redis-cli ping"
```

Expected: `ping` без `AUTH` → `(error) NOAUTH Authentication required`. С `-a $REDIS_PASSWORD` → `PONG`.

- [ ] **Step 6: Прогнать UI smoke**

Логин в портал aggregator@test.local / Admin123! на http://72.56.232.202:18085/ — убедиться что session работает (sessions хранятся в Redis). Если не пускает → откатить task'и Section 1, диагностировать.

- [ ] **Step 7: Firewall rule на 6379**

```bash
./scripts/server.sh exec "sudo iptables -I INPUT -p tcp --dport 6379 ! -s 127.0.0.1 -j DROP"
./scripts/server.sh exec "sudo iptables -I INPUT -p tcp --dport 6379 -i docker0 -j ACCEPT"
```

(Если iptables не настроен — `ufw deny 6379` или skip; зафиксировать в DONE.)

Verify извне: `redis-cli -h 72.56.232.202 ping` с локальной машины должен timeout/refuse.

- [ ] **Step 8: Commit log/firewall noting (no secrets)**

Никаких файлов добавлять не нужно — этот шаг только подтверждает оперативное действие. В DONE-документе зафиксировать применённое правило.

---

## Section 2 — Materialize helper + cron retry (A3 + hygiene)

**Контекст.** Plan 3 Task 4 заменил 500 на 200+warnings при materialize-сбое — UX починен. Реальная консистентность не восстанавливается: SRA committed, `client_providers`/`client_routes` пустые, sub-account шлёт SMS → reject. Frontend ретрайт идемпотентен, но требует пользователя.

Решение: (a) колонка `last_materialize_error_at` в SRA; (b) общий helper `applyAssignmentMaterializers` для PutOne+Bulk (устраняет 4-кратное дублирование из `network_assignments.go:319-334` и `:447-457`); (c) фоновый cron-ретраер в существующем `cmd/worker/main.go` (или отдельной goroutine), который раз в N минут читает SRA с `last_materialize_error_at IS NOT NULL`, повторяет helper, очищает column при успехе.

### Task 4: Миграция SRA `last_materialize_error_at`

**Files:**
- Create: `migrations/000142_sra_last_materialize_error.up.sql`
- Create: `migrations/000142_sra_last_materialize_error.down.sql`

- [ ] **Step 1: up.sql**

```sql
ALTER TABLE subaccount_routing_assignment
    ADD COLUMN last_materialize_error_at TIMESTAMPTZ,
    ADD COLUMN last_materialize_error_text TEXT,
    ADD COLUMN materialize_retry_count INT NOT NULL DEFAULT 0;

CREATE INDEX idx_sra_pending_retry
    ON subaccount_routing_assignment (last_materialize_error_at)
    WHERE last_materialize_error_at IS NOT NULL;
```

- [ ] **Step 2: down.sql**

```sql
DROP INDEX IF EXISTS idx_sra_pending_retry;
ALTER TABLE subaccount_routing_assignment
    DROP COLUMN IF EXISTS materialize_retry_count,
    DROP COLUMN IF EXISTS last_materialize_error_text,
    DROP COLUMN IF EXISTS last_materialize_error_at;
```

- [ ] **Step 3: Apply migration на sandbox**

```bash
git add migrations/000142_sra_last_materialize_error.up.sql migrations/000142_sra_last_materialize_error.down.sql
git commit -m "migrate: SRA last_materialize_error_at column for retry-state (Plan 4 Task 4)"
git push origin master
./scripts/server.sh sync
./scripts/server.sh migrate
./scripts/server.sh exec "docker compose -f /opt/sms/deployments/docker-compose.yml exec -T postgres psql -U smpp -d smpp_db -c \"\\d subaccount_routing_assignment\""
```

Expected: вывод `\d` показывает 3 новые колонки + индекс.

### Task 5: Helper `applyAssignmentMaterializers` + миграция call-sites

**Files:**
- Create: `internal/services/network/assignment_apply.go`
- Create: `internal/services/network/assignment_apply_test.go`
- Modify: `internal/gateway/portal/handlers/network_assignments.go:319-334` (PutOne)
- Modify: `internal/gateway/portal/handlers/network_assignments.go:447-457` (Bulk)

- [ ] **Step 1: Failing test**

`internal/services/network/assignment_apply_test.go`:

```go
package network

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

type stubProviderMat struct{ err error }

func (s stubProviderMat) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	return s.err
}

type stubRouteMat struct{ err error }

func (s stubRouteMat) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	return s.err
}

func TestApplyAssignmentMaterializers_BothSucceed_ClearsError(t *testing.T) {
	pool := storagetest.MustOpenPool(t)
	defer pool.Close()
	clientID := storagetest.SeedSubAccount(t, pool)
	storagetest.SeedSRAErrorState(t, pool, clientID, "old failure")

	w := ApplyAssignmentMaterializers(context.Background(), pool, stubProviderMat{}, stubRouteMat{}, clientID, nil, nil)
	assert.Empty(t, w)

	var errAt *string
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT last_materialize_error_text FROM subaccount_routing_assignment WHERE client_id=$1",
		clientID,
	).Scan(&errAt))
	assert.Nil(t, errAt)
}

func TestApplyAssignmentMaterializers_ProviderFails_RecordsError(t *testing.T) {
	pool := storagetest.MustOpenPool(t)
	defer pool.Close()
	clientID := storagetest.SeedSubAccount(t, pool)

	w := ApplyAssignmentMaterializers(context.Background(), pool,
		stubProviderMat{err: errors.New("boom")}, stubRouteMat{},
		clientID, nil, nil)
	require.Len(t, w, 1)
	assert.Equal(t, "provider_materialize", w[0]["step"])

	var errText *string
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT last_materialize_error_text FROM subaccount_routing_assignment WHERE client_id=$1",
		clientID,
	).Scan(&errText))
	require.NotNil(t, errText)
	assert.Contains(t, *errText, "boom")
}

func TestApplyAssignmentMaterializers_BothFail_RecordsBoth(t *testing.T) {
	pool := storagetest.MustOpenPool(t)
	defer pool.Close()
	clientID := storagetest.SeedSubAccount(t, pool)

	w := ApplyAssignmentMaterializers(context.Background(), pool,
		stubProviderMat{err: errors.New("p")},
		stubRouteMat{err: errors.New("r")},
		clientID, nil, nil)
	assert.Len(t, w, 2)
}

var _ = pgxpool.Pool{} // keep import if unused
```

`storagetest.SeedSubAccount` и `SeedSRAErrorState` — добавить в `internal/storage/storagetest/fixtures.go` (новые helper'ы; реализация по аналогии с существующими).

- [ ] **Step 2: Прочитать существующий fixtures.go перед добавлением helper'ов**

Read: `internal/storage/storagetest/fixtures.go` (полный файл) — найти как сейчас инициализируется pool, какие helper'ы доступны (`MustOpenPool` или эквивалент, `SeedClient`, `SeedReseller`, и т.д.). НЕ дублировать существующее. Если pool-helper называется иначе (e.g. `OpenTestPool`, `NewPool`) — обновить test'ы в Step 1 на правильное имя ПЕРЕД запуском.

- [ ] **Step 3: Реализовать helper и storagetest-fixtures**

`internal/services/network/assignment_apply.go`:

```go
// Package network provides shared logic for sub-account routing assignments.
package network

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/smpp-server/smpp-server/internal/monitoring/portal"
)

// ProviderApplier и RouteApplier — minimal-interfaces для тестов.
// Совпадают с network.ProviderMaterializer / RouteMaterializer (Plan 3 Task 4).
type ProviderApplier interface {
	ApplyToClient(ctx context.Context, clientID uuid.UUID, providerSetID *uuid.UUID) error
}
type RouteApplier interface {
	ApplyToClient(ctx context.Context, clientID uuid.UUID, routeSetID *uuid.UUID) error
}

// ApplyAssignmentMaterializers вызывает обоих materializer'ов и обновляет
// retry-state в subaccount_routing_assignment. Контракт:
//   - оба success → сбросить last_materialize_error_at/text/retry_count, warnings = nil.
//   - любой fail → записать ошибку, инкрементировать retry_count, вернуть warnings.
//   - 200-семантика: caller всегда возвращает 200, warnings — для UI и cron.
//
// Helper заменяет 4 дубликата из network_assignments.go (PutOne provider/route и
// Bulk provider/route), которые накопились в Plan 3 Task 4.
func ApplyAssignmentMaterializers(
	ctx context.Context,
	pool *pgxpool.Pool,
	pm ProviderApplier,
	rm RouteApplier,
	clientID uuid.UUID,
	providerSetID *uuid.UUID,
	routeSetID *uuid.UUID,
) []map[string]string {
	warnings := []map[string]string{}
	if err := pm.ApplyToClient(ctx, clientID, providerSetID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("provider materialize partial-failure")
		portal.MaterializeFailureTotal.WithLabelValues("provider").Inc()
		warnings = append(warnings, map[string]string{"step": "provider_materialize", "error": err.Error()})
	}
	if err := rm.ApplyToClient(ctx, clientID, routeSetID); err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("route materialize partial-failure")
		portal.MaterializeFailureTotal.WithLabelValues("route").Inc()
		warnings = append(warnings, map[string]string{"step": "route_materialize", "error": err.Error()})
	}

	if len(warnings) == 0 {
		_, err := pool.Exec(ctx, `
			UPDATE subaccount_routing_assignment
			   SET last_materialize_error_at = NULL,
			       last_materialize_error_text = NULL,
			       materialize_retry_count = 0
			 WHERE client_id = $1`, clientID)
		if err != nil {
			log.Warn().Err(err).Str("client_id", clientID.String()).Msg("clear retry-state failed")
		}
		return nil
	}

	errText := ""
	for i, w := range warnings {
		if i > 0 {
			errText += "; "
		}
		errText += w["step"] + ": " + w["error"]
	}
	if _, err := pool.Exec(ctx, `
		UPDATE subaccount_routing_assignment
		   SET last_materialize_error_at = now(),
		       last_materialize_error_text = $2,
		       materialize_retry_count = materialize_retry_count + 1
		 WHERE client_id = $1`, clientID, errText); err != nil {
		log.Warn().Err(err).Str("client_id", clientID.String()).Msg("record retry-state failed")
	}
	return warnings
}
```

В `internal/storage/storagetest/fixtures.go` добавить (если не существует):

```go
// SeedSubAccount creates a minimal sub-account row + SRA row (NULL set IDs)
// and returns the client_id. Test cleanup deletes via existing TX rollback.
func SeedSubAccount(t *testing.T, pool *pgxpool.Pool) uuid.UUID { /* impl */ }

// SeedSRAErrorState pre-populates last_materialize_error_text on existing SRA.
func SeedSRAErrorState(t *testing.T, pool *pgxpool.Pool, clientID uuid.UUID, errText string) { /* impl */ }
```

(Если pool-helper отсутствует — использовать тот же паттерн, что и существующие fixtures.)

- [ ] **Step 4: Заменить 4 inline materialize-блока на вызов helper'а**

В `internal/gateway/portal/handlers/network_assignments.go`:

PutOne (`:318-340`) — заменить блок `warnings := []map[string]string{}; if err := h.providerMat...` ... `if len(warnings) > 0 { resp["warnings"] = warnings }` на:

```go
warnings := network.ApplyAssignmentMaterializers(r.Context(), h.pool, h.providerMat, h.routeMat, clientID, psUUID, rsUUID)
resp := map[string]interface{}{"client_id": clientID.String()}
if len(warnings) > 0 {
    resp["warnings"] = warnings
}
respondJSON(w, http.StatusOK, resp)
```

Bulk (`:447-462`) — аналогичная замена 11-line блока на:

```go
warnings := network.ApplyAssignmentMaterializers(r.Context(), h.pool, h.providerMat, h.routeMat, cid, psUUID, rsUUID)
status := "ok"
if len(warnings) > 0 {
    status = "partial"
}
results = append(results, bulkResultItem{ClientID: idStr, Status: status, Warnings: warnings})
```

Импорт `"github.com/smpp-server/smpp-server/internal/services/network"` добавить если ещё нет.

- [ ] **Step 5: Run tests на сервере**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/... ./internal/gateway/portal/handlers/..."
```

Expected: PASS — новые tests + существующие `TestPutOne_ProviderMaterializeFailure_Returns200WithWarning` и Bulk-вариант.

- [ ] **Step 6: Commit**

```bash
git add internal/services/network/assignment_apply.go internal/services/network/assignment_apply_test.go internal/storage/storagetest/fixtures.go internal/gateway/portal/handlers/network_assignments.go
git commit -m "refactor(network): extract ApplyAssignmentMaterializers helper + record retry-state

Plan 4 Task 5 — устраняет 4-кратное дублирование materialize-блоков.
Helper также записывает last_materialize_error_at для cron-retry (Task 6)."
```

### Task 6: Cron-retry для unmaterialized SRA в `cmd/worker`

**Files:**
- Create: `internal/services/network/retry_loop.go`
- Create: `internal/services/network/retry_loop_test.go`
- Modify: `cmd/worker/main.go` — поднять goroutine с `RetryLoop` (после инициализации materializer'ов)

- [ ] **Step 1: Прочитать `cmd/worker/main.go` для понимания, какие компоненты уже инициализированы**

```bash
```

Read: `cmd/worker/main.go` (полностью). Зафиксировать: есть ли там pool, materializer'ы, signal-handling. Реализация в Step 3 опирается на это.

- [ ] **Step 2: Failing test**

`internal/services/network/retry_loop_test.go`:

```go
package network

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRetryLoop_RetriesPendingRows_AndClearsOnSuccess(t *testing.T) {
	pool := storagetest.MustOpenPool(t)
	defer pool.Close()
	clientID := storagetest.SeedSubAccount(t, pool)
	storagetest.SeedSRAErrorState(t, pool, clientID, "old failure")

	var providerCalls atomic.Int32
	pm := stubProviderApplier{onCall: func() error { providerCalls.Add(1); return nil }}
	rm := stubRouteApplier{onCall: func() error { return nil }}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	require.NoError(t, RetryPendingOnce(ctx, pool, pm, rm))

	assert.GreaterOrEqual(t, providerCalls.Load(), int32(1))

	var errText *string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT last_materialize_error_text FROM subaccount_routing_assignment WHERE client_id=$1",
		clientID,
	).Scan(&errText))
	assert.Nil(t, errText, "retry-state must be cleared on success")
}

func TestRetryLoop_KeepsErrorOnContinuedFailure(t *testing.T) {
	pool := storagetest.MustOpenPool(t)
	defer pool.Close()
	clientID := storagetest.SeedSubAccount(t, pool)
	storagetest.SeedSRAErrorState(t, pool, clientID, "first")

	pm := stubProviderApplier{onCall: func() error { return assert.AnError }}
	rm := stubRouteApplier{onCall: func() error { return nil }}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	require.NoError(t, RetryPendingOnce(ctx, pool, pm, rm))

	var retryCount int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT materialize_retry_count FROM subaccount_routing_assignment WHERE client_id=$1",
		clientID,
	).Scan(&retryCount))
	assert.GreaterOrEqual(t, retryCount, 2, "должен инкрементировать счётчик при повторном fail")
	_ = uuid.Nil
}

type stubProviderApplier struct{ onCall func() error }

func (s stubProviderApplier) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	return s.onCall()
}

type stubRouteApplier struct{ onCall func() error }

func (s stubRouteApplier) ApplyToClient(_ context.Context, _ uuid.UUID, _ *uuid.UUID) error {
	return s.onCall()
}
```

- [ ] **Step 3: Реализация**

`internal/services/network/retry_loop.go`:

```go
package network

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// RetryPendingOnce читает все SRA-rows с last_materialize_error_at IS NOT NULL
// и пробует применить материализацию повторно. Idempotent. Не реализует
// exponential backoff на уровне отдельной строки — это полагается на интервал
// верхнего цикла (RunRetryLoop). Возвращает error только при невозможности
// прочитать pending-rows; per-row сбои остаются записанными в SRA.
func RetryPendingOnce(ctx context.Context, pool *pgxpool.Pool, pm ProviderApplier, rm RouteApplier) error {
	rows, err := pool.Query(ctx, `
		SELECT client_id, provider_set_id, route_set_id
		  FROM subaccount_routing_assignment
		 WHERE last_materialize_error_at IS NOT NULL
		 ORDER BY last_materialize_error_at ASC
		 LIMIT 100`)
	if err != nil {
		return err
	}
	type pending struct {
		clientID    uuid.UUID
		providerSet *uuid.UUID
		routeSet    *uuid.UUID
	}
	var batch []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.clientID, &p.providerSet, &p.routeSet); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, p)
	}
	rows.Close()

	for _, p := range batch {
		warnings := ApplyAssignmentMaterializers(ctx, pool, pm, rm, p.clientID, p.providerSet, p.routeSet)
		if len(warnings) == 0 {
			log.Info().Str("client_id", p.clientID.String()).Msg("retry succeeded — SRA materialized")
		}
	}
	return nil
}

// RunRetryLoop запускает RetryPendingOnce каждые `interval` до отмены ctx.
// При ctx.Done — graceful exit. Используется в cmd/worker для фонового retry.
func RunRetryLoop(ctx context.Context, pool *pgxpool.Pool, pm ProviderApplier, rm RouteApplier, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := RetryPendingOnce(ctx, pool, pm, rm); err != nil {
				log.Error().Err(err).Msg("retry loop iteration failed")
			}
		}
	}
}
```

- [ ] **Step 4: Wire в `cmd/worker/main.go`**

В worker'е после инициализации pool и materializer'ов (точные строки определить в Step 1) добавить:

```go
// SRA retry loop (Plan 4 Task 6) — каждые 60 секунд проверяем pending materialize.
go networkpkg.RunRetryLoop(ctx, dbPool, providerMat, routeMat, 60*time.Second)
```

Импорт `networkpkg "github.com/smpp-server/smpp-server/internal/services/network"`.

Если worker не имеет инициализированных materializer'ов (зависит от структуры — см. Step 1), создать их через `network.NewProviderSetMaterializer(...)` и `network.NewRouteSetMaterializer(...)`.

- [ ] **Step 5: Tests + build**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T dev go build -buildvcs=false ./cmd/worker/..."
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/..."
```

Expected: build OK, tests PASS.

- [ ] **Step 6: Deploy worker и smoke**

```bash
./scripts/server.sh deploy worker
./scripts/server.sh logs worker
```

В логах должен появиться (после первого тика): нет error'ов, нет крэшей. Симулировать pending row вручную:

```bash
./scripts/server.sh exec "docker compose -f /opt/sms/deployments/docker-compose.yml exec -T postgres psql -U smpp -d smpp_db -c \"UPDATE subaccount_routing_assignment SET last_materialize_error_at=now(), last_materialize_error_text='test' WHERE client_id=(SELECT client_id FROM subaccount_routing_assignment LIMIT 1)\""
```

Подождать 60 сек, проверить что row очищен:

```bash
./scripts/server.sh exec "docker compose -f /opt/sms/deployments/docker-compose.yml exec -T postgres psql -U smpp -d smpp_db -c \"SELECT client_id, last_materialize_error_at, materialize_retry_count FROM subaccount_routing_assignment WHERE last_materialize_error_at IS NOT NULL\""
```

Expected: 0 rows, или (если повторный fail) — `materialize_retry_count` > 0.

- [ ] **Step 7: Commit**

```bash
git add internal/services/network/retry_loop.go internal/services/network/retry_loop_test.go cmd/worker/main.go
git commit -m "feat(worker): SRA retry loop for unmaterialized assignments

Plan 4 Task 6 — каждые 60s повторяет ApplyAssignmentMaterializers для строк
с last_materialize_error_at IS NOT NULL. Закрывает gap из Plan 3 Task 4
(committed-but-unmaterialized SRA остаётся неприменённым)."
```

### Task 7: Prometheus метрика `sra_pending_retry_count` + alert hint

**Files:**
- Modify: `internal/monitoring/portal/metrics.go` (или где живёт `MaterializeFailureTotal`)
- Modify: `internal/services/network/retry_loop.go` — set gauge каждый tick

- [ ] **Step 1: Найти metrics.go**

```bash
grep -rn "MaterializeFailureTotal" c:/projects/sms/internal/monitoring --include="*.go"
```

- [ ] **Step 2: Добавить gauge**

```go
var SRAPendingRetryGauge = promauto.NewGauge(prometheus.GaugeOpts{
    Name: "sra_pending_retry_count",
    Help: "Number of subaccount_routing_assignment rows pending materialize retry.",
})
```

- [ ] **Step 3: Установить gauge в RetryPendingOnce**

После SELECT'а pending-rows в `RetryPendingOnce`, перед циклом:

```go
portal.SRAPendingRetryGauge.Set(float64(len(batch)))
```

(Импорт portal-package.)

- [ ] **Step 4: Build, test, commit**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T dev go build -buildvcs=false ./..."
git add internal/monitoring/portal/metrics.go internal/services/network/retry_loop.go
git commit -m "feat(metrics): SRAPendingRetryGauge for pending materialize retry tracking (Plan 4 Task 7)"
```

---

## Section 3 — Frontend warnings rendering

### Task 8: Render warnings в AssignmentsPage + SubAccountNetworkSection

**Files:**
- Modify: `portal-frontend/src/pages/network/AssignmentsPage.tsx` — после `performBulk` показывать `partial`-результаты
- Modify: `portal-frontend/src/components/network/SubAccountNetworkSection.tsx` (или эквивалентный компонент, который вызывает `networkApi.putAssignment`)
- Modify: `portal-frontend/src/api/networkApi.ts` — убедиться что types для `BulkAssignResult` и `PutAssignmentResult` включают `warnings: Array<{step:string;error:string}>`

- [ ] **Step 1: Verify types**

Прочитать `networkApi.ts`. Найти типы для bulk и putAssignment responses. Если `warnings` отсутствует — добавить:

```ts
export interface BulkAssignResultItem {
  client_id: string;
  status: 'ok' | 'partial' | 'error' | 'conflict';
  error?: string;
  warnings?: Array<{ step: string; error: string }>;
}

export interface PutAssignmentResult {
  client_id: string;
  warnings?: Array<{ step: string; error: string }>;
}
```

- [ ] **Step 2: AssignmentsPage — показать partial в toast**

В `performBulk` (line ~113):

```ts
const ok = r.results.filter((x) => x.status === 'ok').length;
const partial = r.results.filter((x) => x.status === 'partial').length;
const conflictCount = r.results.filter((x) => x.status === 'conflict').length;
const total = r.results.length;
if (ok === total) {
  toast.success(`Назначено: ${ok} из ${total}`);
} else {
  const parts: string[] = [`OK: ${ok}/${total}`];
  if (partial > 0) parts.push(`частично (с warnings): ${partial}`);
  if (conflictCount > 0) parts.push(`конфликтов: ${conflictCount}`);
  toast.warning(parts.join('; '));
}
```

Дополнительно: после bulk — показать модал/панель со списком partial-row'ов (client_id + warnings). Использовать существующий `Dialog` из Radix.

- [ ] **Step 3: SubAccountNetworkSection — показать warnings из putAssignment**

Найти место вызова `networkApi.putAssignment(...)`. После success:

```ts
const result = await networkApi.putAssignment(clientID, body);
if (result.warnings && result.warnings.length > 0) {
  toast.warning(
    `Сохранено, но материализация частично не удалась: ${result.warnings
      .map((w) => `${w.step}: ${w.error}`)
      .join('; ')}. Cron повторит автоматически.`,
    { duration: 8000 },
  );
} else {
  toast.success('Назначение применено');
}
```

- [ ] **Step 4: Lint и type-check**

```bash
cd portal-frontend && npm run lint && npx tsc --noEmit
```

ESLint baseline = 51, не должно превышать.

- [ ] **Step 5: UI smoke на sandbox**

Логин aggregator@test.local. Открыть Назначения. Симулировать partial вручную: в БД выставить `last_materialize_error_at` на одной SRA, нажать «Применить» в bulk-mode на ту же sub-account. Убедиться что toast показывает «частично» и список warnings виден.

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/api/networkApi.ts portal-frontend/src/pages/network/AssignmentsPage.tsx portal-frontend/src/components/network/SubAccountNetworkSection.tsx
git commit -m "feat(portal): render materialize warnings from PutOne and Bulk responses

Plan 4 Task 8 — закрывает UX-долг Plan 3 Task 4. Aggregator теперь видит
partial-результаты bulk-assign и warnings от единичного PUT, понимает что
cron-retry (Task 6) автоматически повторит материализацию."
```

---

## Section 4 — Audit-log writes + UI (C7)

**Контекст.** spec §6.7 требует страницу «История изменений» по entity_type (route_set / provider_set / assignment). В коде:
- `audit_log` таблица существует (`migrations/000017`), partitioned by month, имеет `resource_type` column.
- `internal/services/audit/...` — read-only repository (`QueryAuditLog`), gRPC service `auditv1.AuditServiceClient`.
- `internal/gateway/portal/handlers/audit.go` — read endpoint `GET /portal/v1/audit-log` с фильтрами `action/user_id/date_from/date_to`. **`resource_type` фильтр отсутствует.**
- **Network handlers сейчас НЕ пишут audit-events.** Это первый раз.

Решение: писать audit-rows прямым INSERT'ом в DB из network handlers (proto-Record API нет смысла создавать сейчас — read-only audit-service остаётся read-only). Добавить `resource_type` фильтр в gRPC + REST.

### Task 9: Network audit writer

**Files:**
- Create: `internal/services/network/audit_writer.go`
- Create: `internal/services/network/audit_writer_test.go`

- [ ] **Step 1: Failing test**

`internal/services/network/audit_writer_test.go`:

```go
package network

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smpp-server/smpp-server/internal/storage/storagetest"
)

func TestRecordAuditEvent_InsertsRow(t *testing.T) {
	pool := storagetest.MustOpenPool(t)
	defer pool.Close()
	tenantID := uuid.New()
	userID := uuid.New()

	err := RecordAuditEvent(context.Background(), pool, AuditEvent{
		TenantID:     tenantID,
		UserID:       &userID,
		Action:       "create",
		ResourceType: "route_set",
		ResourceID:   "rs-123",
		Details:      map[string]interface{}{"name": "Default"},
	})
	require.NoError(t, err)

	var (
		action       string
		resourceType string
		resourceID   string
		details      []byte
	)
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT action, resource_type, resource_id, details
		   FROM audit_log
		  WHERE tenant_id=$1 AND user_id=$2
		  ORDER BY created_at DESC LIMIT 1`,
		tenantID, userID,
	).Scan(&action, &resourceType, &resourceID, &details))
	assert.Equal(t, "create", action)
	assert.Equal(t, "route_set", resourceType)
	assert.Equal(t, "rs-123", resourceID)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(details, &parsed))
	assert.Equal(t, "Default", parsed["name"])
}

func TestRecordAuditEvent_NilUserID_StillInserts(t *testing.T) {
	pool := storagetest.MustOpenPool(t)
	defer pool.Close()
	tenantID := uuid.New()

	err := RecordAuditEvent(context.Background(), pool, AuditEvent{
		TenantID:     tenantID,
		Action:       "delete",
		ResourceType: "provider_set",
		ResourceID:   "ps-1",
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE tenant_id=$1 AND user_id IS NULL`, tenantID,
	).Scan(&count))
	assert.Equal(t, 1, count)
}
```

- [ ] **Step 2: Реализация**

`internal/services/network/audit_writer.go`:

```go
package network

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// AuditEvent — запись в audit_log из network handler'ов.
// ResourceType ∈ {"route_set","route_set_item","provider_set","provider_set_item","assignment","route_override"}.
// Action ∈ {"create","update","delete","reorder","apply"}.
// Details — произвольный JSON, обычно {old: {...}, new: {...}} или {summary: "..."}.
type AuditEvent struct {
	TenantID     uuid.UUID              // обычно reseller_id
	UserID       *uuid.UUID             // nil для system-инициированных
	Action       string
	ResourceType string
	ResourceID   string
	Details      map[string]interface{}
	IPAddress    string                  // optional
}

// RecordAuditEvent — direct INSERT в audit_log. Не использует gRPC, потому что
// audit-service сейчас read-only (см. spec §6.7 commentary). Фейл — non-fatal:
// логируем и возвращаем error, но caller обычно игнорирует (mutation уже
// committed; audit-loss не должен блокировать UX).
func RecordAuditEvent(ctx context.Context, pool *pgxpool.Pool, e AuditEvent) error {
	var detailsJSON []byte
	if e.Details != nil {
		b, err := json.Marshal(e.Details)
		if err != nil {
			log.Warn().Err(err).Msg("audit details marshal failed")
			detailsJSON = []byte("{}")
		} else {
			detailsJSON = b
		}
	}
	var ip interface{}
	if e.IPAddress != "" {
		ip = e.IPAddress
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO audit_log (tenant_id, user_id, action, resource_type, resource_id, details, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.TenantID, e.UserID, e.Action, e.ResourceType, e.ResourceID, detailsJSON, ip,
	)
	if err != nil {
		log.Error().Err(err).
			Str("resource_type", e.ResourceType).
			Str("resource_id", e.ResourceID).
			Msg("audit event insert failed")
	}
	return err
}
```

- [ ] **Step 3: Run test**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/..."
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/services/network/audit_writer.go internal/services/network/audit_writer_test.go
git commit -m "feat(network): RecordAuditEvent helper for audit_log direct writes (Plan 4 Task 9)"
```

### Task 10: Wire audit-writes в network handler'ах

**Files:** modify (по одному audit call в каждом mutation-endpoint'е)
- `internal/gateway/portal/handlers/network_route_sets.go` — Create/Update/Delete
- `internal/gateway/portal/handlers/network_route_set_items.go` — Create/Update/Delete/Reorder/Duplicate
- `internal/gateway/portal/handlers/network_provider_sets.go` — Create/Update/Delete
- `internal/gateway/portal/handlers/network_provider_set_items.go` — Create/Update/Delete
- `internal/gateway/portal/handlers/network_assignments.go` — PutOne/Bulk
- `internal/gateway/portal/handlers/subaccount_network_overrides.go` — Add/Update/Delete RouteOverride

- [ ] **Step 1: Каноничный pattern**

После каждого успешного `tx.Commit()` (или после успешного `pool.Exec` для simple mutation'ов):

```go
userID, _ := middleware.GetUserID(r.Context()) // helper exists или nil-pointer-safe
_ = network.RecordAuditEvent(r.Context(), h.pool, network.AuditEvent{
    TenantID:     resellerID,
    UserID:       userID,                          // *uuid.UUID
    Action:       "create",                        // или "update"/"delete"/"reorder"
    ResourceType: "route_set",
    ResourceID:   rsID.String(),
    Details:      map[string]interface{}{"name": req.Name},
    IPAddress:    r.RemoteAddr,
})
```

`_ =` — намеренно: audit-fail не должен ломать UX. Logged внутри RecordAuditEvent.

- [ ] **Step 2: Verify GetUserID helper**

```bash
grep -rn "GetUserID\|UserIDKey\b" c:/projects/sms/internal/gateway/portal/middleware/ --include="*.go" | grep -v _test
```

Если helper отсутствует — создать в middleware/context.go:

```go
func GetUserID(ctx context.Context) (*uuid.UUID, bool) {
    v := ctx.Value(userIDContextKey)
    if v == nil {
        return nil, false
    }
    id, ok := v.(uuid.UUID)
    if !ok {
        return nil, false
    }
    return &id, true
}
```

(Если уже есть — переиспользовать.)

- [ ] **Step 3: Audit calls в каждый mutation handler**

Для каждого файла:
1. Прочитать handler полностью
2. Найти все mutation-endpoints (POST/PUT/DELETE/POST reorder)
3. Вставить audit-call ПОСЛЕ commit/successful exec, ПЕРЕД respondJSON
4. Заполнить Details:
   - Create: `{name, ...key fields}`
   - Update: `{old: {...}, new: {...}}` — для update сначала прочитать row до mutation в той же tx, серилизовать
   - Delete: `{name}` — прочитать перед удалением
   - Reorder: `{old_order: [...], new_order: [...]}`
   - PutOne assignment: `{provider_set_id, route_set_id}` (старые vs новые)
   - Bulk: один общий event на bulk-операцию `{client_count, provider_set_id, route_set_id}` плюс per-client events если status != "error"

ResourceType:
- route-set: `"route_set"`
- route-set-item: `"route_set_item"`, ResourceID = item ID
- provider-set: `"provider_set"`
- provider-set-item: `"provider_set_item"`
- assignment: `"assignment"`, ResourceID = client_id
- route-override: `"route_override"`, ResourceID = override ID

- [ ] **Step 4: Build на сервере**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T dev go build -buildvcs=false ./..."
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/handlers/..."
```

Expected: zero build errors, existing tests PASS.

- [ ] **Step 5: Smoke — UI создание route-set, проверка audit_log**

```bash
./scripts/server.sh exec "docker compose -f /opt/sms/deployments/docker-compose.yml exec -T postgres psql -U smpp -d smpp_db -c \"SELECT count(*) FROM audit_log WHERE resource_type IN ('route_set','provider_set','assignment')\""
```

До UI-действий: 0. После создания одного route-set через UI: ≥ 1.

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/portal/handlers/network_*.go internal/gateway/portal/handlers/subaccount_network_overrides.go internal/gateway/portal/middleware/*.go
git commit -m "feat(audit): write audit_log events for route-set / provider-set / assignment / override mutations

Plan 4 Task 10 — каждый POST/PUT/DELETE/REORDER теперь пишет в audit_log.
Подготовка к UI странице История изменений (Task 12)."
```

### Task 11: `resource_type` фильтр в gRPC + REST audit endpoint

**Files:**
- Modify: `api/proto/auditv1/audit.proto` (добавить `resource_type` в `QueryAuditLogRequest`)
- Regenerate: `api/proto/auditv1/audit.pb.go` через `make proto` или `protoc`
- Modify: `internal/services/audit/domain/audit.go` — добавить `ResourceType` в `AuditLogFilters`
- Modify: `internal/services/audit/infrastructure/repository/audit_repository.go` — `WHERE resource_type = $...`
- Modify: `cmd/services/audit-service/...` — передать filter из gRPC req → domain
- Modify: `internal/gateway/portal/handlers/audit.go` — прочитать `resource_type` query param

- [ ] **Step 1: Прочитать proto**

```bash
```

Read: `api/proto/auditv1/audit.proto` (полный). Зафиксировать поля `QueryAuditLogRequest`.

- [ ] **Step 2: Добавить в proto**

В `QueryAuditLogRequest` добавить:

```proto
string resource_type = 7; // optional filter
```

(Номер поля — следующий свободный; уточнить чтением.)

- [ ] **Step 3: Regenerate**

```bash
./scripts/server.sh exec "cd /opt/sms && make proto"
```

Если make-target нет — найти точный protoc-вызов в Makefile или существующих скриптах.

- [ ] **Step 4: Domain + repository**

В `AuditLogFilters`:
```go
ResourceType string
```

В `audit_repository.go` после блока `if filters.UserID != "" {...}`:
```go
if filters.ResourceType != "" {
    where += fmt.Sprintf(" AND resource_type = $%d", argIdx)
    args = append(args, filters.ResourceType)
    argIdx++
}
```

В audit-service gRPC handler (найти `QueryAuditLog` impl) — пробросить `req.ResourceType` в `filters.ResourceType`.

- [ ] **Step 5: Portal handler**

В `internal/gateway/portal/handlers/audit.go:35-45`:
```go
req := &auditv1.QueryAuditLogRequest{
    TenantId:     clientID.String(),
    Action:       query.Get("action"),
    UserId:       query.Get("user_id"),
    ResourceType: query.Get("resource_type"),
    Page:         page,
    PerPage:      perPage,
}
```

- [ ] **Step 6: Tests**

Расширить `internal/services/audit/infrastructure/repository/audit_repository_test.go` (если существует) или создать:

```go
func TestQueryAuditLog_FilterByResourceType(t *testing.T) { ... }
```

Test проверяет, что при `ResourceType="route_set"` возвращаются только matching rows.

- [ ] **Step 7: Build + tests + deploy**

```bash
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T dev go build -buildvcs=false ./..."
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/audit/... ./internal/gateway/portal/handlers/..."
```

- [ ] **Step 8: Commit**

```bash
git add api/proto/auditv1/audit.proto api/proto/auditv1/audit.pb.go internal/services/audit/ cmd/services/audit-service/ internal/gateway/portal/handlers/audit.go
git commit -m "feat(audit): resource_type filter in QueryAuditLog (gRPC + REST)

Plan 4 Task 11 — backend готов к network audit-log странице (Task 12)."
```

### Task 12: Frontend страница «История изменений»

**Files:**
- Create: `portal-frontend/src/pages/network/NetworkAuditLogPage.tsx`
- Modify: `portal-frontend/src/api/networkApi.ts` — добавить `getAuditLog({resource_type, date_from, date_to, page})`
- Modify: `portal-frontend/src/App.tsx` (или router-config) — route `/network/audit-log`
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx` — sidebar link «История изменений» под секцией «Сеть»

- [ ] **Step 1: networkApi.getAuditLog**

```ts
export interface AuditLogEntry {
  id: string;
  user_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details: string; // JSON string
  created_at: string;
}

export interface AuditLogResponse {
  entries: AuditLogEntry[];
  total: number;
  page: number;
  total_pages: number;
}

getAuditLog: async (params: {
  resource_type?: string;
  date_from?: string;
  date_to?: string;
  page?: number;
  per_page?: number;
}): Promise<AuditLogResponse> => {
  const qs = new URLSearchParams();
  if (params.resource_type) qs.set('resource_type', params.resource_type);
  if (params.date_from) qs.set('date_from', params.date_from);
  if (params.date_to) qs.set('date_to', params.date_to);
  if (params.page) qs.set('page', String(params.page));
  if (params.per_page) qs.set('per_page', String(params.per_page));
  return apiFetch<AuditLogResponse>(`/portal/v1/audit-log?${qs.toString()}`);
},
```

- [ ] **Step 2: NetworkAuditLogPage**

Минимальный UI:
- `Select` фильтр по resource_type: `route_set`, `provider_set`, `assignment`, `route_override`, `(все)`
- Date range picker
- Таблица: created_at | resource_type | action | resource_id | user_id (truncated) | [↓ details]
- Кнопка «Раскрыть details» — показать JSON `details` в pre-блоке (использовать существующий `Collapse` или Radix `Accordion`)
- Пагинация: existing pattern из других сетевых страниц

Layout — следовать `RouteSetsPage.tsx` для consistency.

- [ ] **Step 3: Route + sidebar**

В `App.tsx` (или router-конфиге) добавить:
```tsx
<Route path="/network/audit-log" element={<NetworkAuditLogPage />} />
```

В `UserLayout.tsx` под существующим блоком network (где живут «Поставщики», «Наборы провайдеров» и т.п.) добавить:
```tsx
<NavLink to="/network/audit-log">История изменений</NavLink>
```

(Точный pattern follow existing — может быть `<SidebarItem>` компонент.)

- [ ] **Step 4: Lint + type-check**

```bash
cd portal-frontend && npm run lint && npx tsc --noEmit
```

ESLint baseline 51, не превышать.

- [ ] **Step 5: UI smoke**

Логин aggregator@test.local. Открыть /network/audit-log. Создать route-set (Task 10 audit-write должен сработать). Обновить страницу — увидеть новую запись.

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/pages/network/NetworkAuditLogPage.tsx portal-frontend/src/api/networkApi.ts portal-frontend/src/App.tsx portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat(portal): network audit-log page

Plan 4 Task 12 — реализует spec §6.7. Aggregator видит историю изменений
route-set / provider-set / assignment / override с фильтрацией и details."
```

### Task 13: Verify aggregator-only scope для audit-log

**Контекст.** Существующий endpoint `GET /audit-log` использует `clientID` (= reseller_id для aggregator'а). Нужно убедиться что обычный sub-account НЕ видит audit-log других sub-account'ов.

**Files:**
- Verify only — `internal/gateway/portal/handlers/audit.go`

- [ ] **Step 1: Прочитать audit.go и router.go**

Подтвердить:
1. `audit.go:29` — `clientID = middleware.GetClientID(r.Context())` — это session-owner.
2. `audit.go:40` — `TenantId: clientID.String()` — фильтр по own ID.
3. `router.go:278` — endpoint mounted на `protected` router (auth).

Sub-account'у aggregator'а — `clientID` это его own client_id, не reseller'а. Audit-events родительского reseller'а пишутся с `tenant_id = reseller_id`. Sub-account своих событий НЕ создаёт (mutation-endpoints под `/sub-accounts/*` через ResellerOnlyMiddleware заблокированы для не-aggregator'ов). Это корректно.

- [ ] **Step 2: Подтвердить test**

Создать `internal/gateway/portal/handlers/audit_scope_test.go`:

```go
func TestListAuditLog_ScopedToOwnTenant(t *testing.T) {
    // Setup: 2 reseller'а, audit-rows под обоими.
    // Call ListAuditLog как reseller A — видит только его rows.
}
```

Реализация follow существующий test-pattern (e.g. `internal/gateway/portal/handlers/network_route_sets_test.go`).

- [ ] **Step 3: Run + commit**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/gateway/portal/handlers/... -run TestListAuditLog"
git add internal/gateway/portal/handlers/audit_scope_test.go
git commit -m "test(audit): verify tenant scope для ListAuditLog (Plan 4 Task 13)"
```

---

## Section 5 — Final smoke + DONE marker

### Task 14: Final smoke + DONE document

- [ ] **Step 1: Local check**

```bash
./scripts/check.sh
```

Expected: PASS, ESLint 51/51 warnings или меньше.

- [ ] **Step 2: Server tests суммарно**

```bash
git push origin master
./scripts/server.sh sync
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./internal/services/network/... ./internal/services/audit/... ./internal/gateway/portal/handlers/... ./internal/gateway/portal/middleware/... ./internal/config/..."
```

Expected: все PASS.

- [ ] **Step 3: Manual smoke — full happy path**

Зайти aggregator@test.local на http://72.56.232.202:18085/:
1. Создать новый route-set → проверить audit-log запись.
2. PUT assignment на sub-account → если success — без warning toast'а.
3. Симулировать materialize fail (сломать prerequisite) → увидеть warning toast → подождать 60s → проверить retry прошёл (audit-log не пишет retry, но БД-row очищена).
4. Открыть /network/audit-log → видеть все события.

- [ ] **Step 4: Redis hijack regression test**

С локальной машины:
```bash
redis-cli -h 72.56.232.202 -p 6379 ping
```

Expected: connection timeout или connection refused (firewall) или `NOAUTH Authentication required` (если firewall не сработал, но requirepass — да). НЕ должно быть `PONG`.

- [ ] **Step 5: DONE marker**

Создать `docs/superpowers/plans/2026-05-XX-aggregator-routing-plan-4-DONE.md` (XX = дата завершения) по шаблону Plan 3 DONE: список tasks, commits, smoke results, OUT-of-scope для Plan 5.

```bash
git add docs/superpowers/plans/
git commit -m "docs(plan): Plan 4 DONE — production hardening complete

Plan 4 closes prod-readiness блокеры aggregator routing (Redis hardening,
SRA retry, warnings UI, audit-log UI). Готовы к prod-rollout aggregator
routing'а. OUT-of-scope: E2E CI, operator/country preview accuracy
(retained for Plan 5)."
```

- [ ] **Step 6: Memory updates**

Обновить:
- `project_redis_hijack_2026_05_04` → закрыть (requirepass + firewall применены, sandbox защищён).
- `project_aggregator_decisions.md` → добавить, что Plan 4 завершил retry-loop и audit-trail.

---

## Out of scope (Plan 5 кандидаты)

- **E2E CI** — отдельный инфра-эпик (GH Actions с Linux runner + playwright или Docker on sandbox).
- **Operator-condition matching в `network_route_preview`** — JOIN на operators table.
- **Country-prefix mapping** — вынести из hardcoded в DB/config.
- **N-row signature compare in `findDuplicateOverrideSignature`** — generated column в SQL (defer до scaling pressure).
- **Двухуровневая reseller-иерархия UI** — defer per spec §3.4.
- **Optimistic concurrency (412 на PUT)** — defer per spec §6.5.
- **Differential preview** — defer per spec §4.6.

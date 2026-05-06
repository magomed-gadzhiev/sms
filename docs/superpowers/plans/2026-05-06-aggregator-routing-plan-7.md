# Aggregator Routing — Plan 7: Production Cutover

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Подготовить и выполнить cutover aggregator-routing подсистемы (фундамент Plans 1-6) на production environment с минимальным риском downtime/data-loss.

**Architecture:** docker-compose clone на новом VM; Caddy для TLS auto-Let's Encrypt; email-relay alertmanager; pg_basebackup daily snapshot; canary-режим первые 24h (один test-client allowlisted на app-уровне); deploy через `git pull + docker compose up` с downtime-окном 2-3 минуты на сервис; rollback через предыдущий git tag + restore из snapshot.

**Tech Stack:** Docker Compose, Caddy (TLS), Postgres 15 partitioning, Prometheus + Alertmanager (email receiver), Grafana provisioning, pg_basebackup, golang-migrate, Go 1.24 (advisory-lock в retry-loop).

**MVP defaults (зафиксированы 2026-05-06):**
- P1 hosting: docker-compose clone на новом VM (не k8s).
- P2 alert receiver: email через SMTP-relay.
- P3 TLS: Caddy auto-Let's Encrypt.
- P4 cutover scope: полный stack включая SMPP, но canary 24h (один client allowlisted).
- P5 data: greenfield (нет legacy импорта).
- P6 deploy: `git pull + docker compose up` с downtime-окном.
- P7 backup: pg_basebackup daily, retention 7 дней, RPO ~24h.

Каждая задача → subagent-driven (implementer + spec reviewer + code-quality reviewer) с обязательным review-gate перед коммитом. Trivial follow-ups — inline (≤ 2 строки edit).

---

## Stage 0 — Sandbox foundation (закрытие хвостов Plan 6)

### Task 1: Apply Redis firewall на sandbox + verify

**Контекст:** `scripts/redis-firewall.sh` написан в Plan 6 (Task 6), но не применён — для `iptables -I` нужен sudo, claude user без sudo. Это блокер cutover (Redis с requirepass, но без firewall — defense только один слой). Memory `project_redis_hijack_2026_05_04` фиксирует прецедент.

**Files:**
- Verify only: `scripts/redis-firewall.sh`
- Doc update: `docs/ops/redis-firewall-apply.md` (Create)

- [ ] **Step 1: Прочитать текущий iptables на sandbox**

```bash
./scripts/server.sh exec "sudo -n iptables -L INPUT -n 2>&1 | grep ':6379' || echo 'NOT APPLIED'"
```

Expected: либо вывод правил, либо `NOT APPLIED`. Если sudo требует пароль (`a password is required`) — escalate пользователю: "Нужно вручную залогиниться SSH под пользователем с sudo и выполнить `sudo bash /opt/sms/scripts/redis-firewall.sh`". Дождаться подтверждения.

- [ ] **Step 2: После apply — повторная verify**

```bash
./scripts/server.sh exec "sudo -n iptables -L INPUT -n | grep ':6379'"
```

Expected: правила DROP для not-127.0.0.1/not-docker-bridge на 6379.

- [ ] **Step 3: Probe от внешнего хоста (windows локально)**

```bash
nc -zv 72.56.232.202 6379 -w 3
```

Expected: `Connection timed out` (DROP, не REJECT). Если `Connected` — firewall не работает, escalate.

- [ ] **Step 4: Создать `docs/ops/redis-firewall-apply.md`**

Содержание (полный текст):

```markdown
# Redis Firewall Apply Procedure

## Когда применять

При первом deploy на новый VM, и после любого `iptables -F` (например, после рестарта VM, если `iptables-persistent` не настроен).

## Как применять

1. SSH под пользователем с sudo:
   `ssh user@<host>`
2. Выполнить скрипт:
   `sudo bash /opt/sms/scripts/redis-firewall.sh`
3. Verify:
   `sudo iptables -L INPUT -n | grep ':6379'` — должны быть DROP-правила.
4. Probe извне:
   `nc -zv <host> 6379 -w 3` с локального хоста — должен timeout.

## Persistence

iptables правила НЕ переживают reboot без `iptables-persistent`. После применения:
`sudo apt-get install -y iptables-persistent && sudo netfilter-persistent save`

## Rollback

`sudo iptables -F INPUT` — снимает все правила (опасно, открывает Redis обратно). Используется только при инциденте, когда firewall блокирует легитимный traffic.
```

- [ ] **Step 5: Commit**

```bash
git add docs/ops/redis-firewall-apply.md
git commit -m "docs(ops): redis firewall apply procedure (Plan 7 Task 1)"
```

---

### Task 2: A1 — Advisory-lock в RetryPendingOnce — **DEFERRED to Plan 8**

**2026-05-06 update:** при имплементации обнаружено design flaw — `pg_try_advisory_xact_lock` в SELECT WHERE релизится сразу после `rows.Close()`, до начала processing. Real race window = весь applier time (~ms-ы до сотен ms), не "единицы ms" как ассертил оригинальный план. Two goroutine c overlapping ticks claim'ят все rows независимо, lock даёт защиту только при literal-microsecond-overlap SELECT'ах (probability < 1%). Approach unsuitable.

**Альтернативы (для Plan 8):**
- (b) UPDATE-RETURNING claim с новой колонкой `claimed_at` (миграция + UPDATE-pattern): атомарный claim, работает для multi-replica.
- (a) Обхватить SELECT + applier в `pgx.Tx`: требует refactor сигнатуры `ApplyAssignmentMaterializers` (`*pgxpool.Pool` → `pgx.Tx`-совместимый интерфейс). Больше рефакторинга, чем (b).

**Текущее состояние:** revert commit 186a217. Single-replica worker prod (как было) — race не реализуема. После scale-up Plan 8 решает корректно.

**Original (incorrect) implementation skipped:**


**Контекст:** `internal/services/network/retry_loop.go` сейчас читает SRA-rows и обрабатывает их без synchronization. Single replica — race не realisable. На prod scale-up к 2+ replicas worker'а duplicate-work + race на `materialize_retry_count++`. A1 страховка: per-row `pg_try_advisory_xact_lock(client_id_high, client_id_low)` — реплика, у которой не получился lock, пропускает row на этом тике (другая реплика обрабатывает).

**Critical caveat:** `pg_advisory_xact_lock` берёт int8/int8 пару. UUID → 2x int64 через `(uuid_send(client_id))::bytea` ручное split. Альтернатива: `hashtext(client_id::text)` — int4, теряем 96 бит entropy, риск collision на 10k-row scale ≈ negligible (1 из 4B). Выбираем hashtext для простоты, документируем trade-off в коде.

**Files:**
- Modify: `internal/services/network/retry_loop.go`
- Test: `internal/services/network/retry_loop_advisory_lock_test.go` (Create)

- [ ] **Step 1: Написать failing test для advisory-lock**

Test scenario: запустить два goroutine, каждый вызывает `RetryPendingOnce` параллельно, считать ApplyAssignmentMaterializers calls — должно быть ровно `len(batch)` (а не 2x), все unique по client_id.

Создать `internal/services/network/retry_loop_advisory_lock_test.go`:

```go
package network_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/example/sms/internal/services/network"
	"github.com/example/sms/internal/testutil"
)

// countingApplier фиксирует client_ids обработанных rows (race-free через mutex).
type countingApplier struct {
	mu     sync.Mutex
	called map[uuid.UUID]int
	delay  time.Duration
}

func (c *countingApplier) Apply(ctx context.Context, _ *pgxpool.Pool, clientID uuid.UUID, _ *uuid.UUID) []network.MaterializeWarning {
	c.mu.Lock()
	c.called[clientID]++
	c.mu.Unlock()
	if c.delay > 0 {
		time.Sleep(c.delay) // удерживаем lock
	}
	return nil
}

func TestRetryPendingOnce_AdvisoryLockPreventsDuplicateWork(t *testing.T) {
	pool := testutil.AcquireTestPool(t)
	ctx := context.Background()

	// Seed: 5 stuck SRA-rows.
	clientIDs := make([]uuid.UUID, 5)
	for i := range clientIDs {
		clientIDs[i] = uuid.New()
		testutil.SeedStuckSRARow(t, pool, clientIDs[i])
	}
	t.Cleanup(func() {
		for _, id := range clientIDs {
			testutil.CleanupSRARow(t, pool, id)
		}
	})

	pm := &countingApplier{called: make(map[uuid.UUID]int), delay: 200 * time.Millisecond}
	rm := &countingApplier{called: make(map[uuid.UUID]int)}

	var wg sync.WaitGroup
	var totalErr atomic.Int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := network.RetryPendingOnce(ctx, pool, pm, rm); err != nil {
				totalErr.Add(1)
			}
		}()
	}
	wg.Wait()

	require.Zero(t, totalErr.Load(), "RetryPendingOnce должен не возвращать err")
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, id := range clientIDs {
		require.Equal(t, 1, pm.called[id],
			"client %s должен обрабатываться ровно один раз (advisory-lock); got %d", id, pm.called[id])
	}
}
```

**ПРОВЕРИТЬ перед использованием:** существуют ли helpers `testutil.AcquireTestPool`, `testutil.SeedStuckSRARow`, `testutil.CleanupSRARow`. Memory `feedback_verify_before_asserting` требует. Если нет — implementer пишет их в `internal/testutil/sra_fixtures.go` как часть этой task'и (минимально: Pool из ENV `TEST_DATABASE_URL`, INSERT в `subaccount_routing_assignment` с `last_materialize_error_at = now()`, DELETE по client_id).

Также проверить актуальную сигнатуру `ProviderApplier`/`RouteApplier` в `internal/services/network/applier.go` — если интерфейс отличается от Apply(ctx, pool, clientID, *uuid.UUID), привести в соответствие.

- [ ] **Step 2: Run test, verify FAIL**

```bash
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false -run TestRetryPendingOnce_AdvisoryLockPreventsDuplicateWork ./internal/services/network/..."
```

Expected: FAIL — at least one client_id обработан 2 раза (race есть).

- [ ] **Step 3: Modify `RetryPendingOnce` — добавить advisory-lock в SELECT**

Заменить SELECT в `internal/services/network/retry_loop.go:32-38`:

```go
rows, err := pool.Query(ctx, `
    SELECT client_id, provider_set_id, route_set_id
      FROM subaccount_routing_assignment
     WHERE last_materialize_error_at IS NOT NULL
       AND materialize_retry_count < 100
       AND pg_try_advisory_xact_lock(hashtext(client_id::text)::bigint)
     ORDER BY last_materialize_error_at ASC
     LIMIT 100`)
```

Добавить блок-комментарий перед SELECT:

```go
// pg_try_advisory_xact_lock(hashtext(client_id::text)::bigint) — synchronization
// для multi-replica worker'а: каждая реплика берёт row, только если ещё одна
// не держит lock на тот же client_id. xact_lock освобождается auto-commit'ом
// query (pgxpool single query == implicit transaction). hashtext даёт int4 →
// cast to bigint; collision risk на 10k clients ≈ 1/4B, acceptable.
// SELECT FOR UPDATE SKIP LOCKED не используем — он блокирует строку, а нам
// нужен advisory-lock логического уровня (ApplyAssignmentMaterializers
// внутри открывает свои транзакции и UPDATE'ит ту же row).
```

Critical: `xact_lock` живёт до конца транзакции. `pool.Query` создаёт implicit transaction только на длительность Query → Scan. После `rows.Close()` lock уже снят, и долгая `ApplyAssignmentMaterializers` идёт без lock. Это **correctly** — lock защищает только этап SELECT/claim, не processing. Если две реплики сделают SELECT одновременно, advisory предотвратит claim одной row двумя репликами; processing обеих rows параллельно не ломает invariants (ApplyAssignmentMaterializers internally idempotent через `materialize_retry_count++`).

Verify: read `internal/services/network/applier.go` (или where `ApplyAssignmentMaterializers` lives) — confirm idempotency (UPDATE через `materialize_retry_count = materialize_retry_count + 1` без condition). Если есть условный INSERT без `ON CONFLICT` — advisory-lock не достаточен, нужен row-level FOR UPDATE. Если confirm idempotency — coding done.

- [ ] **Step 4: Run test, verify PASS**

Same command as Step 2. Expected: PASS, каждый client обработан ровно 1 раз.

- [ ] **Step 5: Race-detector run (A5)**

```bash
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false -race -run TestRetryPendingOnce ./internal/services/network/..."
```

Expected: PASS без `WARNING: DATA RACE`. Если detector падает с `gcc not found` в dev container — добавить `apt-get install -y gcc` в `deployments/docker/Dockerfile.dev` (rebuild dev container) и повторить. Если gcc недоступен на sandbox VM (Device Guard или подобное) — задокументировать в task review как known limitation, race-detector только в CI (Plan 8).

- [ ] **Step 6: Commit**

```bash
git add internal/services/network/retry_loop.go internal/services/network/retry_loop_advisory_lock_test.go internal/testutil/sra_fixtures.go
git commit -m "feat(network): advisory-lock в SRA retry-loop для multi-replica safety (Plan 7 Task 2)

Per-row pg_try_advisory_xact_lock(hashtext(client_id)) гарантирует, что
claim row'ы происходит ровно одной репликой. Processing после claim'а
параллелизуется, что correctly — ApplyAssignmentMaterializers идемпотентна.

Test verify: 2 параллельных RetryPendingOnce → каждый client_id
обработан ровно 1 раз (без advisory-lock — 2x duplicate work)."
```

---

### Task 3: B1 doc — prod-migration-procedure.md

**Контекст:** Все Plan 1-6 миграции (1-144) накатывались на пустой sandbox без CONCURRENTLY. На prod с live traffic'ом `CREATE INDEX` берёт `ShareLock` на таблицу — блокирует writes на минуты. Greenfield первый накат — лочить нечего, но операционные процедуры (создание индекса для оптимизации, ADD COLUMN с дефолтом) после go-live должны идти через CONCURRENTLY/multi-step.

**Files:**
- Create: `docs/ops/prod-migration-procedure.md`

- [ ] **Step 1: Создать `docs/ops/prod-migration-procedure.md`**

Полный текст:

```markdown
# Production Migration Procedure

## Применимость

Greenfield first-deploy: накатывать `golang-migrate up` стандартно — таблиц нет, locks ни на что. **После первого live-traffic'а** любая новая миграция должна следовать процедурам ниже.

## Запрещённые в live-traffic операции

| Операция | Почему | Замена |
|---|---|---|
| `CREATE INDEX foo ON bar(...)` | ShareLock — writes блокированы | `CREATE INDEX CONCURRENTLY` |
| `ALTER TABLE ... ADD COLUMN ... NOT NULL DEFAULT v` | в pg < 11 rewrite всей таблицы | 3-step: ADD COLUMN nullable → backfill batched → SET NOT NULL |
| `ALTER TABLE ... ALTER COLUMN ... TYPE` | rewrite таблицы | новая колонка + backfill + DROP старой (multi-release) |
| `DROP COLUMN` без preparation | ломает inflight queries из старых деплоев | сначала remove из app code → deploy → DROP в следующем релизе |
| Любой `ALTER TABLE ... ADD CONSTRAINT NOT VALID → VALIDATE` за один шаг | VALIDATE берёт ShareUpdateExclusive | `ADD ... NOT VALID` в одной миграции, `VALIDATE CONSTRAINT` в следующей после прогрева |

## CONCURRENTLY edge case

`CREATE INDEX CONCURRENTLY` **нельзя** запустить внутри транзакции. golang-migrate по умолчанию оборачивает каждый файл миграции в transaction. Способы:

1. **Гибрид:** разделить миграцию — `.up.sql` без транзакции (golang-migrate magic comment `-- migrate:no-transaction` если поддерживается версией; verify в `pkg/migrate/...` или `cmd/migrate/main.go`).
2. **Manual:** запустить `CREATE INDEX CONCURRENTLY` через `psql` напрямую вне миграции, потом insert строку в `schema_migrations` руками с тем же version, потом `up` нормальной миграции (которая уже no-op для этого индекса). Документировать в commit message.

**Рекомендация:** Способ 2 для редких случаев, способ 1 — если стандартизировать. Plan 7 не выбирает — решение по факту первой post-cutover миграции, документируется в этом файле тогда же.

## ADD COLUMN с дефолтом — 3-step pattern

Migration N (release K):
- `ALTER TABLE foo ADD COLUMN bar text NULL;`
- В app code: writes пишут `bar` (старые читатели игнорируют).

Backfill (release K+1, отдельная миграция или manual):
- `UPDATE foo SET bar = 'default' WHERE bar IS NULL;` — батчами по 10k, не одной транзакцией (для больших таблиц).

Migration N+1 (release K+2):
- `ALTER TABLE foo ALTER COLUMN bar SET NOT NULL;`
- `ALTER TABLE foo ALTER COLUMN bar SET DEFAULT 'default';`

## Migration safety review checklist

Перед merge каждой post-cutover миграции:

- [ ] Локи: какие locks таблицы берёт миграция? (pg `pg_locks` view + EXPLAIN на тестовой копии)
- [ ] Backfill: если `UPDATE` затрагивает >10k rows — батчинг? cursor + LIMIT?
- [ ] Reversibility: `.down.sql` действительно откатывает .up без data loss? (Test через `migrate down 1; migrate up 1` на staging копии prod)
- [ ] App-code compat: новый код жёстко требует новую колонку, или graceful fallback? (Если жёстко — release order важен: миграция → deploy кода).
- [ ] Партиции: если меняется partitioned table — изменение применяется ко всем партициям? (golang-migrate сам не парсит, проверить вручную через `pg_inherits`).

## Procedure

1. Pre-deploy на staging копии prod (`pg_basebackup` от prod, restore на staging-host).
2. Замерить duration: `\timing on; <SQL>;`. Если > 30s на staging копии — пересмотреть стратегию.
3. Apply на prod с window: уведомить ops, запустить migrate из maintenance host (`./scripts/server.sh migrate` на prod-host).
4. Verify: `\d <table>` через `psql` — нет блокировок (`SELECT * FROM pg_stat_activity WHERE wait_event_type='Lock'`).
5. Watch: 15min на dashboard SRA-aggregator + general portal-gateway метрики — нет regression.

## Rollback миграции

Только если миграция reversible (`.down.sql` существует и тестирован). После live-data — обычно destructive (DROP COLUMN теряет data). Предпочтение — forward-fix (новая миграция, исправляющая broken state) над rollback.
```

- [ ] **Step 2: Verify внутренние ссылки**

`grep -n "migrate:no-transaction\|no-transaction" cmd/migrate/main.go pkg/migrate/*.go 2>/dev/null` — определить, поддерживает ли наш wrapper флаг no-transaction. Если да — упомянуть в doc'е конкретный синтаксис; если нет — оставить общую рекомендацию "способ 2 (psql + ручной insert)".

- [ ] **Step 3: Commit**

```bash
git add docs/ops/prod-migration-procedure.md
git commit -m "docs(ops): production migration procedure (Plan 7 Task 3)

Документирует CONCURRENTLY-safe patterns, 3-step ADD COLUMN с дефолтом,
locks-checklist перед merge post-cutover миграций. Применимо после first
live-traffic; greenfield first-deploy идёт через стандартный migrate up."
```

---

## Stage 1 — Prod environment skeleton

### Task 4: Caddy reverse-proxy + TLS auto-Let's Encrypt

**Контекст:** На sandbox портал работает на 18085 без TLS (HTTP). На prod внешний trafic идёт через TLS. Caddy выбран потому что zero-config Let's Encrypt: `domain.com { reverse_proxy portal-gateway:8080 }` достаточно для valid certificate. Альтернатива — nginx + certbot — больше boilerplate.

**Files:**
- Create: `deployments/configs/caddy/Caddyfile`
- Create: `deployments/docker-compose.prod.yml` (override-файл, частично — финал в Task 5)

- [ ] **Step 1: Создать `deployments/configs/caddy/Caddyfile`**

```
# {$PORTAL_DOMAIN} и {$PORTAL_GATEWAY_HOST} — env-переменные, заполняются
# из .env.prod на момент `docker compose up`. Без них caddy не стартует.
{$PORTAL_DOMAIN} {
	tls {$LETSENCRYPT_EMAIL}

	# Portal SPA + API через portal-gateway.
	reverse_proxy {$PORTAL_GATEWAY_HOST}:8080 {
		header_up X-Real-IP {remote_host}
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Proto {scheme}
	}

	# Security headers — baseline OWASP.
	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "strict-origin-when-cross-origin"
		# CSP — оставляем permissive до отдельного audit'а; жёсткая CSP
		# ломает Vite-built bundle с inline-modulepreload.
		-Server
	}

	# Логи в stdout → docker logs.
	log {
		output stdout
		format json
	}

	# Compress responses.
	encode gzip
}

# Admin domain (отдельный subdomain для админки если PORTAL_ADMIN_DOMAIN задан).
# Опционально — если не задан, admin доступ через {$PORTAL_DOMAIN}/admin.
{$PORTAL_ADMIN_DOMAIN:placeholder.invalid} {
	@hasAdminDomain expression `{env.PORTAL_ADMIN_DOMAIN} != ""`
	handle @hasAdminDomain {
		tls {$LETSENCRYPT_EMAIL}
		reverse_proxy {$PORTAL_GATEWAY_HOST}:8080
	}
}
```

**ПРОВЕРИТЬ:** какой реально порт у portal-gateway внутри сети compose (`grep -A 5 "portal-gateway:" deployments/docker-compose.yml | grep ports`). Если не 8080 — исправить в Caddyfile.

- [ ] **Step 2: Создать `deployments/docker-compose.prod.yml` skeleton (Caddy-only часть; rest в Task 5)**

```yaml
# Production overlay. Применять: docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
version: '3.8'

services:
  caddy:
    image: caddy:2.8-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./configs/caddy/Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    environment:
      PORTAL_DOMAIN: ${PORTAL_DOMAIN}
      PORTAL_ADMIN_DOMAIN: ${PORTAL_ADMIN_DOMAIN:-}
      LETSENCRYPT_EMAIL: ${LETSENCRYPT_EMAIL}
      PORTAL_GATEWAY_HOST: portal-gateway
    depends_on:
      - portal-gateway
    networks:
      - sms-network

  # Portal-gateway: убираем publish 18085 наружу — теперь только internal.
  portal-gateway:
    ports: !reset []

volumes:
  caddy_data:
  caddy_config:
```

**ПРОВЕРИТЬ:** имя network в основном compose (`grep -A 2 "^networks:" deployments/docker-compose.yml`). Если не `sms-network` — исправить.

- [ ] **Step 3: Validate Caddyfile syntax**

```bash
./scripts/server.sh exec "cd /opt/sms && docker run --rm -v /opt/sms/deployments/configs/caddy/Caddyfile:/etc/caddy/Caddyfile:ro caddy:2.8-alpine caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile"
```

Expected: `Valid configuration` или конкретная ошибка с line number. На sandbox env-переменные не заданы — caddy validate падает с `placeholder evaluation`. Workaround: `-e PORTAL_DOMAIN=example.com -e LETSENCRYPT_EMAIL=ops@example.com -e PORTAL_GATEWAY_HOST=portal-gateway` в docker run.

- [ ] **Step 4: Commit**

```bash
git add deployments/configs/caddy/Caddyfile deployments/docker-compose.prod.yml
git commit -m "ops(prod): Caddy reverse-proxy с TLS auto-Let's Encrypt (Plan 7 Task 4)

docker-compose.prod.yml — overlay для prod-only сервисов. Caddy слушает 80/443,
проксирует на portal-gateway:8080. Security headers (HSTS, X-Frame-Options).
Domain через env PORTAL_DOMAIN, заполняется в .env.prod.

Sandbox не затронут (overlay не подключается)."
```

---

### Task 5: docker-compose.prod.yml — расширение и .env.prod.example

**Контекст:** Task 4 добавил Caddy. Этот шаг — оставшиеся prod-overrides: убрать debug-флаги, скрыть наружу только TLS-порты (80/443) + SMPP-порты (2775/2776), задать prod-aware logging level, секреты вынести в .env.prod.

**Files:**
- Modify: `deployments/docker-compose.prod.yml`
- Create: `deployments/.env.prod.example`
- Create: `docs/ops/prod-secrets-onboarding.md`

- [ ] **Step 1: Расширить `deployments/docker-compose.prod.yml`**

К содержимому из Task 4 добавить (внутри `services:`):

```yaml
  postgres:
    # Prod: убираем publish 5432 наружу.
    ports: !reset []
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    command:
      - "postgres"
      - "-c"
      - "config_file=/etc/postgresql/postgresql.conf"
      - "-c"
      - "shared_preload_libraries=pg_stat_statements"
    volumes:
      - postgres_data_prod:/var/lib/postgresql/data
      - ./configs/postgresql.conf:/etc/postgresql/postgresql.conf:ro

  redis:
    ports: !reset []
    command:
      - "redis-server"
      - "/usr/local/etc/redis/redis.conf"
      - "--requirepass"
      - "${REDIS_PASSWORD}"

  smpp-gateway:
    # Только 2775 наружу (SMPP), 2112 metrics остаётся internal.
    ports:
      - "2775:2775"

  smpp-server:
    ports:
      - "2776:2775"

  # ВСЕ остальные сервисы — ports !reset, доступ только через caddy/internal.
  client-gateway-1:
    ports: !reset []
  client-gateway-2:
    ports: !reset []
  admin-gateway-1:
    ports: !reset []
  admin-gateway-2:
    ports: !reset []
  portal-gateway:
    ports: !reset []
  api:
    ports: !reset []
  worker:
    ports: !reset []
  pipeline-worker:
    ports: !reset []
  dlr-delivery:
    ports: !reset []
  prometheus:
    ports: !reset []
  grafana:
    ports: !reset []
  loki:
    ports: !reset []
  promtail:
    ports: !reset []
  haproxy:
    ports: !reset []
  pgbouncer:
    ports: !reset []

  # Worker — увеличить log level для prod (info, не debug).
  worker:
    environment:
      LOG_LEVEL: info

volumes:
  postgres_data_prod:
```

**ПРОВЕРИТЬ:** имена всех сервисов. `grep -E "^  [a-z][a-z0-9-]+:$" deployments/docker-compose.yml`. Все без `ports !reset` — добавить. Особенно — какие сейчас имеют publish (`grep -B 1 "ports:" deployments/docker-compose.yml | grep -v "ports:"`).

- [ ] **Step 2: Создать `deployments/.env.prod.example`**

```bash
# === REQUIRED — заполнить на момент cutover ===
# Domain портала (FQDN с A-record на VM IP).
PORTAL_DOMAIN=portal.example.com
# Optional. Если не задан — admin доступен через {PORTAL_DOMAIN}/admin.
PORTAL_ADMIN_DOMAIN=
# Email для Let's Encrypt registration.
LETSENCRYPT_EMAIL=ops@example.com

# === DATABASE ===
POSTGRES_PASSWORD=__GENERATE__   # openssl rand -base64 32
POSTGRES_USER=smpp
POSTGRES_DB=smpp_db

# === REDIS ===
REDIS_PASSWORD=__GENERATE__   # openssl rand -base64 32

# === JWT secret (для admin-gateway, portal-gateway, client-gateway auth) ===
JWT_SECRET=__GENERATE__   # openssl rand -base64 64

# === Alertmanager SMTP (Task 9) ===
ALERTMANAGER_SMTP_HOST=smtp.example.com:587
ALERTMANAGER_SMTP_FROM=alerts@example.com
ALERTMANAGER_SMTP_USER=alerts@example.com
ALERTMANAGER_SMTP_PASSWORD=__GENERATE__
ALERTMANAGER_TO=ops@example.com

# === Grafana admin (бутстрап) ===
GRAFANA_ADMIN_PASSWORD=__GENERATE__

# === Logging ===
LOG_LEVEL=info

# === Backup retention ===
PG_BACKUP_RETENTION_DAYS=7

# === Canary (Task 12) ===
# UUID единственного клиента, которому разрешено sending первые 24h после cutover.
# Пустая строка после canary → все clients allowed.
CANARY_CLIENT_IDS=
```

- [ ] **Step 3: Создать `docs/ops/prod-secrets-onboarding.md`**

```markdown
# Production Secrets Onboarding

## Generate

Все `__GENERATE__` placeholder'ы в `.env.prod` заменить через:

```bash
openssl rand -base64 32   # для passwords
openssl rand -base64 64   # для JWT_SECRET
```

## Storage

`.env.prod` хранится **только** на prod-VM в `/opt/sms/deployments/.env.prod` с правами `chmod 600 root:root` (или owner = deploy user). Этот файл **не** в git, в `.gitignore` proverit:

```bash
grep -E "^\.env\.prod" deployments/.gitignore deployments/.env.prod.example .gitignore 2>/dev/null
```

Если отсутствует — добавить в корневой `.gitignore`:
```
deployments/.env.prod
.env.prod
```

## Rotation

JWT_SECRET ротация — пересмотр всех активных sessions (логаут всех). Procedure:
1. Generate new secret.
2. Update `.env.prod`.
3. `docker compose restart portal-gateway admin-gateway client-gateway-1 client-gateway-2`.
4. Все active sessions invalidated.

POSTGRES/REDIS passwords — change в `.env.prod` + `docker compose down + up`. Внимание: postgres password change требует `ALTER USER smpp WITH PASSWORD '...'` SQL команду до docker compose down (иначе данные теряются — postgres init password только на пустом volume).

## Backup of secrets

`.env.prod` бэкапится в encrypted vault (gpg --symmetric с ops master password) ежемесячно. Procedure:
```bash
gpg --symmetric --cipher-algo AES256 -o env.prod.$(date +%F).gpg /opt/sms/deployments/.env.prod
# Move env.prod.YYYY-MM-DD.gpg в защищённое хранилище.
```
```

- [ ] **Step 4: Verify .gitignore**

```bash
grep -E "^\.env\.prod|deployments/\.env\.prod" .gitignore deployments/.gitignore 2>/dev/null
```

Если строки нет — добавить:
```bash
echo -e "\n# Prod secrets\ndeployments/.env.prod\n.env.prod" >> .gitignore
```

- [ ] **Step 5: Commit**

```bash
git add deployments/docker-compose.prod.yml deployments/.env.prod.example docs/ops/prod-secrets-onboarding.md .gitignore
git commit -m "ops(prod): compose overlay + secrets template (Plan 7 Task 5)

docker-compose.prod.yml: убирает все publish'ы кроме 80/443/2775/2776,
включает requirepass для Redis, log_level=info для worker.
.env.prod.example: все required vars с __GENERATE__ маркерами.
prod-secrets-onboarding.md: процедура generate/store/rotation."
```

---

## Stage 2 — Database hardening

### Task 6: Audit_log partition auto-maintenance

**Контекст:** `audit_log` партиционирован помесячно, но вручную через `migrations/000029_create_2026_partitions.up.sql` — партиции до `audit_log_y2026m12`. После 2026-12 первый INSERT упадёт с `no partition of relation "audit_log" found for row`. На prod это incident. Решение: добавить периодическую задачу в worker'е, которая раз в день проверяет, есть ли партиция на N+1 месяц вперёд, и если нет — создаёт.

**Альтернативы:**
- pg_partman: extension, требует install в postgres image, lock-in. Отвергнуто.
- Cron в host (bash + psql): зависимость от crond вне docker, ломается на k8s/migration. Отвергнуто.
- Worker Go-cron: использует уже работающий `cmd/worker`, runs in-cluster, нет внешних зависимостей. Выбрано.

**Files:**
- Create: `internal/services/maintenance/partition_maintainer.go`
- Create: `internal/services/maintenance/partition_maintainer_test.go`
- Modify: `cmd/worker/main.go` (добавить goroutine)

- [ ] **Step 1: Failing test для PartitionMaintainer**

`internal/services/maintenance/partition_maintainer_test.go`:

```go
package maintenance_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/example/sms/internal/services/maintenance"
	"github.com/example/sms/internal/testutil"
)

func TestEnsureFuturePartitions_CreatesMissingPartitions(t *testing.T) {
	pool := testutil.AcquireTestPool(t)
	ctx := context.Background()

	// Forwardness 6 месяцев. Самая дальняя должна быть now + 6 months.
	now := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", now, 6))

	// Должны существовать audit_log_y2026m05 ... audit_log_y2026m11 (6 forward incl current).
	for i := 0; i < 6; i++ {
		t := now.AddDate(0, i, 0)
		name := fmt.Sprintf("audit_log_y%dm%02d", t.Year(), t.Month())
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM pg_class WHERE relname=$1)`, name).Scan(&exists))
		require.True(t, exists, "partition %s должна существовать", name)
	}
}

func TestEnsureFuturePartitions_Idempotent(t *testing.T) {
	pool := testutil.AcquireTestPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)

	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", now, 3))
	// Повторный вызов не должен падать (CREATE IF NOT EXISTS).
	require.NoError(t, maintenance.EnsureFuturePartitions(ctx, pool, "audit_log", now, 3))
}
```

**ПРОВЕРИТЬ:** существует ли `internal/testutil.AcquireTestPool` (Task 2 уже использовал — должен существовать после Task 2). Если нет — добавить там.

- [ ] **Step 2: Verify test FAIL**

```bash
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false -run TestEnsureFuturePartitions ./internal/services/maintenance/..."
```

Expected: FAIL — package not exists.

- [ ] **Step 3: Implement `internal/services/maintenance/partition_maintainer.go`**

```go
package maintenance

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// EnsureFuturePartitions гарантирует, что для partitioned-таблицы tableName
// существуют монтьные партиции от now до now+forwardMonths включительно.
// Idempotent — использует CREATE TABLE IF NOT EXISTS PARTITION OF.
//
// Применяется ТОЛЬКО для таблиц с RANGE partitioning по timestamp-колонке
// (created_at) с месячным шагом. Совместим с существующими миграциями
// 000001/000029 — формат имени audit_log_yYYYYmMM.
func EnsureFuturePartitions(ctx context.Context, pool *pgxpool.Pool, tableName string, now time.Time, forwardMonths int) error {
	if forwardMonths <= 0 {
		return fmt.Errorf("forwardMonths must be positive, got %d", forwardMonths)
	}
	for i := 0; i <= forwardMonths; i++ {
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, i, 0)
		monthEnd := monthStart.AddDate(0, 1, 0)
		partName := fmt.Sprintf("%s_y%dm%02d", tableName, monthStart.Year(), int(monthStart.Month()))
		ddl := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
			partName, tableName,
			monthStart.Format("2006-01-02"),
			monthEnd.Format("2006-01-02"),
		)
		if _, err := pool.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("create partition %s: %w", partName, err)
		}
	}
	return nil
}

// RunPartitionMaintenanceLoop запускает EnsureFuturePartitions раз в interval
// (рекомендуется 24h). Поддерживает таблицы из tableNames с одинаковым
// forwardMonths окном.
func RunPartitionMaintenanceLoop(ctx context.Context, pool *pgxpool.Pool, tableNames []string, forwardMonths int, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	// Первый прогон — сразу при старте.
	runOnce := func() {
		for _, tbl := range tableNames {
			if err := EnsureFuturePartitions(ctx, pool, tbl, time.Now().UTC(), forwardMonths); err != nil {
				log.Error().Err(err).Str("table", tbl).Msg("partition maintenance failed")
			} else {
				log.Info().Str("table", tbl).Int("forward_months", forwardMonths).
					Msg("partition maintenance: ensured future partitions")
			}
		}
	}
	runOnce()
	log.Info().Strs("tables", tableNames).Dur("interval", interval).
		Msg("partition maintenance loop started")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("partition maintenance loop stopped")
			return
		case <-t.C:
			runOnce()
		}
	}
}
```

- [ ] **Step 4: Wire в `cmd/worker/main.go`**

`grep -n "RunRetryLoop\|go " cmd/worker/main.go` — найти, где запускается retry loop, добавить рядом:

```go
// Plan 7 Task 6: ensure audit_log + lookup_log + messages партиции
// присутствуют на 6 месяцев вперёд.
go maintenance.RunPartitionMaintenanceLoop(ctx, pool,
    []string{"audit_log", "lookup_log", "messages", "deliveries", "delivery_attempts"},
    6, 24*time.Hour)
```

**ПРОВЕРИТЬ:** все ли таблицы из списка реально партиционированы (CLAUDE.md упоминает их пять). `psql -c "SELECT inhparent::regclass FROM pg_inherits LIMIT 50"` через server.sh exec на sandbox postgres. Если какая-то таблица НЕ партиционирована — убрать из списка (EnsureFuturePartitions упадёт ошибкой `is not partitioned`).

Импорт: `"github.com/example/sms/internal/services/maintenance"`.

- [ ] **Step 5: Verify test PASS**

Same command as Step 2. Expected: PASS.

- [ ] **Step 6: Smoke на sandbox — стартовать worker, проверить log**

```bash
./scripts/server.sh deploy worker
./scripts/server.sh logs worker | grep "partition maintenance"
```

Expected: логи `ensured future partitions` для каждой таблицы.

- [ ] **Step 7: Commit**

```bash
git add internal/services/maintenance/ cmd/worker/main.go
git commit -m "feat(maintenance): auto-create future monthly partitions (Plan 7 Task 6)

Worker раз в 24h вызывает EnsureFuturePartitions для audit_log, lookup_log,
messages, deliveries, delivery_attempts — гарантирует наличие партиций на
6 месяцев вперёд. Idempotent (CREATE IF NOT EXISTS PARTITION OF). Замещает
ручные миграции типа 000029_create_2026_partitions для будущих лет."
```

---

### Task 7: Prod admin seed — отдельный механизм без test creds

**Контекст:** `migrations/000116` сидит `aggregator@test.local` + `admin@example.com` с `Admin123!`. На prod это security-risk. Решение: миграция 000116 переименовать в `000116_sandbox_test_users.up.sql` и добавить guard — выполняться только если `SANDBOX_MODE=true` в env. На prod creds создаются через one-shot CLI (`cmd/seed-admin`).

**Уточнение:** `cmd/seed-admin` уже существует (упомянут в CLAUDE.md). Использовать его. Миграцию 000116 пометить как sandbox-only, но **не удалять**: она применена на sandbox, removal сломает migrations history. Вместо — добавить idempotent guard на уровне SQL.

**Files:**
- Modify: `migrations/000116_*.up.sql` (читать сначала)
- Read/Modify: `cmd/seed-admin/main.go`
- Create: `docs/ops/prod-admin-bootstrap.md`

- [ ] **Step 1: Прочитать текущее состояние**

```bash
cat migrations/000116_*.up.sql migrations/000116_*.down.sql
ls cmd/seed-admin/
cat cmd/seed-admin/main.go
```

Зафиксировать в комментарии task review:
- какой формат password hash (bcrypt cost?)
- какие fields user-row требуются (org_id, role_id, etc.)

- [ ] **Step 2: Modify migration 000116 — добавить guard**

В начало `migrations/000116_*.up.sql` добавить блок:

```sql
-- Plan 7 Task 7: миграция должна выполняться только в sandbox/dev environment.
-- На prod admin user создаётся через cmd/seed-admin (см. docs/ops/prod-admin-bootstrap.md).
-- Guard: переменная sandbox_mode проверяется через current_setting; если
-- не установлена — миграция skip'ается (создаёт NO rows).
DO $$
BEGIN
    IF current_setting('app.sandbox_mode', true) IS DISTINCT FROM 'true' THEN
        RAISE NOTICE 'Skipping sandbox test users — app.sandbox_mode != true';
        RETURN;
    END IF;
    -- ОРИГИНАЛЬНЫЙ INSERT ЗДЕСЬ — переместить inside DO ... END блока,
    -- внутри IF NOT skip.
END $$;
```

**Critical:** `current_setting('app.sandbox_mode', true)` — второй параметр `true` означает "missing_ok". Если переменная не задана, возвращает NULL, не error.

Установить переменную для sandbox: добавить в `deployments/configs/postgresql.conf`:

```
# Plan 7 Task 7: marker для sandbox-only seed migrations.
app.sandbox_mode = 'true'
```

На prod — НЕ добавлять эту строку. Тем самым 000116 в проде skip'ится при первом migrate up.

- [ ] **Step 3: Verify guard — re-apply migration на sandbox**

```bash
./scripts/server.sh exec "docker compose -f deployments/docker-compose.yml restart postgres"
./scripts/server.sh exec "docker compose -f deployments/docker-compose.yml exec -T postgres psql -U smpp -d smpp_db -c \"SHOW app.sandbox_mode;\""
```

Expected: `true`. Если не — postgres.conf не подхватился, проверить mount.

- [ ] **Step 4: Прочитать `cmd/seed-admin/main.go`**

```bash
cat cmd/seed-admin/main.go
```

Verify, что он принимает CLI args `--email`, `--password`, `--org-name` (или подобные). Если не принимает — расширить.

Если `cmd/seed-admin/main.go` сейчас hardcoded'ит `admin@example.com`, рефакторить (interactive prompt или CLI flags). Implementer оценивает по факту.

- [ ] **Step 5: Создать `docs/ops/prod-admin-bootstrap.md`**

```markdown
# Production Admin Bootstrap

## После first migrate up на prod

Партиция 000116 skip'ится (sandbox_mode != true в postgres.conf prod). User space пуст.

## Создать первого admin

```bash
ssh user@<prod-host>
cd /opt/sms
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml exec dev \
    go run ./cmd/seed-admin \
    --email "ops@example.com" \
    --org-name "ProductionOrg" \
    --role admin \
    --force-password-change true
```

CLI запросит пароль интерактивно (НЕ передавать через --password в bash history).

## Verify

Войти через `https://<PORTAL_DOMAIN>/admin` с этим email + новый пароль. На первом login форма требует смену пароля.

## Rollback / lockout recovery

Если admin пароль потерян до смены initial:

```bash
docker compose ... exec dev go run ./cmd/seed-admin --reset --email "ops@example.com"
```

(если `--reset` flag не существует — implementer добавляет в Task 7 step 4).
```

- [ ] **Step 6: Commit**

```bash
git add migrations/000116_*.up.sql cmd/seed-admin/main.go deployments/configs/postgresql.conf docs/ops/prod-admin-bootstrap.md
git commit -m "feat(seed): prod admin bootstrap через cmd/seed-admin (Plan 7 Task 7)

Migration 000116 теперь guard'ится через current_setting('app.sandbox_mode').
Sandbox postgres.conf задаёт app.sandbox_mode='true' → seed применяется.
Prod postgres.conf не имеет этой переменной → seed skip'ится.

cmd/seed-admin расширен принимать --email/--org-name через CLI args.
docs/ops/prod-admin-bootstrap.md описывает procedure после first migrate up."
```

---

## Stage 3 — Observability prod

### Task 8: External labels per env + templated alert thresholds

**Контекст:** `deployments/configs/prometheus/rules/sra-aggregator.yml` (Plan 6) хардкодит threshold `0.05 events/sec` для `SRAMaterializeInitialFailureRate`. На sandbox с минимальным traffic'ом 4 fail в 5min → false alert. На prod traffic'ом — real signal. Решение: external labels per env + threshold через template.

**Files:**
- Modify: `deployments/configs/prometheus.yml`
- Modify: `deployments/configs/prometheus/rules/sra-aggregator.yml`
- Create: `deployments/configs/prometheus.prod.yml` (overlay file: external_labels env=prod)

- [ ] **Step 1: В `deployments/configs/prometheus.yml` добавить external_labels**

В блок `global:` (top of file):

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    env: ${PROMETHEUS_ENV:sandbox}
```

`${PROMETHEUS_ENV:sandbox}` — env-substitution с дефолтом sandbox. Prometheus поддерживает env subst через `--enable-feature=expand-external-labels` flag. Добавить в command в compose:

`grep -n "prometheus" deployments/docker-compose.yml`, найти command секцию. Добавить:

```yaml
command:
  - "--config.file=/etc/prometheus/prometheus.yml"
  - "--enable-feature=expand-external-labels"
```

И env:

```yaml
environment:
  PROMETHEUS_ENV: ${PROMETHEUS_ENV:-sandbox}
```

В `.env.prod.example` (Task 5) добавить строку:

```
PROMETHEUS_ENV=prod
```

- [ ] **Step 2: Modify `deployments/configs/prometheus/rules/sra-aggregator.yml`**

Прочитать текущий файл:

```bash
cat deployments/configs/prometheus/rules/sra-aggregator.yml
```

Найти `SRAMaterializeInitialFailureRate`. Threshold 0.05 заменить на conditional через `{{ if eq $externalLabels.env "prod" }}`. Прометей не поддерживает Go-templates в expr напрямую — only в annotations. Альтернатива: **два параллельных alert'а** с разными expr и labels:

```yaml
- alert: SRAMaterializeInitialFailureRateProd
  expr: |
    rate(portal_sra_materialize_failure_total{source="initial"}[5m]) > 0.05
  for: 5m
  labels:
    severity: warning
    env: prod
    component: sra-aggregator
  annotations:
    summary: "SRA initial materialize failure rate (prod) > 0.05 events/sec"
    description: "Worker materialization failures > 0.05/sec в течение 5 мин — symptom общей проблемы (DB? router corruption? config drift?)"

- alert: SRAMaterializeInitialFailureRateSandbox
  expr: |
    rate(portal_sra_materialize_failure_total{source="initial"}[5m]) > 0.5
  for: 5m
  labels:
    severity: warning
    env: sandbox
    component: sra-aggregator
  annotations:
    summary: "SRA initial materialize failure rate (sandbox) > 0.5 events/sec"
    description: "Sandbox 10x prod threshold — игнорируем тестовый traffic ниже этого порога."
```

Alertmanager в Task 9 routes by `env` label → разные receivers (или один receiver с tag).

**Альтернатива (cleaner):** держать ОДИН alert, но `for:` time увеличить на sandbox (`for: 30m` vs prod `for: 5m`). Memory `feedback_no_code_in_chat` — выбираю первый (два alert'а), потому что threshold — функция от traffic volume, а not от noise tolerance, и prod/sandbox traffic различаются на ~2 порядка.

- [ ] **Step 3: Verify rules syntax**

```bash
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T prometheus promtool check rules /etc/prometheus/rules/sra-aggregator.yml"
```

Expected: `SUCCESS: ...` без errors.

- [ ] **Step 4: Verify external_labels на sandbox**

```bash
./scripts/server.sh exec "curl -s http://localhost:9090/api/v1/status/config | grep -A 3 external_labels"
```

Expected: `env: sandbox`.

- [ ] **Step 5: Commit**

```bash
git add deployments/configs/prometheus.yml deployments/configs/prometheus/rules/sra-aggregator.yml deployments/docker-compose.yml deployments/.env.prod.example
git commit -m "ops(metrics): external_labels env + per-env alert thresholds (Plan 7 Task 8)

PROMETHEUS_ENV={sandbox|prod} → external label env. Alert
SRAMaterializeInitialFailureRate split на две версии (prod 0.05/sec,
sandbox 0.5/sec — 10x для tolerance тестового traffic'а).
Alertmanager routes по env label (Task 9)."
```

---

### Task 9: Alertmanager service + email receiver

**Контекст:** Plan 6 написал alert rules, но без Alertmanager они fire'ятся в /dev/null. Email через SMTP-relay — выбран как universal default (см. P2).

**Files:**
- Create: `deployments/configs/alertmanager/alertmanager.yml`
- Modify: `deployments/configs/prometheus.yml` (alerting block)
- Modify: `deployments/docker-compose.yml` (alertmanager service)
- Modify: `deployments/docker-compose.prod.yml` (env passthrough)

- [ ] **Step 1: Создать `deployments/configs/alertmanager/alertmanager.yml`**

```yaml
global:
  smtp_smarthost: '${ALERTMANAGER_SMTP_HOST}'
  smtp_from: '${ALERTMANAGER_SMTP_FROM}'
  smtp_auth_username: '${ALERTMANAGER_SMTP_USER}'
  smtp_auth_password: '${ALERTMANAGER_SMTP_PASSWORD}'
  smtp_require_tls: true
  resolve_timeout: 5m

route:
  group_by: ['alertname', 'severity', 'env']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: 'email-default'
  routes:
    # Critical → отдельный subject prefix.
    - matchers:
        - severity = critical
      receiver: 'email-critical'
      group_wait: 10s
      repeat_interval: 1h
    # Sandbox → noop receiver (логируем, не emailим — слишком шумно).
    - matchers:
        - env = sandbox
      receiver: 'log-only'

receivers:
  - name: 'email-default'
    email_configs:
      - to: '${ALERTMANAGER_TO}'
        send_resolved: true
        headers:
          Subject: '[SMS-Platform {{ .CommonLabels.env }}] {{ .CommonLabels.alertname }} ({{ .Status }})'

  - name: 'email-critical'
    email_configs:
      - to: '${ALERTMANAGER_TO}'
        send_resolved: true
        headers:
          Subject: '[SMS-Platform CRITICAL {{ .CommonLabels.env }}] {{ .CommonLabels.alertname }}'

  - name: 'log-only'
    # Без email_configs — alert логируется в alertmanager logs, никуда не уходит.
    # Используется для sandbox.

inhibit_rules:
  - source_matchers:
      - severity = critical
    target_matchers:
      - severity = warning
    equal: ['alertname', 'env']
```

`${VAR}` substitution: alertmanager сам по себе **не** делает env subst в YAML. Workaround:
- Either entrypoint script с `envsubst` перед стартом (стандартное решение).
- Or использовать alertmanager 0.26+ с `--web.config.file` (новый features, не надёжно).

Выбираем envsubst. Создать `deployments/configs/alertmanager/entrypoint.sh`:

```bash
#!/bin/sh
set -e
envsubst < /etc/alertmanager/alertmanager.yml.tpl > /etc/alertmanager/alertmanager.yml
exec /bin/alertmanager --config.file=/etc/alertmanager/alertmanager.yml --storage.path=/alertmanager
```

И переименовать `alertmanager.yml` → `alertmanager.yml.tpl`. Сделать `chmod +x entrypoint.sh`.

- [ ] **Step 2: Add alertmanager service в `deployments/docker-compose.yml`**

```yaml
  alertmanager:
    image: prom/alertmanager:v0.27.0
    restart: unless-stopped
    volumes:
      - ./configs/alertmanager:/etc/alertmanager:ro
      - alertmanager_data:/alertmanager
    entrypoint: /etc/alertmanager/entrypoint.sh
    environment:
      ALERTMANAGER_SMTP_HOST: ${ALERTMANAGER_SMTP_HOST:-localhost:25}
      ALERTMANAGER_SMTP_FROM: ${ALERTMANAGER_SMTP_FROM:-alerts@localhost}
      ALERTMANAGER_SMTP_USER: ${ALERTMANAGER_SMTP_USER:-}
      ALERTMANAGER_SMTP_PASSWORD: ${ALERTMANAGER_SMTP_PASSWORD:-}
      ALERTMANAGER_TO: ${ALERTMANAGER_TO:-ops@localhost}
    networks:
      - sms-network

volumes:
  alertmanager_data:
```

(volumes блок дополняет существующий top-level `volumes:` — placement в нужном месте файла).

- [ ] **Step 3: Modify `deployments/configs/prometheus.yml` — alerting block**

В конец файла (или после `rule_files`):

```yaml
alerting:
  alertmanagers:
    - static_configs:
        - targets:
            - alertmanager:9093
```

- [ ] **Step 4: Verify alertmanager стартует на sandbox**

`.env` на sandbox дефолтные значения SMTP — alertmanager стартует но email не шлёт (sandbox unreachable smtp). Это OK, sandbox env=sandbox → log-only receiver.

```bash
./scripts/server.sh deploy alertmanager
./scripts/server.sh logs alertmanager | head -30
```

Expected: `Starting Alertmanager`, без crash. `level=error` про SMTP допустим (на log-only маршруте email не отправляется, но global SMTP config валидируется при старте). Если crash на startup из-за пустого SMTP host — изменить default `localhost:25` на placeholder, который не валидируется агрессивно.

- [ ] **Step 5: Verify alert routes**

```bash
./scripts/server.sh exec "curl -s http://localhost:9093/api/v2/status | head -50"
```

Expected: JSON с `versionInfo`, `config.original` content.

```bash
./scripts/server.sh exec "curl -s http://localhost:9090/api/v1/alerts | head"
```

Expected: текущие fire'ящиеся alerts (если есть).

- [ ] **Step 6: Commit**

```bash
git add deployments/configs/alertmanager/ deployments/configs/prometheus.yml deployments/docker-compose.yml
git commit -m "ops(alerts): Alertmanager + email receiver (Plan 7 Task 9)

Alertmanager 0.27.0 в compose. SMTP-receiver через env-vars
(envsubst в entrypoint). Routes: prod → email, sandbox → log-only,
critical → high-frequency repeat (1h vs default 4h). Inhibit:
critical suppresses warning с тем же alertname+env."
```

---

### Task 10: Grafana dashboards provisioning hardening

**Контекст:** `deployments/configs/grafana/provisioning/dashboards/dashboards.yml` уже работает (sra-aggregator.json подцеплен в Plan 6). Этот task — verify, что provisioning structure готов к prod (datasource alertmanager-aware, dashboards immutable=true).

**Files:**
- Read first, then potentially modify: `deployments/configs/grafana/provisioning/dashboards/dashboards.yml`
- Modify: `deployments/configs/grafana/provisioning/datasources/datasources.yml`

- [ ] **Step 1: Прочитать текущие provisioning files**

```bash
cat deployments/configs/grafana/provisioning/dashboards/dashboards.yml
cat deployments/configs/grafana/provisioning/datasources/datasources.yml
cat deployments/configs/grafana/provisioning/datasources/loki.yml
```

Зафиксировать в task review.

- [ ] **Step 2: Verify datasources.yml имеет prometheus + alertmanager**

Если alertmanager datasource отсутствует — добавить в `datasources.yml`:

```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
  - name: Alertmanager
    type: alertmanager
    access: proxy
    url: http://alertmanager:9093
    jsonData:
      implementation: prometheus
```

(loki — отдельный файл loki.yml, не трогаем).

- [ ] **Step 3: Verify dashboards.yml**

Если есть `disableDeletion: false` — поменять на `true` для prod (immutable provisioned dashboards):

```yaml
apiVersion: 1
providers:
  - name: 'sms-platform'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: true
    updateIntervalSeconds: 30
    allowUiUpdates: false
    options:
      path: /etc/grafana/provisioning/dashboards
```

`allowUiUpdates: false` — UI-changes не сохраняются (для prod хорошо: dashboard-as-code single source of truth). На sandbox это раздражает — там лучше `true`. Решение: оставить `false`, на sandbox развинчивать через UI всё равно не workflow.

- [ ] **Step 4: Restart grafana, verify dashboards visible**

```bash
./scripts/server.sh exec "docker compose -f deployments/docker-compose.yml restart grafana"
sleep 5
./scripts/server.sh exec "curl -s -u admin:admin http://localhost:3000/api/search | head -50"
```

Expected: JSON с `sra-aggregator` в списке dashboards. (admin:admin sandbox-default; на prod password из env через Task 5).

- [ ] **Step 5: Commit**

```bash
git add deployments/configs/grafana/provisioning/
git commit -m "ops(grafana): provisioning hardening для prod (Plan 7 Task 10)

datasources.yml: добавлен Alertmanager datasource (для UI alert-обзора).
dashboards.yml: disableDeletion=true, allowUiUpdates=false — provisioned
dashboards immutable, единый source of truth = git."
```

---

## Stage 4 — Deploy pipeline

### Task 11: deploy-prod.sh + rollback-prod.sh

**Контекст:** Sandbox использует `scripts/server.sh deploy` — `git pull && docker compose up -d --build`. На prod нужно: pre-deploy snapshot, migrate apply, deploy с verify health-check, abort если fail. Rollback: previous git tag + restore snapshot.

**Files:**
- Create: `scripts/deploy-prod.sh`
- Create: `scripts/rollback-prod.sh`
- Create: `scripts/healthcheck.sh`
- Modify: `scripts/server.sh` (добавить prod-команды)

- [ ] **Step 1: Создать `scripts/healthcheck.sh`**

```bash
#!/bin/bash
# Health check для всех критических prod-сервисов после deploy.
# Возвращает 0 если все ок, 1 если хотя бы один сервис не отвечает за timeout.
set -euo pipefail

TIMEOUT=${HEALTHCHECK_TIMEOUT:-60}  # секунды на каждый сервис

declare -A CHECKS=(
    ["postgres"]="docker compose -f deployments/docker-compose.yml exec -T postgres pg_isready -U smpp"
    ["redis"]="docker compose -f deployments/docker-compose.yml exec -T redis redis-cli -a \${REDIS_PASSWORD} ping | grep -q PONG"
    ["portal-gateway"]="curl -fsS http://localhost:8080/health"
    ["admin-gateway"]="curl -fsS http://localhost:8081/health"
    ["worker"]="docker compose -f deployments/docker-compose.yml ps worker | grep -q 'Up'"
    ["prometheus"]="curl -fsS http://localhost:9090/-/healthy"
    ["alertmanager"]="curl -fsS http://localhost:9093/-/healthy"
    ["caddy"]="curl -fsS -k https://localhost/health || curl -fsS http://localhost:80/health"
)

failed=()
for svc in "${!CHECKS[@]}"; do
    cmd="${CHECKS[$svc]}"
    echo "Checking $svc..."
    if timeout "$TIMEOUT" bash -c "$cmd" >/dev/null 2>&1; then
        echo "  OK"
    else
        echo "  FAIL"
        failed+=("$svc")
    fi
done

if [ ${#failed[@]} -gt 0 ]; then
    echo "FAILED services: ${failed[*]}"
    exit 1
fi
echo "All services healthy."
```

`chmod +x scripts/healthcheck.sh`.

**ПРОВЕРИТЬ:** реальные health endpoints на portal/admin gateway. `grep -rn "/health\|HealthCheck" cmd/portal-gateway/ cmd/admin-gateway/ internal/api/` — выдать real path. Если /health отсутствует — добавлять health-endpoint **out of scope** Plan 7 (отдельный task). Workaround: проверять через `docker compose ps | grep Up` для этих сервисов.

- [ ] **Step 2: Создать `scripts/deploy-prod.sh`**

```bash
#!/bin/bash
# Production deploy procedure.
# Использование: ./scripts/deploy-prod.sh [--service <name>]
# Без --service — full stack deploy.
set -euo pipefail

cd /opt/sms

# Pre-flight checks.
echo "=== Pre-flight ==="
test -f deployments/.env.prod || { echo ".env.prod missing — abort"; exit 1; }
test -d /var/backups/sms-pg || mkdir -p /var/backups/sms-pg

# Snapshot DB перед deploy.
echo "=== Backup DB ==="
SNAPSHOT_DIR="/var/backups/sms-pg/pre-deploy-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$SNAPSHOT_DIR"
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml exec -T postgres \
    pg_basebackup -U smpp -D - -Ft -P -X fetch | gzip > "$SNAPSHOT_DIR/base.tar.gz"
echo "Snapshot: $SNAPSHOT_DIR/base.tar.gz"

# Capture текущий git ref (для rollback).
PREV_REF=$(git rev-parse HEAD)
echo "$PREV_REF" > "$SNAPSHOT_DIR/prev_ref.txt"
echo "Previous ref: $PREV_REF (saved для rollback)"

# Pull latest.
echo "=== Git pull ==="
git fetch origin
TARGET_REF=${DEPLOY_REF:-$(git rev-parse origin/master)}
git checkout "$TARGET_REF"
echo "Deploying ref: $TARGET_REF"

# Apply migrations.
echo "=== Migrations ==="
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml exec -T migrate \
    migrate -path /migrations -database "$DATABASE_URL" up

# Deploy.
echo "=== Deploy ==="
SERVICE_FLAG=""
if [ "${1:-}" = "--service" ] && [ -n "${2:-}" ]; then
    SERVICE_FLAG="$2"
    docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml \
        up -d --build "$SERVICE_FLAG"
else
    docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml \
        up -d --build
fi

# Wait services + health check.
echo "=== Health check (60s grace) ==="
sleep 60
if ./scripts/healthcheck.sh; then
    echo "=== Deploy succeeded ==="
    echo "Snapshot retained: $SNAPSHOT_DIR (manual cleanup after 7 days)"
else
    echo "=== Health check FAILED — initiating rollback ==="
    ./scripts/rollback-prod.sh "$PREV_REF" "$SNAPSHOT_DIR"
    exit 1
fi
```

`chmod +x scripts/deploy-prod.sh`.

- [ ] **Step 3: Создать `scripts/rollback-prod.sh`**

```bash
#!/bin/bash
# Production rollback. Args: <previous_git_ref> [<snapshot_dir>]
# Если <snapshot_dir> не передан — только git revert + redeploy (миграции
# не откатываются). С snapshot_dir — full DB restore.
set -euo pipefail

cd /opt/sms

PREV_REF=${1:?previous git ref required}
SNAPSHOT_DIR=${2:-}

echo "=== Rollback to $PREV_REF ==="
git checkout "$PREV_REF"

if [ -n "$SNAPSHOT_DIR" ] && [ -f "$SNAPSHOT_DIR/base.tar.gz" ]; then
    echo "=== Restore DB from $SNAPSHOT_DIR ==="
    echo "WARNING: this will DROP current postgres state. Continue? (y/N)"
    read -r answer
    if [ "$answer" != "y" ]; then
        echo "Aborted DB restore. Code rollback only."
    else
        docker compose -f deployments/docker-compose.yml stop postgres
        docker volume rm sms_postgres_data_prod || true
        docker compose -f deployments/docker-compose.yml up -d postgres
        sleep 10
        zcat "$SNAPSHOT_DIR/base.tar.gz" | docker compose -f deployments/docker-compose.yml exec -T postgres tar -xf - -C /var/lib/postgresql/data
        docker compose -f deployments/docker-compose.yml restart postgres
    fi
fi

echo "=== Redeploy on previous ref ==="
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml up -d --build

sleep 30
./scripts/healthcheck.sh && echo "Rollback succeeded" || echo "Rollback FAILED — manual intervention required"
```

`chmod +x scripts/rollback-prod.sh`.

- [ ] **Step 4: Validate scripts (syntax + shellcheck)**

```bash
bash -n scripts/deploy-prod.sh scripts/rollback-prod.sh scripts/healthcheck.sh
docker run --rm -v "$(pwd)/scripts:/scripts" koalaman/shellcheck:latest /scripts/deploy-prod.sh /scripts/rollback-prod.sh /scripts/healthcheck.sh
```

Expected: no syntax errors. shellcheck warnings допустимы (некоторые false-positive в complex scripts).

- [ ] **Step 5: Commit**

```bash
git add scripts/deploy-prod.sh scripts/rollback-prod.sh scripts/healthcheck.sh
git commit -m "ops(deploy): production deploy + rollback scripts (Plan 7 Task 11)

deploy-prod.sh: pre-flight + pg_basebackup snapshot + git checkout +
migrate up + docker compose up + health check; auto-rollback при fail.
rollback-prod.sh: revert git ref + optional DB restore from snapshot.
healthcheck.sh: pings все критические сервисы за 60s timeout."
```

---

## Stage 5 — Cutover-day execution

### Task 12: Smoke-test script + canary procedure

**Контекст:** После deploy нужно подтвердить, что вся подсистема aggregator-routing работает: SRA-row создаётся на assignment, retry-loop работает, materialize success, audit-log пишется. Smoke = scripted black-box на E2E API уровень.

Canary: первые 24h после cutover только один client allowlisted в SMPP-gateway. Realization — env-flag `CANARY_CLIENT_IDS=<uuid>` в client-gateway/SMPP-gateway, который при non-empty rejects всех остальных. Спец logика — НЕ строить полный allowlist-фреймворк; просто early-return в submit_sm handler.

**Files:**
- Create: `scripts/smoke-prod.sh`
- Modify: `internal/gateway/client/handler.go` (или wherever submit_sm handler) — canary check
- Modify: `internal/smpp/server.go` (SMPP-side canary check)

- [ ] **Step 1: Найти submit-handler entry points**

```bash
grep -rn "func.*Submit\|SubmitSM\|HandleSubmitSM\|sendMessage" cmd/client-gateway/ internal/gateway/client/ internal/smpp/ internal/gateway/smpp_gateway/ 2>/dev/null | head -20
```

Зафиксировать: точные file:line entry points для (a) HTTP API send, (b) SMPP submit_sm.

- [ ] **Step 2: Implement canary guard в HTTP API entry**

В found file (например `internal/gateway/client/handler.go`), в начало handler-функции (после auth, до бизнес-логики):

```go
// Plan 7 Task 12: canary mode после prod cutover. Если CANARY_CLIENT_IDS
// non-empty, разрешать send только перечисленным client_id'ам. Снимать через
// `unset` env-var + restart после 24h успешной observation.
if canary := os.Getenv("CANARY_CLIENT_IDS"); canary != "" {
    allowed := false
    for _, id := range strings.Split(canary, ",") {
        if strings.TrimSpace(id) == clientID.String() {
            allowed = true
            break
        }
    }
    if !allowed {
        log.Warn().Str("client_id", clientID.String()).Str("canary_ids", canary).
            Msg("Canary mode: client not in allowlist, rejecting send")
        http.Error(w, "service in canary mode — please retry after 24h", http.StatusServiceUnavailable)
        return
    }
}
```

**ПРОВЕРИТЬ:** в чём конкретный тип возврата handler'а, где взять clientID (context? auth claims?). Implementer корректирует под факт.

- [ ] **Step 3: Implement canary guard в SMPP submit_sm handler**

В SMPP submit_sm handler (Step 1 нашёл location), аналогично:

```go
if canary := os.Getenv("CANARY_CLIENT_IDS"); canary != "" {
    if !isInCanaryAllowlist(canary, session.ClientID) {
        // SMPP error code: ESME_RX_T_APPN (0x00000064) — generic temporary failure.
        return smpp.ErrThrottling
    }
}
```

`isInCanaryAllowlist` — helper в новом файле `internal/smpp/canary.go` (или ближайший подходящий).

- [ ] **Step 4: Test canary guard**

Unit test для `isInCanaryAllowlist`:

```go
func TestCanaryAllowlist(t *testing.T) {
    tests := []struct {
        canaryEnv string
        clientID  string
        want      bool
    }{
        {"", "any", true},  // empty env — все разрешены (canary off)
        {"abc-123", "abc-123", true},
        {"abc-123,def-456", "def-456", true},
        {"abc-123", "xyz-999", false},
        {" abc-123 ", "abc-123", true},  // whitespace tolerance
    }
    for _, tt := range tests {
        got := isInCanaryAllowlist(tt.canaryEnv, tt.clientID)
        require.Equal(t, tt.want, got, "canary=%q client=%q", tt.canaryEnv, tt.clientID)
    }
}
```

Run:
```bash
./scripts/server.sh exec "cd /opt/sms && docker compose -f deployments/docker-compose.yml exec -T dev go test -buildvcs=false -run TestCanaryAllowlist ./internal/smpp/..."
```

Expected: PASS.

- [ ] **Step 5: Создать `scripts/smoke-prod.sh`**

```bash
#!/bin/bash
# Production smoke test — black-box после deploy.
# Требует: PORTAL_DOMAIN, ADMIN_EMAIL, ADMIN_PASSWORD в env.
set -euo pipefail

PORTAL_URL="https://${PORTAL_DOMAIN}"
ADMIN_EMAIL=${ADMIN_EMAIL:?required}
ADMIN_PASSWORD=${ADMIN_PASSWORD:?required}

echo "=== Smoke: TLS reachable ==="
curl -fsS -o /dev/null "$PORTAL_URL/health" || { echo "TLS or portal unreachable"; exit 1; }

echo "=== Smoke: Admin login ==="
TOKEN=$(curl -fsS -X POST "$PORTAL_URL/portal/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" \
    | jq -r '.token')
test -n "$TOKEN" || { echo "Login failed"; exit 1; }

echo "=== Smoke: SRA-stuck endpoint reachable ==="
curl -fsS -H "Authorization: Bearer $TOKEN" "$PORTAL_URL/portal/v1/admin/network/sra-stuck" \
    | jq '.rows | length' >/dev/null

echo "=== Smoke: Prometheus scraping все targets ==="
# Через portal-gateway не имеем доступа к internal:9090. Если prometheus
# доступен через admin-domain — проверка тут. Иначе — manual через ssh.
echo "   (manual: ssh prod-host && curl localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.health!=\"up\")')"

echo "=== Smoke: Все алерты в Alertmanager — sane ==="
echo "   (manual: ssh prod-host && curl localhost:9093/api/v2/alerts | jq '[.[] | .labels.alertname]')"

echo "=== Smoke: Worker partition maintenance log ==="
echo "   (manual: ./scripts/server.sh logs worker | grep 'partition maintenance')"

echo ""
echo "=== AUTOMATED smoke OK ==="
echo "Now run MANUAL checks listed above."
```

`chmod +x scripts/smoke-prod.sh`.

- [ ] **Step 6: Commit**

```bash
git add scripts/smoke-prod.sh internal/gateway/client/handler.go internal/smpp/canary.go internal/smpp/canary_test.go
git commit -m "feat(canary): canary mode + smoke-prod script (Plan 7 Task 12)

CANARY_CLIENT_IDS env-var активирует allowlist в HTTP send + SMPP submit_sm.
Empty/unset → canary off (все разрешены). После cutover 24h observation
ops снимает env и restart.

scripts/smoke-prod.sh: TLS + login + SRA-stuck endpoint + manual checklist."
```

---

### Task 13: cutover-runbook.md

**Контекст:** Полная процедура cutover-day от ops-perspective. Markdown checklist, который ops-человек выполняет шаг за шагом. Включает все pre-/during-/post-checks.

**Files:**
- Create: `docs/ops/prod-cutover-runbook.md`

- [ ] **Step 1: Создать `docs/ops/prod-cutover-runbook.md`**

```markdown
# Production Cutover Runbook

**Audience:** ops-инженер, выполняющий cutover. Один проход через документ — один cutover.

**Estimated duration:** 2-4 часа (включая 60min observation buffer перед canary release).

## Pre-cutover (за 24-48h до)

- [ ] **VM provisioned** — указанная host'ом cutover'а. Минимум: 8 vCPU, 16GB RAM, 200GB SSD, Ubuntu 22.04 LTS.
- [ ] **DNS A-record** для `${PORTAL_DOMAIN}` указывает на VM IP, TTL ≤ 300s.
- [ ] **Firewall на VM**: iptables/ufw разрешает 22 (SSH from ops bastion only), 80 + 443 (public), 2775 + 2776 (SMPP — IP-allowlist по согласованию с клиентами). Всё остальное — DROP.
- [ ] **Docker + docker-compose установлены**: `docker --version` ≥ 24.0, `docker compose version` ≥ 2.20.
- [ ] **SMTP-relay credentials получены** — `ALERTMANAGER_SMTP_*` готовы вписать в .env.prod.
- [ ] **`/opt/sms` склонирован**:
  ```
  ssh user@<vm>
  sudo mkdir -p /opt/sms && sudo chown $USER /opt/sms
  cd /opt && git clone https://github.com/example/sms.git
  ```
- [ ] **`.env.prod` собран** на основе `.env.prod.example`, secrets generated через `openssl rand`.
- [ ] **`.env.prod` копирован на VM** в `/opt/sms/deployments/.env.prod`, `chmod 600`.
- [ ] **PORTAL_ADMIN credentials decided** — какой email + initial password для `cmd/seed-admin`.
- [ ] **Backup destination prepared** — `/var/backups/sms-pg` существует, `chown` на deploy-user.
- [ ] **Redis firewall script подготовлен** — `scripts/redis-firewall.sh` reachable, sudo verified.

## Cutover Hour 0 — initial deploy

- [ ] T+0: SSH на prod VM, `cd /opt/sms`.
- [ ] T+0: `git checkout master && git pull` (свежий ref).
- [ ] T+5: First-time bring-up:
  ```
  docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod up -d postgres redis
  sleep 30
  ```
- [ ] T+10: Apply migrations:
  ```
  docker compose ... run --rm migrate migrate -path /migrations -database "$DATABASE_URL" up
  ```
  Expected: `144/u sra_retry_cap_index` или последняя миграция, БЕЗ error. Если error — abort, не продолжать.
- [ ] T+15: Bootstrap admin user:
  ```
  docker compose ... exec dev go run ./cmd/seed-admin --email "$ADMIN_EMAIL" --org-name "ProdOrg" --role admin --force-password-change true
  ```
  Ввести password интерактивно.
- [ ] T+20: Bring up rest:
  ```
  docker compose -f ... -f docker-compose.prod.yml --env-file .env.prod up -d
  ```
- [ ] T+25: `./scripts/healthcheck.sh` — все зелёные.
- [ ] T+30: Apply Redis firewall:
  ```
  sudo bash scripts/redis-firewall.sh
  sudo apt-get install -y iptables-persistent && sudo netfilter-persistent save
  ```
  Verify: `nc -zv <vm-ip> 6379 -w 3` от ops bastion → timeout (DROP).
- [ ] T+35: Verify TLS:
  ```
  curl -fsSv https://${PORTAL_DOMAIN}/health
  ```
  (caddy auto-Let's Encrypt — первый запрос на /health тригерит cert request; certificate в течение 10s).
- [ ] T+40: Login через UI: `https://${PORTAL_DOMAIN}/admin` → admin email/password (форсированная смена пароля при первом входе).

## Cutover Hour 1 — canary release

- [ ] T+60: Создать canary client через UI:
  - Sidebar → Clients → Create.
  - Email: `canary@<owner-domain>`, password generated.
  - Verify: client_id отображается в UI.
- [ ] T+65: Set `CANARY_CLIENT_IDS=<canary-client-uuid>` в `.env.prod`, restart gateways:
  ```
  vim deployments/.env.prod   # add CANARY_CLIENT_IDS=...
  docker compose ... up -d portal-gateway client-gateway-1 client-gateway-2 admin-gateway-1 admin-gateway-2 smpp-gateway smpp-server
  ```
- [ ] T+70: Smoke test:
  ```
  PORTAL_DOMAIN=portal.example.com ADMIN_EMAIL=... ADMIN_PASSWORD=... ./scripts/smoke-prod.sh
  ```
- [ ] T+75: Manual smoke — отправить SMS через canary client (HTTP API):
  ```
  curl -X POST https://${PORTAL_DOMAIN}/api/v1/messages \
      -H "Authorization: Bearer <canary-client-token>" \
      -H 'Content-Type: application/json' \
      -d '{"to":"+77001234567","text":"Cutover smoke","sender":"TestSender"}'
  ```
  Expected: HTTP 202 + message_id.
- [ ] T+80: Verify в Grafana SRA-aggregator dashboard:
  - `portal_sra_pending_retry_count` = 0 (или малое).
  - `portal_sra_retry_give_up_count` = 0.
  - `portal_sra_materialize_failure_total` rate ~0.
- [ ] T+90: Verify alertmanager не fire'ится паникой:
  ```
  curl -s http://localhost:9093/api/v2/alerts | jq '.[] | select(.status.state=="active") | .labels.alertname'
  ```
  Expected: пусто или ожидаемые (например `Watchdog` alertmanager built-in).

## Cutover Hour 1-24 — observation

- [ ] T+120: 30min check на dashboard.
- [ ] T+360 (6h): repeat check.
- [ ] T+720 (12h): repeat check.
- [ ] T+1440 (24h): repeat check + verify pg_basebackup ran (Task 14):
  ```
  ls -lh /var/backups/sms-pg/daily-*.tar.gz | tail -3
  ```
  Expected: один свежий снапшот за последние 24h.

## Cutover Hour 24+ — full release

- [ ] Verify zero alerts fired за 24h:
  ```
  ssh prod && docker logs alertmanager 2>&1 | grep -i "sent" | head
  ```
- [ ] Remove canary:
  ```
  vim deployments/.env.prod   # CANARY_CLIENT_IDS=
  docker compose ... up -d portal-gateway client-gateway-1 client-gateway-2 admin-gateway-1 admin-gateway-2 smpp-gateway smpp-server
  ```
- [ ] Onboard real clients через UI (sidebar → Clients → Create).
- [ ] Communicate go-live ops + clients.

## Abort criteria (rollback trigger)

Любое из:
- Health check не зелёный спустя 10min after deploy.
- Alertmanager fired `SRARetryGiveUpRows` или `SRAMaterializeInitialFailureRateProd` в первые 60min.
- Manual SMS-send из canary failed (HTTP non-2xx или message stuck в pending > 5min).
- Database errors в logs `portal-gateway`/`worker`.

**Procedure:**
```
./scripts/rollback-prod.sh <prev_ref> <snapshot_dir>
```

Если rollback тоже failed — escalate ops on-call, manual remediation.

## Post-cutover — week 1

- Daily: Grafana dashboard review, проверить нет ли trend'ов в metrics.
- Day 3: intentional alert-fire test (Task 14), verify email доехал.
- Day 7: backup restore test (Task 14), verify pg_basebackup actually restorable.
```

- [ ] **Step 2: Commit**

```bash
git add docs/ops/prod-cutover-runbook.md
git commit -m "docs(ops): production cutover runbook (Plan 7 Task 13)

Шаг-за-шагом checklist для ops: pre-cutover, hour 0 (initial deploy),
hour 1 (canary), hour 1-24 (observation), hour 24+ (full release).
Abort criteria + rollback trigger. Predicted duration: 2-4h initial,
24h до full release."
```

---

## Stage 6 — Post-cutover hardening

### Task 14: pg_basebackup daily cron + restore-test + intentional-alert procedure

**Контекст:** Daily snapshot нужен independently от deploy-time snapshots. Restore-test — раз в неделю автоматически проверяем, что backup actually restorable. Intentional alert-fire — manual procedure для verification, что email доходит.

**Files:**
- Create: `scripts/backup-pg-daily.sh`
- Create: `scripts/restore-test.sh`
- Modify: `deployments/docker-compose.prod.yml` (добавить cron service)
- Create: `docs/ops/intentional-alert-test.md`

- [ ] **Step 1: Создать `scripts/backup-pg-daily.sh`**

```bash
#!/bin/bash
set -euo pipefail
cd /opt/sms

BACKUP_DIR="/var/backups/sms-pg"
RETENTION_DAYS=${PG_BACKUP_RETENTION_DAYS:-7}
mkdir -p "$BACKUP_DIR"

DATE=$(date +%Y%m%d-%H%M%S)
TARGET="$BACKUP_DIR/daily-$DATE.tar.gz"

echo "=== pg_basebackup → $TARGET ==="
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml \
    exec -T postgres pg_basebackup -U smpp -D - -Ft -P -X fetch | gzip > "$TARGET"

# Verify size — пустой/маленький snapshot = тревога.
SIZE=$(stat -c%s "$TARGET")
if [ "$SIZE" -lt 1048576 ]; then  # < 1MB — backup битый
    echo "ERROR: backup size $SIZE bytes < 1MB — likely corrupted"
    exit 1
fi

echo "Snapshot OK: $TARGET ($((SIZE / 1024 / 1024)) MB)"

# Retention cleanup.
find "$BACKUP_DIR" -name 'daily-*.tar.gz' -mtime +"$RETENTION_DAYS" -delete
echo "Retention cleanup: removed snapshots older than $RETENTION_DAYS days"
```

`chmod +x scripts/backup-pg-daily.sh`.

- [ ] **Step 2: Создать `scripts/restore-test.sh`**

```bash
#!/bin/bash
# Weekly restore test: poднимает изолированный postgres-instance, restore'ит
# самый свежий snapshot, проверяет что row count > 0 в нескольких таблицах.
# Цель: верифицировать, что pg_basebackup actually restorable.
set -euo pipefail
cd /opt/sms

BACKUP_DIR="/var/backups/sms-pg"
LATEST=$(ls -t "$BACKUP_DIR"/daily-*.tar.gz 2>/dev/null | head -1)
test -n "$LATEST" || { echo "No backup found — abort"; exit 1; }

echo "=== Restore test: $LATEST ==="

# Spin up isolated postgres.
TEST_NAME="pg-restore-test-$(date +%s)"
TEST_DATA_DIR=$(mktemp -d)
docker run -d --name "$TEST_NAME" \
    -v "$TEST_DATA_DIR:/var/lib/postgresql/data" \
    -e POSTGRES_PASSWORD=test \
    -e POSTGRES_USER=smpp \
    -e POSTGRES_DB=smpp_db \
    postgres:15

sleep 15  # Postgres init.

# Restore.
docker stop "$TEST_NAME"
zcat "$LATEST" | docker run --rm -i -v "$TEST_DATA_DIR:/var/lib/postgresql/data" alpine tar -xf - -C /var/lib/postgresql/data
docker start "$TEST_NAME"
sleep 10

# Verify несколько критических таблиц.
for tbl in users orgs subaccount_routing_assignment; do
    COUNT=$(docker exec "$TEST_NAME" psql -U smpp -d smpp_db -tAc "SELECT count(*) FROM $tbl")
    if [ "$COUNT" -lt 0 ]; then
        echo "ERROR: $tbl count = $COUNT (negative? table missing?)"
        docker rm -f "$TEST_NAME"
        rm -rf "$TEST_DATA_DIR"
        exit 1
    fi
    echo "  $tbl: $COUNT rows"
done

docker rm -f "$TEST_NAME"
rm -rf "$TEST_DATA_DIR"

echo "=== Restore test PASSED ==="
```

`chmod +x scripts/restore-test.sh`.

- [ ] **Step 3: Wire cron в host crontab (manual step) ИЛИ docker cron service**

Опция A (хост crontab — проще):

В `docs/ops/prod-cutover-runbook.md` Pre-cutover список добавить:

```
- [ ] Cron jobs:
  ```
  sudo tee /etc/cron.d/sms-pg-backup <<EOF
  # Daily pg_basebackup at 03:00.
  0 3 * * * deploy-user cd /opt/sms && ./scripts/backup-pg-daily.sh >> /var/log/sms-backup.log 2>&1
  # Weekly restore test on Sunday 04:00.
  0 4 * * 0 deploy-user cd /opt/sms && ./scripts/restore-test.sh >> /var/log/sms-restore-test.log 2>&1
  EOF
  ```

Опция B (docker container — изоляция, но добавляет deps): отвергаю — cron в alpine container ломкий, host crontab надёжнее.

Implementer выбирает A, документирует в runbook (см. модификацию выше — добавить step в Task 13 runbook).

- [ ] **Step 4: Создать `docs/ops/intentional-alert-test.md`**

```markdown
# Intentional Alert Fire Test

## Когда применять

- Day 3 после cutover — verify, что alert→email pipeline работает.
- После любого изменения alertmanager config или SMTP credentials.

## Procedure

### Method A — manually fire через alertmanager API

```bash
ssh prod-host
curl -X POST http://localhost:9093/api/v2/alerts -H 'Content-Type: application/json' -d '[
  {
    "labels": {
      "alertname": "IntentionalTestAlert",
      "severity": "warning",
      "env": "prod",
      "component": "ops-test"
    },
    "annotations": {
      "summary": "Intentional test alert (Plan 7 verification)",
      "description": "If you receive this email, alertmanager → SMTP → ops-mailbox pipeline works."
    },
    "startsAt": "'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'"
  }
]'
```

Wait 30s. Check ops mailbox. Expected: email с subject `[SMS-Platform prod] IntentionalTestAlert (firing)`.

После 5min — alert auto-resolve (default `resolve_timeout=5m`). Получишь второй email `(resolved)`.

### Method B — реальный alert через scale-up retry-count

```sql
-- На prod postgres:
INSERT INTO subaccount_routing_assignment (client_id, last_materialize_error_at, materialize_retry_count)
SELECT gen_random_uuid(), now(), 100
FROM generate_series(1, 6);
```

Это создаст 6 stuck rows c retry_count=100 → SRARetryGiveUpRows alert (threshold 5) fire'ится в течение 5min. Cleanup:
```sql
DELETE FROM subaccount_routing_assignment WHERE materialize_retry_count = 100 AND client_id IN (<test-uuids>);
```

**Использовать только Method A на prod.** Method B — для staging.

## Если email не пришёл

1. Check alertmanager logs: `docker logs alertmanager 2>&1 | tail -50` — ошибки SMTP.
2. Verify SMTP credentials: `telnet $ALERTMANAGER_SMTP_HOST` от prod-host.
3. Check spam folder.
4. Re-check alertmanager.yml routes — env=prod не должен матчить log-only маршрут.
```

- [ ] **Step 5: Commit**

```bash
git add scripts/backup-pg-daily.sh scripts/restore-test.sh docs/ops/intentional-alert-test.md docs/ops/prod-cutover-runbook.md
git commit -m "ops(prod): backup cron + restore test + alert verification (Plan 7 Task 14)

backup-pg-daily.sh: pg_basebackup в /var/backups/sms-pg/daily-*.tar.gz,
retention 7 days. Verify size > 1MB.
restore-test.sh: spins up isolated postgres-15, restore latest, verify
critical tables non-empty. Запускается weekly через host crontab.
intentional-alert-test.md: manual alertmanager fire procedure для
day-3 verification, что email pipeline жив."
```

---

## Финальная Self-Review (для plan author)

Spec coverage check:

| Stage / Topic | Task |
|---|---|
| Sandbox foundation | T1, T2, T3 |
| Prod env (Caddy/TLS) | T4 |
| Prod compose + secrets | T5 |
| Audit_log retention (Spec H2) | T6 |
| Prod admin seed | T7 |
| Per-env alert thresholds (Spec B2) | T8 |
| Alertmanager (Spec B3) | T9 |
| Grafana provisioning prod-ready | T10 |
| Deploy/rollback scripts (Spec B1 doc, P6 deploy strategy) | T3, T11 |
| Smoke + canary (Spec P4) | T12 |
| Cutover runbook | T13 |
| Backup + restore-test + alert verify (P7) | T14 |
| Multi-replica safety (Spec A1) | T2 |

**Что НЕ покрыто (out-of-scope, как и было решено):**
- A2 bulk ResourceID structured detail — WONTFIX до ops запроса.
- A3 toast.warning — отдельный UX-sweep.
- A4 E2E в CI — Plan 8.
- C1 orphan-trigger guard — perceived risk low; добавлено в memory как known caveat.
- C2/C3 SRA-stuck UX — отложено до реального запроса.
- D1/D2 architecture — brainstorm.
- E1/E2/E3 spec OUT-of-scope — brainstorm.
- F1 RU operator preview — brainstorm с product-input.
- G2 ESLint ratchet — параллельная мини-итерация.
- H1 OpenTelemetry — brainstorm.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-06-aggregator-routing-plan-7.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, two-stage review (spec + code-quality), fast iteration. Соответствует workflow Plans 1-6.

**2. Inline Execution** — выполнить tasks в этой сессии через executing-plans, batch execution с checkpoint-review каждые 3 task'а.

**Which approach?**

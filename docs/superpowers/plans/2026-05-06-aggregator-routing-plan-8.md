# Plan 8 — Pre-cutover hardening: TLS expansion + cutover-readiness polish

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть последние pre-cutover gap'ы Plan 7 — расширить Caddy TLS на все gateway-порты (client/admin/portal-gateway HTTP+gRPC), сделать DB-restore автоматическим в rollback-prod.sh, упрочнить seed-admin против дефолтных паролей и автоматизировать canary-rejection в smoke-prod. После Plan 8 онбординг публичных клиентов в день cutover'а перестаёт быть рисковым.

**Architecture:** Caddy 2.8 расширяется на gateway-порты; HaProxy остаётся как L7 балансер за Caddy (Caddy 443 → HaProxy 8080/8081/8082 → client/admin/portal-gateway). gRPC-порты client-gateway/admin-gateway получают TLS через Caddy `reverse_proxy` с `transport http { versions h2c }` (gRPC over HTTP/2). HaProxy stats:8404 убирается с published — теперь только через SSH-tunnel. seed-admin перестаёт принимать дефолтный пароль и обязывает `ADMIN_PASSWORD` env (минимум 16 символов). `rollback-prod.sh` получает реальный embed offline-restore (stop postgres → alpine tar -xz в volume → start). `smoke-prod.sh` регистрирует второй (non-canary) client и проверяет, что send возвращает Unavailable=503 — без ручного шага в runbook'е.

**Tech Stack:** Caddy 2.8-alpine, HaProxy 2.8 (existing), Docker Compose 2.24+, Go 1.24.0 (seed-admin hardening), bash (rollback/smoke). Без новых зависимостей.

---

## Контекст: что уже есть

- `deployments/docker-compose.prod.yml` — overlay с Caddy перед `portal-gateway` (только portal). Остальные gateway-порты (8080/8081/8082/8083/9090/19095/8404) **published** через HaProxy без TLS. Это явно зафиксировано в `.env.prod.example:55-59` как Plan 8 gap.
- `deployments/configs/caddy/Caddyfile` — единственный virtual host: `{$PORTAL_DOMAIN}` → `portal-gateway:8082`.
- `deployments/configs/haproxy-new.cfg` — frontend'ы на 8080 (client_http), 8081 (admin_http), 8082 (portal_http), 8083 (portal_frontend), 9090 (client_grpc, mode tcp), 9091 (admin_grpc, mode tcp), 8404 (stats).
- `scripts/rollback-prod.sh:23-46` — печатает manual procedure, требует `y/N` confirm, в non-interactive auto-skip'ает DB restore.
- `scripts/restore-test.sh:30-37` — рабочий offline-restore алгоритм, который надо переиспользовать в rollback.
- `cmd/seed-admin/main.go:19,31` — `defaultPassword = "Admin123!"`, fallback в `envOr("ADMIN_PASSWORD", defaultPassword)`. Не fail-fast.
- `scripts/smoke-prod.sh` — 3 проверки (TLS, login, SRA-stuck endpoint). Canary rejection — manual step в runbook'е T+75.
- `internal/gateway/canary/canary.go` — `EnvVar = "CANARY_CLIENT_IDS"`, `IsAllowed(clientID string) bool`. Возвращает 503 Unavailable в client-gateway gRPC-сервере на `SendMessage` и `SendBulkMessages`.

## Подход: что и почему

**Почему Caddy перед HaProxy, а не Caddy вместо HaProxy.** HaProxy уже несёт healthcheck, balancing между client-gateway-1/-2 и admin-gateway-1/-2, mode tcp для gRPC frontends. Заменять HaProxy = переписать всю балансировку под Caddy `reverse_proxy` lb_policy. Цена не оправдана. Caddy фронтит TLS + HSTS, HaProxy продолжает балансировать L7. Жертва: один лишний hop в request path; перевешивает: minimal change, HaProxy stats остаются.

**Почему grpc через Caddy, а не отдельный TLS-frontend в HaProxy.** HaProxy `bind *:9090 ssl crt ...` требует ручного управления сертификатами, ротации, синхронизации с Caddy LE. Caddy `reverse_proxy h2c` работает out-of-the-box, переиспользует существующий ACME-flow. Жертва: gRPC переключается с plaintext на h2c за TLS, клиенты должны использовать TLS-aware gRPC-channel; перевешивает: единый ACME, единая cert-store.

**Stats endpoint.** HaProxy stats:8404 — internal-only после Plan 8. Доступ через SSH-tunnel (как Prometheus/Grafana). Не TLS-фронтить — overkill для одного админа.

**DB restore embed.** Алгоритм уже в `restore-test.sh:30-37` — переиспользовать с минимальной адаптацией (TEST_NAME → реальный postgres-контейнер).

**seed-admin fail-fast.** Default `Admin123!` оставить нельзя в prod — security audit fail. Решение: env `ADMIN_PASSWORD` обязателен и >= 16 символов. ADMIN_USERNAME/EMAIL остаются с дефолтами (не security-критичны).

## Вне scope

- **Multi-replica advisory-lock REDO** (A1 в брифе) — Plan 9, после реального scale-up trigger'а.
- **Audit-log retention auto-drop** (A4) — Plan 9, после 6+ месяцев реальных audit-данных.
- **Mandatory password change на login** (B1 UI часть) — Plan 9, требует UI flow.
- **E2E tests в CI** (C1), **race detector** (C2), **ESLint ratchet** (C3) — backlog к будущим UI/test-плану.
- **Post-cutover планы** (D1, D2) — после реального cutover'а.
- **Архитектурные** (E1-E6) — отдельные brainstorm'ы по trigger'ам.

---

## Files

**Modify:**
- `deployments/configs/caddy/Caddyfile` — добавить виртуальные хосты для api/admin/grpc subdomain'ов.
- `deployments/docker-compose.prod.yml` — `haproxy:` ports `!override` (убрать 8080/8081/8082/8083/9090/19095/8404 публикацию, оставить пусто или 80/443 Caddy уже есть). Добавить depends_on caddy → haproxy.
- `deployments/.env.prod.example` — добавить `API_DOMAIN`, `ADMIN_DOMAIN`, `GRPC_DOMAIN` переменные; убрать KNOWN GAP блок.
- `scripts/rollback-prod.sh` — заменить manual-print блок на embedded offline restore.
- `scripts/smoke-prod.sh` — добавить Smoke 4 (canary rejection) + Smoke 5 (api.* TLS reachable).
- `scripts/deploy-prod.sh` — добавить image pre-pull шаг перед `up -d --build`.
- `cmd/seed-admin/main.go:19-31` — убрать defaultPassword, fail-fast при пустом/коротком ADMIN_PASSWORD.
- `cmd/seed-admin/main_test.go` — новый файл с tests на validation.
- `docs/runbooks/cutover-prod.md` (если существует) — обновить step T+75 (canary rejection теперь в smoke автоматически).

**Verify (read-only sanity check):**
- `deployments/configs/haproxy-new.cfg` — frontend'ы 8080/8081 теперь обслуживают только internal docker-network трафик от Caddy.

---

## Tasks

### Task 1: seed-admin fail-fast против дефолтного пароля

**Files:**
- Modify: `cmd/seed-admin/main.go:16-21,29-31`
- Create: `cmd/seed-admin/main_test.go`

**Rationale:** Default `Admin123!` в prod = критическая дыра. Если оператор забыл задать `ADMIN_PASSWORD` — silently создаётся admin с известным паролем. Fail-fast устраняет ошибку оператора.

- [ ] **Step 1: Refactor — выделить validation в чистую функцию**

В `cmd/seed-admin/main.go` после `envOr` добавить:

```go
// validateAdminPassword проверяет, что пароль задан и достаточно длинный.
// Возвращает ошибку с описанием проблемы — main печатает и exit'ит.
//
// Минимум 16 символов: не bcrypt-стойкость, а защита от очевидных слабых
// паролей (Admin123!, password, 12345678). Для bcrypt cost=10 16 случайных
// символов = ~96 бит энтропии, достаточно.
func validateAdminPassword(pw string) error {
    if pw == "" {
        return fmt.Errorf("ADMIN_PASSWORD env var is required (no default in production)")
    }
    if len(pw) < 16 {
        return fmt.Errorf("ADMIN_PASSWORD must be at least 16 characters (got %d)", len(pw))
    }
    return nil
}
```

Удалить константу `defaultPassword = "Admin123!"`. Изменить `password := envOr("ADMIN_PASSWORD", defaultPassword)` → `password := os.Getenv("ADMIN_PASSWORD")`. Сразу после — вызвать `validateAdminPassword`:

```go
password := os.Getenv("ADMIN_PASSWORD")
if err := validateAdminPassword(password); err != nil {
    log.Fatalf("seed-admin: %v", err)
}
```

- [ ] **Step 2: Test для validation**

Создать `cmd/seed-admin/main_test.go`:

```go
package main

import "testing"

func TestValidateAdminPassword(t *testing.T) {
    tests := []struct {
        name    string
        pw      string
        wantErr bool
    }{
        {"empty", "", true},
        {"too short", "Short1!", true},
        {"exactly 15", "123456789012345", true},
        {"exactly 16", "1234567890123456", false},
        {"long random", "aB3$xYz9!qWe7&rT", false},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            err := validateAdminPassword(tc.pw)
            if tc.wantErr && err == nil {
                t.Errorf("validateAdminPassword(%q) = nil, want error", tc.pw)
            }
            if !tc.wantErr && err != nil {
                t.Errorf("validateAdminPassword(%q) = %v, want nil", tc.pw, err)
            }
        })
    }
}
```

- [ ] **Step 3: Run tests локально через dev-container на сервере**

Тесты cmd/seed-admin не требуют DB — можно прогнать локально:

Run на сервере:
```
docker compose -f deployments/docker-compose.yml exec -T dev go test -buildvcs=false ./cmd/seed-admin/...
```

Expected: PASS (5 sub-tests).

Если падает с "Device Guard" локально на Windows — игнорировать, тест пройдёт на сервере и в CI.

- [ ] **Step 4: Verify build не сломан**

Run на сервере:
```
docker compose -f deployments/docker-compose.yml exec -T dev go build -buildvcs=false ./cmd/seed-admin/
```

Expected: exit 0, бинарь собирается.

- [ ] **Step 5: Update prod-admin-bootstrap doc если есть**

Проверить:
```
test -f docs/ops/prod-admin-bootstrap.md && grep -n "ADMIN_PASSWORD\|Admin123" docs/ops/prod-admin-bootstrap.md
```

Если файл существует и упоминает `Admin123!` — заменить на инструкцию `openssl rand -base64 24` для генерации пароля. Если файла нет — skip.

- [ ] **Step 6: Commit**

```
git status
git add cmd/seed-admin/main.go cmd/seed-admin/main_test.go
# если bootstrap doc обновлён — git add docs/ops/prod-admin-bootstrap.md
git commit -m "fix(seed-admin): require ADMIN_PASSWORD env, min 16 chars (Plan 8 Task 1)

Default password 'Admin123!' was a known security gap pre-cutover.
seed-admin now fails fast if ADMIN_PASSWORD is unset or shorter than
16 characters, removing the silent-default risk."
```

---

### Task 2: Caddy TLS expansion на client/admin gateways (HTTP)

**Files:**
- Modify: `deployments/configs/caddy/Caddyfile`
- Modify: `deployments/.env.prod.example`

**Rationale:** Сейчас `8080` (client_http), `8081` (admin_http), `8082` (portal_http через portal-gateway, уже под Caddy), `8083` (portal_frontend) — published без TLS. Public client'ы день-1 = MitM-риск. Решение: Caddy term'ит TLS на subdomain'ах `api.$PORTAL_DOMAIN`, `admin.$PORTAL_DOMAIN`, проксирует на HaProxy через docker network.

- [ ] **Step 1: Добавить переменные в .env.prod.example**

В `deployments/.env.prod.example` после строки `LETSENCRYPT_EMAIL=ops@example.com`:

```
# Subdomains for gateway TLS (Plan 8). DNS A-records требуются ДО первого
# `up -d` — иначе Let's Encrypt HTTP-01 challenge fails.
# Verify: dig +short api.$PORTAL_DOMAIN admin.$PORTAL_DOMAIN grpc.$PORTAL_DOMAIN
API_DOMAIN=api.example.com
ADMIN_DOMAIN=admin.example.com
GRPC_DOMAIN=grpc.example.com
```

И удалить блок `# === KNOWN GAP ===` (строки 55-59 в текущей версии — после Plan 8 gap closed).

- [ ] **Step 2: Расширить Caddyfile на api + admin subdomain'ы**

Полностью переписать `deployments/configs/caddy/Caddyfile` (текущий 1-host, новый 3 HTTP-host'а; gRPC-host добавится в Task 3):

```
# Caddy auto-TLS на portal/api/admin subdomain'ы. gRPC TLS — отдельный
# host в Task 3 (Plan 8). Stats HaProxy 8404 теперь доступен только через
# SSH-tunnel и НЕ TLS-фронтится — internal-only admin tool.
#
# DNS PREREQUISITE: A-records для PORTAL_DOMAIN, API_DOMAIN, ADMIN_DOMAIN,
# GRPC_DOMAIN указывают на VM IP ДО первого `up -d`. Verify:
#   for d in $PORTAL_DOMAIN $API_DOMAIN $ADMIN_DOMAIN $GRPC_DOMAIN; do
#     echo -n "$d: "; dig +short $d
#   done
#
# LE STAGING TOGGLE для smoke: раскомментировать `acme_ca` ниже на dry-run.
# После validate restart caddy с боевым acme_ca (default).

(security_headers) {
	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "strict-origin-when-cross-origin"
		Permissions-Policy "geolocation=(), microphone=(), camera=(), payment=()"
		-Server
	}
}

# Portal SPA + portal-gateway API (existing).
{$PORTAL_DOMAIN} {
	tls {$LETSENCRYPT_EMAIL}
	# acme_ca https://acme-staging-v02.api.letsencrypt.org/directory

	reverse_proxy portal-gateway:8082 {
		header_up X-Real-IP {remote_host}
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Proto {scheme}
	}

	import security_headers
	log {
		output stdout
		format json
	}
	encode gzip
}

# Client API → HaProxy → client-gateway-1/2 на :8080 (HTTP).
{$API_DOMAIN} {
	tls {$LETSENCRYPT_EMAIL}
	# acme_ca https://acme-staging-v02.api.letsencrypt.org/directory

	reverse_proxy haproxy:8080 {
		header_up X-Real-IP {remote_host}
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Proto {scheme}
	}

	import security_headers
	log {
		output stdout
		format json
	}
	encode gzip
}

# Admin API → HaProxy → admin-gateway-1/2 на :8081 (HTTP).
{$ADMIN_DOMAIN} {
	tls {$LETSENCRYPT_EMAIL}
	# acme_ca https://acme-staging-v02.api.letsencrypt.org/directory

	reverse_proxy haproxy:8081 {
		header_up X-Real-IP {remote_host}
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Proto {scheme}
	}

	import security_headers
	log {
		output stdout
		format json
	}
	encode gzip
}
```

- [ ] **Step 3: Validate Caddyfile синтаксис локально**

В Windows нет caddy в PATH, но container валидирует на старте. Проверим через docker:

Run на сервере:
```
docker run --rm -v $(pwd)/deployments/configs/caddy/Caddyfile:/etc/caddy/Caddyfile:ro caddy:2.8-alpine caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
```

Expected: `Valid configuration` (с warning'ами про unset env-vars — это OK, runtime-substitution).

Если `Valid configuration` не выведено — вычитать errors, исправить синтаксис, retry.

- [ ] **Step 4: Commit Task 2**

```
git status
git add deployments/configs/caddy/Caddyfile deployments/.env.prod.example
git commit -m "feat(caddy): TLS expansion на api/admin gateway-subdomain'ы (Plan 8 Task 2)

Caddy теперь term'ит TLS на api.\$PORTAL_DOMAIN и admin.\$PORTAL_DOMAIN
помимо portal. Закрывает основной pre-cutover security gap: client/admin
HTTP API больше не нуждаются в plaintext-published port'ах после Task 4."
```

---

### Task 3: Caddy h2c reverse-proxy для gRPC

**Files:**
- Modify: `deployments/configs/caddy/Caddyfile` (добавить host блок)

**Rationale:** Client-gateway gRPC (9090) и admin-gateway gRPC (9091, published 19095) — единственный надёжный канал для high-throughput SMS-send из enterprise-клиентов. Без TLS = MitM на токены. Caddy умеет h2c (HTTP/2 cleartext) reverse-proxy с TLS на frontend, plain h2c на backend. HaProxy mode tcp балансирует h2c между gateway-инстансами.

**Path discrimination:** Один subdomain `grpc.$PORTAL_DOMAIN` обслуживает оба gRPC (client + admin) — разделение через path `/SMSService.*` → client, `/AdminService.*` → admin. Имена сервисов проверить по `api/*.proto` (если grpc-services по-другому назван — адаптировать matcher).

- [ ] **Step 1: Verify gRPC service names через proto**

Run:
```
grep -rn "service " api/*.proto api/*/*.proto 2>&1 | grep -E "service [A-Z]" | head -20
```

Записать имена обнаруженных сервисов клиентского gateway'а (что-то вроде `MessageService`, `SmsService`, `ClientService`) и admin'а (`AdminService` etc.).

**Если services разделены по разным backends** (client-gateway exposed `MessageService`, admin-gateway exposed `AdminService`) — matcher по имени работает. **Если оба gateway'а exposed похожие services** — разделять по path не получится, нужны два subdomain'а (api-grpc и admin-grpc).

Зафиксировать findings в commit message Step 4.

- [ ] **Step 2: Добавить gRPC host в Caddyfile**

Variant A (если services различны по имени, e.g. `sms.v1.MessageService` vs `admin.v1.AdminService`):

В `deployments/configs/caddy/Caddyfile` после admin host'а добавить:

```
# gRPC fan-out: один TLS-host на оба gateway'а через path-matcher.
# Caddy term'ит TLS, проксирует h2c на HaProxy mode tcp.
# Если service names в proto другие — обновить matcher path.
{$GRPC_DOMAIN} {
	tls {$LETSENCRYPT_EMAIL}
	# acme_ca https://acme-staging-v02.api.letsencrypt.org/directory

	@admin path /admin.* /AdminService/*
	reverse_proxy @admin haproxy:9091 {
		transport http {
			versions h2c
		}
	}

	# Default — client gRPC. Если path не matched как admin — идёт в client.
	reverse_proxy haproxy:9090 {
		transport http {
			versions h2c
		}
	}

	import security_headers
	log {
		output stdout
		format json
	}
}
```

Variant B (если services не различимы по path) — заменить блок на два host'а:

```
{$GRPC_DOMAIN} {
	tls {$LETSENCRYPT_EMAIL}
	reverse_proxy haproxy:9090 {
		transport http { versions h2c }
	}
	import security_headers
	log { output stdout; format json }
}

# Отдельный admin-grpc — добавить ADMIN_GRPC_DOMAIN в .env.prod.example.
{$ADMIN_GRPC_DOMAIN} {
	tls {$LETSENCRYPT_EMAIL}
	reverse_proxy haproxy:9091 {
		transport http { versions h2c }
	}
	import security_headers
	log { output stdout; format json }
}
```

При Variant B также добавить `ADMIN_GRPC_DOMAIN=admin-grpc.example.com` в `.env.prod.example`.

Implementer выбирает variant в зависимости от Step 1 finding'а — записать в commit message.

- [ ] **Step 3: Validate Caddyfile**

Run на сервере:
```
docker run --rm -v $(pwd)/deployments/configs/caddy/Caddyfile:/etc/caddy/Caddyfile:ro caddy:2.8-alpine caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
```

Expected: `Valid configuration`.

- [ ] **Step 4: Commit Task 3**

```
git status
git add deployments/configs/caddy/Caddyfile
# если Variant B — добавить .env.prod.example
git commit -m "feat(caddy): h2c reverse-proxy для gRPC client/admin gateway (Plan 8 Task 3)

Caddy term'ит TLS на grpc.\$PORTAL_DOMAIN и проксирует h2c на HaProxy
mode tcp. [Variant A path-based fan-out OR Variant B two subdomains —
implementer записал свой выбор + service-names finding].

Закрывает gRPC TLS gap для enterprise SMS-клиентов."
```

---

### Task 4: docker-compose.prod.yml — убрать HaProxy ports наружу

**Files:**
- Modify: `deployments/docker-compose.prod.yml:42-43`

**Rationale:** После Tasks 2-3 Caddy term'ит TLS, HaProxy получает трафик через docker network (`haproxy:8080`, `haproxy:8081`, etc.). Нет смысла оставлять HaProxy ports published — это shadow access bypass'ящий TLS. Stats:8404 — internal-only через SSH-tunnel.

- [ ] **Step 1: Override haproxy ports в prod overlay**

В `deployments/docker-compose.prod.yml` после строки 39 (`# === Gateways (HTTP/gRPC internals — доступ через haproxy) ===`) добавить блок для haproxy. Заменить **полностью** комментарии 40-50 на:

```yaml
  # haproxy: TLS-фронт = Caddy (Plan 8 Tasks 2-3). HaProxy теперь internal-only,
  # принимает трафик от caddy через docker network. Stats:8404 доступен через
  # SSH-tunnel: ssh -L 8404:haproxy:8404 prod-host.
  haproxy:
    ports: !override []
    depends_on:
      - client-gateway-1
      - client-gateway-2
      - admin-gateway-1
      - admin-gateway-2
      - portal-gateway
      - portal-frontend

  # smpp-gateway: оставляем published ТОЛЬКО 2775 (SMPP TCP); base также
  # публикует 2110 (Prometheus metrics) — он не должен торчать наружу,
  # Prometheus scrape'ит через docker network. !override заменяет весь
  # список ports на one-element массив.
  smpp-gateway:
    ports: !override
      - "2775:2775"
```

- [ ] **Step 2: Update Caddy depends_on**

В том же файле найти `caddy:` блок (строки 13-32) и заменить `depends_on:` секцию (строка 29-30):

```yaml
    depends_on:
      - portal-gateway
      - haproxy
```

Caddy теперь зависит от haproxy (для api/admin/grpc upstream'ов).

- [ ] **Step 3: Validate compose merged config**

Run на сервере:
```
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod config 2>&1 | grep -A20 "haproxy:" | head -30
```

Expected: в merged config'е у haproxy `ports: []` (пусто), depends_on содержит все 6 services.

```
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod config 2>&1 | grep -E "8080:8080|8081:8081|8082:8082|8083:8083|9090:9090|19095:9091|8404:8404"
```

Expected: пустой вывод (все эти port-mappings убраны в prod).

Если grep видит mapping — overlay не применился, проверить `!override` синтаксис.

- [ ] **Step 4: Commit Task 4**

```
git status
git add deployments/docker-compose.prod.yml
git commit -m "feat(compose): убрать HaProxy ports из prod overlay (Plan 8 Task 4)

После Caddy TLS expansion (Tasks 2-3) HaProxy принимает трафик только
через docker network. ports !override [] закрывает shadow-access на
8080/8081/8082/8083/9090/19095/8404. Stats:8404 — через SSH-tunnel."
```

---

### Task 5: rollback-prod.sh — embedded offline DB restore

**Files:**
- Modify: `scripts/rollback-prod.sh:20-47`

**Rationale:** Сейчас при rollback с DB restore скрипт печатает manual procedure и в non-interactive режиме skip'ает. После реального prod-incident'а оператор не должен copy-paste'ить 5 команд из stdout под стрессом. Алгоритм уже отлажен в `restore-test.sh:30-37` — переиспользовать.

- [ ] **Step 1: Заменить manual-print блок на embedded restore**

В `scripts/rollback-prod.sh` заменить блок строк 20-47 (от `if [ -n "$SNAPSHOT_DIR" ]` до закрытия `fi` перед `=== Redeploy`) на:

```bash
if [ -n "$SNAPSHOT_DIR" ] && [ -f "$SNAPSHOT_DIR/base.tar.gz" ]; then
    echo ""
    echo "=== DB Restore from $SNAPSHOT_DIR ==="
    echo "DESTRUCTIVE — postgres data будет полностью заменён snapshot'ом."

    if [ -t 0 ]; then
        echo "Continue? (yes/N)"
        read -r answer
        if [ "$answer" != "yes" ]; then
            echo "Aborted by operator."
            exit 1
        fi
    else
        echo "(non-interactive mode: proceeding with restore — caller responsibility)"
    fi

    # Resolve postgres data volume name. Compose project name = directory
    # parent имя (deployments), volume = "<project>_postgres-data".
    VOL=$(docker volume ls --format '{{.Name}}' | grep -E '_postgres-data$' | head -1)
    if [ -z "$VOL" ]; then
        echo "ERROR: postgres-data volume not found via 'docker volume ls'"
        exit 1
    fi
    echo "Using volume: $VOL"

    echo "Stopping postgres ..."
    $COMPOSE stop postgres

    echo "Restoring snapshot offline (alpine helper) ..."
    docker run --rm \
        -v "$VOL:/data" \
        -v "$SNAPSHOT_DIR:/backup:ro" \
        alpine sh -c '
            set -eu
            rm -rf /data/* /data/.[!.]* 2>/dev/null || true
            tar -xzf /backup/base.tar.gz -C /data
            chown -R 999:999 /data
        '

    echo "Starting postgres ..."
    $COMPOSE up -d postgres

    echo "Waiting 30s for postgres recovery + accept connections ..."
    sleep 30

    # Sanity check.
    if ! $COMPOSE exec -T postgres psql -U smpp -d smpp_db -c "SELECT 1" >/dev/null 2>&1; then
        echo "ERROR: postgres unreachable after restore — manual intervention required"
        exit 1
    fi
    echo "DB restore OK"
fi
```

Уровень изменений: блок 20-47 целиком заменён. Остальные части скрипта (header, redeploy после `fi`) не трогать.

- [ ] **Step 2: Verify скрипт синтаксически корректен**

Run на сервере:
```
bash -n scripts/rollback-prod.sh && echo OK
```

Expected: `OK`. Если syntax error — править.

- [ ] **Step 3: Dry-run unit-test через restore-test.sh harness**

`scripts/restore-test.sh` уже использует тот же алгоритм на изолированном postgres — фактически он покрывает Task 5. Запустить ещё раз для confidence (на сервере, текущий backup должен быть):

Run на сервере:
```
ls -lh /var/backups/sms-pg/daily-*.tar.gz | tail -3
sudo bash scripts/restore-test.sh 2>&1 | tail -20
```

Expected: `Restore test PASSED`.

Если PASSED — алгоритм рабочий, embed в rollback-prod.sh не сломан.

- [ ] **Step 4: Commit Task 5**

```
git status
git add scripts/rollback-prod.sh
git commit -m "feat(rollback): embed offline DB restore вместо manual procedure (Plan 8 Task 5)

После реального incident'а оператор не должен copy-paste'ить 5 команд под
стрессом. Алгоритм идентичен restore-test.sh (валидируется weekly)."
```

---

### Task 6: smoke-prod.sh — automated canary rejection check + api.* TLS check

**Files:**
- Modify: `scripts/smoke-prod.sh`
- Modify: `docs/runbooks/cutover-prod.md` (если существует — обновить step T+75)

**Rationale:** Текущий runbook говорит «вручную в T+75 зарегистрируй second client и попробуй send — expect 503». Под стрессом cutover-night оператор пропустит шаг или сделает неправильно. Автоматизировать.

**Подход:** Smoke регистрирует non-canary client (через admin API), пытается login, отправляет один SendMessage через client-gateway gRPC API (через api.* HTTP-bridge или прямо REST если есть), expect 503 Unavailable. После теста — удалить test-client'а (cleanup).

**Pre-flight verify:** Что admin API имеет endpoint регистрации client'а и client API имеет HTTP-endpoint для send (не только gRPC). Если HTTP-send нет — smoke использует grpcurl против `grpc.$PORTAL_DOMAIN`.

- [ ] **Step 1: Verify admin client-create endpoint и client send endpoint**

Run на сервере:
```
docker compose -f deployments/docker-compose.yml exec -T admin-gateway-1 wget -qO- http://localhost:8081/openapi.json 2>/dev/null | grep -E "client|register" | head -10
```

Если openapi.json нет — grep handler'ы:
```
grep -rn "POST.*client\|RegisterClient\|CreateClient" internal/api/ internal/gateway/admin/ 2>&1 | head -10
grep -rn "POST.*send\|/send\|/messages" internal/gateway/client/ 2>&1 | head -10
```

Записать concrete пути в commit message Step 4. Если client-gateway exposes только gRPC — установить `grpcurl` в smoke flow или skip canary check (с предупреждением в runbook'е что ручной шаг остаётся).

- [ ] **Step 2: Добавить Smoke 4 (api.* TLS reachable) и Smoke 5 (canary rejection)**

В `scripts/smoke-prod.sh` после строки 25 (`echo "OK"` после Smoke 3) и до финального `echo "AUTOMATED smoke OK"` добавить:

```bash
echo "=== Smoke 4: api/admin/grpc subdomain TLS reachable ==="
API_URL="https://${API_DOMAIN:?API_DOMAIN required for Smoke 4}"
ADMIN_URL="https://${ADMIN_DOMAIN:?ADMIN_DOMAIN required for Smoke 4}"

curl -fsS -o /dev/null --max-time 10 "$API_URL/health" || { echo "FAIL: $API_URL/health unreachable"; exit 1; }
curl -fsS -o /dev/null --max-time 10 "$ADMIN_URL/health" || { echo "FAIL: $ADMIN_URL/health unreachable"; exit 1; }
echo "OK"

echo "=== Smoke 5: canary rejection (non-allowlisted client → 503) ==="
# Skip если CANARY_CLIENT_IDS не задан в server env — canary off, проверка
# теряет смысл (любой client будет allowed → false positive если не 503).
CANARY_ON=$(curl -fsS --max-time 10 -H "Authorization: Bearer $TOKEN" "$ADMIN_URL/portal/v1/admin/system/canary-status" 2>/dev/null | jq -r '.enabled // false')
if [ "$CANARY_ON" != "true" ]; then
    echo "SKIP: canary mode disabled on server (CANARY_CLIENT_IDS unset)"
else
    # Регистрируем второго client'а — заведомо НЕ в allowlist'е.
    TEST_CLIENT_RESP=$(curl -fsS --max-time 10 -X POST "$ADMIN_URL/portal/v1/admin/clients" \
        -H "Authorization: Bearer $TOKEN" \
        -H 'Content-Type: application/json' \
        -d '{"name":"smoke-canary-test","email":"smoke-canary@test.local","password":"SmokeTest1234567!"}')
    TEST_CLIENT_ID=$(echo "$TEST_CLIENT_RESP" | jq -r '.id')
    test -n "$TEST_CLIENT_ID" || { echo "FAIL: client-create returned no id: $TEST_CLIENT_RESP"; exit 1; }

    # Cleanup при exit (success или fail).
    trap 'curl -fsS --max-time 10 -X DELETE -H "Authorization: Bearer $TOKEN" "$ADMIN_URL/portal/v1/admin/clients/$TEST_CLIENT_ID" >/dev/null 2>&1 || true' EXIT

    # Login как test-client.
    CLIENT_TOKEN=$(curl -fsS --max-time 10 -X POST "$API_URL/v1/auth/login" \
        -H 'Content-Type: application/json' \
        -d '{"email":"smoke-canary@test.local","password":"SmokeTest1234567!"}' \
        | jq -r '.token')
    test -n "$CLIENT_TOKEN" || { echo "FAIL: client login no token"; exit 1; }

    # Попробовать send — expect 503 Unavailable.
    HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 -X POST "$API_URL/v1/messages" \
        -H "Authorization: Bearer $CLIENT_TOKEN" \
        -H 'Content-Type: application/json' \
        -d '{"to":"+79990000000","text":"smoke","sender":"test"}')
    if [ "$HTTP_CODE" != "503" ]; then
        echo "FAIL: expected 503 (canary reject), got $HTTP_CODE"
        exit 1
    fi
    echo "OK (canary correctly rejected non-allowlisted client)"
fi
```

**Замечания implementer'у:**
- `/portal/v1/admin/system/canary-status` — **новый endpoint**, который, возможно, придётся добавить в admin-gateway (см. Task 7). Если решили НЕ добавлять — Smoke 5 проверяет только успех 503 безусловно (требует чтобы CANARY_CLIENT_IDS на сервере был задан).
- Пути `/portal/v1/admin/clients` (POST/DELETE) и `/v1/auth/login`, `/v1/messages` — **подтвердить через Step 1**. Если не совпадают — заменить на реальные.
- Если client API exposes только gRPC — заменить `curl POST /v1/messages` на `grpcurl -H "authorization: Bearer $CLIENT_TOKEN" grpc.$PORTAL_DOMAIN:443 <PackageName.Service>/Send <args>` и expect exit code 14 (Unavailable).

- [ ] **Step 3: Verify smoke бежит против sandbox без crash'а**

Sandbox не имеет TLS subdomain'ов и canary mode выключен — Smoke 4/5 ожидаемо SKIP'ятся / fail'ят. Цель Step 3 — проверить bash-syntax не сломан.

Run:
```
bash -n scripts/smoke-prod.sh && echo OK
```

Expected: `OK`.

Полный smoke в Plan 8 не запускается (нет prod env). На cutover-day будет запущен впервые.

- [ ] **Step 4: Update runbook (если файл существует)**

Run:
```
test -f docs/runbooks/cutover-prod.md && grep -n "T+75\|canary" docs/runbooks/cutover-prod.md | head -5
```

Если совпадения есть — найти T+75 step с manual canary check и заменить на: «Запустить `./scripts/smoke-prod.sh` — Smoke 5 проверяет canary rejection автоматически».

- [ ] **Step 5: Commit Task 6**

```
git status
git add scripts/smoke-prod.sh
# если runbook обновлён — git add docs/runbooks/cutover-prod.md
git commit -m "feat(smoke): automate canary rejection check + api/admin TLS verify (Plan 8 Task 6)

Smoke 4 проверяет TLS на api.\$PORTAL_DOMAIN и admin.\$PORTAL_DOMAIN.
Smoke 5 регистрирует non-canary test-client и проверяет 503 на send.
Cutover-day: ручной T+75 шаг убран из runbook'а."
```

---

### Task 7: deploy-prod.sh — image pre-pull + canary-status admin endpoint (если необходим)

**Files:**
- Modify: `scripts/deploy-prod.sh:60-73`
- (Conditional) Create: `internal/api/admin/canary_status.go` + регистрация в admin-gateway router'е.

**Rationale:** Сейчас `up -d --build` тянет base-image'ы и собирает services одновременно — на slow link это первый minute downtime. Pre-pull base images отдельно даёт оператору видимость прогресса и предотвращает timeout если build тормозит на pull.

**Canary-status endpoint:** Если в Task 6 Step 2 решили использовать `/portal/v1/admin/system/canary-status` — добавить здесь. Если использовать строгий «требуем CANARY_CLIENT_IDS != пусто» — пропустить под-задачу.

- [ ] **Step 1: Add image pre-pull в deploy-prod.sh**

В `scripts/deploy-prod.sh` найти секцию `=== Deploy ===` (строка 64-73). Перед `if [ "${1:-}" = "--service" ]` (строка 67) добавить:

```bash
# Pre-pull base images: caddy, postgres, redis, kafka, alpine helpers.
# Цель — отделить slow-pull от build phase, чтобы фокус оператора был
# на одном этапе. --include-deps подтягивает все зависимости services.
echo ""
echo "=== Pre-pull base images ==="
$COMPOSE pull --include-deps --quiet 2>&1 | tail -5 || {
    echo "WARN: pre-pull failed — continuing with build (it will retry pulls)"
}
```

- [ ] **Step 2: (Conditional) Add canary-status endpoint**

**ТОЛЬКО ЕСЛИ** Task 6 Step 2 использовал `/portal/v1/admin/system/canary-status`. Иначе skip Step 2.

Найти где в admin-gateway регистрируются handler'ы:
```
grep -rn "system\|/admin/" internal/gateway/admin/ internal/api/admin/ 2>&1 | grep -E "Handle|Route|/admin/" | head -10
```

Создать handler `internal/api/admin/canary_status.go`:

```go
package admin

import (
    "encoding/json"
    "net/http"
    "os"

    "github.com/smpp-server/smpp-server/internal/gateway/canary"
)

// CanaryStatusHandler возвращает {"enabled": true|false} в зависимости от
// CANARY_CLIENT_IDS env. Используется smoke-prod.sh для skip-логики
// canary-rejection check'а.
func CanaryStatusHandler(w http.ResponseWriter, r *http.Request) {
    enabled := os.Getenv(canary.EnvVar) != ""
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]bool{"enabled": enabled})
}
```

Зарегистрировать роут в admin-gateway routing setup (точное место — в файле, найденном через grep выше). Пример строки:

```go
adminRouter.HandleFunc("/portal/v1/admin/system/canary-status", admin.CanaryStatusHandler).Methods("GET")
```

Опубликовать через middleware которое уже требует admin auth (адаптировать по существующему pattern — каждый /portal/v1/admin/* endpoint проходит через тот же auth middleware).

- [ ] **Step 3: Verify build не сломан**

Run на сервере (если Step 2 был выполнен):
```
docker compose -f deployments/docker-compose.yml exec -T dev go build -buildvcs=false ./cmd/admin-gateway/...
docker compose -f deployments/docker-compose.yml exec -T dev go vet -buildvcs=false ./internal/api/admin/...
```

Expected: exit 0.

Если Step 2 пропущен — skip.

- [ ] **Step 4: Verify deploy-prod.sh syntax**

Run:
```
bash -n scripts/deploy-prod.sh && echo OK
```

Expected: `OK`.

- [ ] **Step 5: Commit Task 7**

```
git status
git add scripts/deploy-prod.sh
# если Step 2 — git add internal/api/admin/canary_status.go и router-файл
git commit -m "feat(deploy): image pre-pull + (optional) canary-status endpoint (Plan 8 Task 7)

Pre-pull отделяет slow image-tug от build phase. [Если canary-status
endpoint добавлен — описать.]"
```

---

### Task 8: End-to-end validation на sandbox

**Files:**
- Modify: `deployments/.env.prod.example` (verify по результатам)
- Создать: `docs/superpowers/plans/2026-05-06-aggregator-routing-plan-8-validation.md` (заметки validation runa, опционально)

**Rationale:** Plan 8 — preparation, реальный prod-VM нет. Цель Task 8 — прогнать prod-overlay стек на sandbox с подменой DNS (через `/etc/hosts` или sandbox-domain'ом) и убедиться что Caddy поднимается, гейты proxied корректно, smoke-prod.sh проходит Smoke 1-5.

**Подход:** sandbox-server (72.56.232.202) уже хост sandbox stack'а. Сделать второй compose project (например `sms-prod-rehearsal/` clone) либо `down` существующий и поднять с overlay. Жертва: sandbox недоступен на время rehearsal'а; перевешивает: единственный способ выловить bugs до cutover'а.

**Альтернатива:** docker compose локально на dev-машине, без LE (toggle staging). Caddy LE staging cert не trusted, но Smoke 1 будет работать с `--insecure`/curl `-k`.

- [ ] **Step 1: Pre-flight DNS substitute**

На sandbox-server:
- Решить: использовать staging-domain (sandbox.example.com и subdomain'ы с реальным DNS) или `/etc/hosts` mock с self-signed cert.
- Для скорости — рекомендую staging cert через LE staging endpoint (раскомментировать `acme_ca https://acme-staging-v02...` в Caddyfile).

Выбор зафиксировать в commit message.

- [ ] **Step 2: Заполнить sandbox `.env.prod-rehearsal`**

Скопировать `.env.prod.example` → `.env.prod-rehearsal`, заполнить:

```
PORTAL_DOMAIN=portal-rehearsal.<your-test-domain>
API_DOMAIN=api-rehearsal.<your-test-domain>
ADMIN_DOMAIN=admin-rehearsal.<your-test-domain>
GRPC_DOMAIN=grpc-rehearsal.<your-test-domain>
LETSENCRYPT_EMAIL=<test-email>
POSTGRES_PASSWORD=<openssl rand -base64 32>
REDIS_PASSWORD=<openssl rand -base64 32>
JWT_SECRET=<openssl rand -base64 64>
GRAFANA_ADMIN_PASSWORD=<openssl rand -base64 24>
ALERTMANAGER_*=<test creds или dev-null SMTP>
LOG_LEVEL=info
PG_BACKUP_RETENTION_DAYS=7
CANARY_CLIENT_IDS=<UUID одного test-client'а — для Smoke 5>
PROMETHEUS_ENV=rehearsal
```

`chmod 600 deployments/.env.prod-rehearsal`.

DNS A-records создать заранее (Cloudflare/Route53/etc.) — без них Caddy ACME-loop'ит.

- [ ] **Step 3: Bring up rehearsal stack**

На sandbox-server (`/opt/sms`):

```
git fetch origin && git checkout master  # после merge'а Plan 8
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod-rehearsal up -d
sleep 60  # caddy LE issue + services warm-up
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod-rehearsal ps | head -40
```

Expected: всех services `Up`. Если caddy `Restarting` — `docker compose ... logs caddy | tail -30` проверить ACME error (DNS не резолвится / port 80 занят).

- [ ] **Step 4: Run smoke-prod.sh**

```
# одноразовый run on rehearsal
PORTAL_DOMAIN=portal-rehearsal.<test-domain> \
API_DOMAIN=api-rehearsal.<test-domain> \
ADMIN_DOMAIN=admin-rehearsal.<test-domain> \
ADMIN_EMAIL=admin@example.com \
ADMIN_PASSWORD=<rehearsal admin pwd> \
bash scripts/smoke-prod.sh
```

Expected: все 5 smoke checks PASS. Smoke 5 (canary) — REQUIRES `CANARY_CLIENT_IDS` set + admin endpoint canary-status (если Task 7 Step 2 был сделан).

Если что-то fail'ит — root cause:
- Smoke 1 fail = LE staging cert + curl без `-k`. Решение: `curl -k` или dropdown к LE prod cert.
- Smoke 2 fail = login endpoint URL изменился, либо seed-admin не запущен с rehearsal pwd. Run `cmd/seed-admin` с `ADMIN_PASSWORD=<...>`.
- Smoke 4 fail = subdomain DNS не резолвится / Caddy не оформил cert.
- Smoke 5 fail = expected 503, получили 200 = canary не активен (CANARY_CLIENT_IDS unset?) или client gateway не получает env (verify через `docker compose ... exec client-gateway-1 env | grep CANARY`).

- [ ] **Step 5: Tear down rehearsal**

```
docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml --env-file deployments/.env.prod-rehearsal down
# поднять обратно sandbox stack без overlay
docker compose -f deployments/docker-compose.yml up -d
```

Sandbox возвращён в pre-rehearsal состояние.

- [ ] **Step 6: Commit findings (если были fix'ы по результатам rehearsal'а)**

Если rehearsal обнаружил проблемы — fix-commit'ы делать в рамках Task 1-7 retroactive (не как Task 8 commit). Task 8 сам по себе не commit'ит код — это validation.

Если всё прошло без fix'ов — никакого commit'а.

---

## Self-review

**Spec coverage (брифинг от пользователя):**
- A2 (HaProxy TLS) — Task 2 (HTTP) + Task 3 (gRPC) + Task 4 (close ports). ✓
- A3 (DB-restore embed) — Task 5. ✓
- B1 (seed-admin fail-fast) — Task 1. ✓ (UI часть — backlog, см. Out of scope.)
- B2 (smoke canary automation) — Task 6. ✓
- B3 (image pre-pull) — Task 7 Step 1. ✓
- F1 (sandbox cleanup test partitions) — **NOT in plan**. Включить Task 9?

**Решение по F1:** F1 = 5-минутный verify на sandbox через `\d audit_log_y2030*`. Drop команды можно сделать вручную если найдены leftover'ы. Не пишу как отдельный Task — добавляю как замечание в Task 8 Step 5 (после tear-down rehearsal'а оператор verify'ит partition cleanliness).

**Type/name consistency:**
- `EnvVar = "CANARY_CLIENT_IDS"` (canary.go) использован в Task 6 Step 2 и Task 7 Step 2. ✓
- `validateAdminPassword` (Task 1) — единственное имя по всему плану. ✓
- `CanaryStatusHandler` (Task 7) — единственное имя. ✓
- Caddy host blocks — `{$PORTAL_DOMAIN}` (existing), `{$API_DOMAIN}`, `{$ADMIN_DOMAIN}`, `{$GRPC_DOMAIN}` — единая конвенция. ✓

**Placeholder scan:** все steps содержат конкретный код или конкретные команды. Conditional Task 7 Step 2 явно зависит от Task 6 Step 2 решения и помечен «Skip if ...». ✓

**Готовность исполнять:**
- Все file paths verified против live codebase (Read'ы выше).
- Все скрипт-секции (rollback restore, smoke checks) согласованы с существующим кодом (`restore-test.sh`, `canary.go`).
- Variant A/B в Task 3 явно зависит от proto-finding'а в Step 1 — implementer выбирает на основе данных.

---

## Дополнения по execution

**Review-gate:** После каждого Task 1-7 — non-negotiable subagent review (superpowers:code-reviewer). Inline fix допустим только для тривиальных 1-2 line edit'ов; всё остальное — fix-iteration commit. Task 8 = validation, не code-review (запускается на sandbox).

**Branching:** Все commits на master. Каждый Task — один commit (либо несколько если review-fix). Никаких force-push, никаких amend.

**Server testing:** Tests запускаются через dev-container на sms-server: `docker compose -f deployments/docker-compose.yml exec -T -e TEST_DATABASE_URL=postgres://smpp:smpp_password@postgres:5432/smpp_db dev go test -buildvcs=false ./...`. Локальный Windows — Device Guard блокирует Go-toolchain.

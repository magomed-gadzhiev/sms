# Handoff: PR #37 (perf-batch) — завершение деплоя на sandbox

**Статус:** код в master (`9e40ac9`), миграции применены до 123, **образы НЕ пересобраны → код не задеплоен в контейнерах**. Деплой прерван по решению пользователя из-за SSH-нестабильности sandbox-сервера.

**Назначение этого документа:** инструкции для следующего AI-агента (или человека), который будет завершать деплой с другого компьютера. Контекст PR-обсуждения недоступен — здесь самодостаточная памятка.

---

## Что уже сделано

1. ✅ `master` обновлён до `9e40ac9` (Merge PR #37 «perf+fix: hard-cache hot-path + load-test knobs + sandbox-seed guard»).
2. ✅ Миграции прогнаны: `schema_migrations.version = 123`, `dirty = false`.
3. ✅ Sandbox test-users существуют: `aggregator@test.local`, `subacc@test.local` — пароль `Admin123!`.
4. ✅ Code review пройден локально через `superpowers:code-reviewer`, BLOCKER-фиксы применены: B1 (production-guard для миграций 116/118), B2 (Kafka async форсит sync для `sms.outgoing`), H1 (auth cache-hit проверяет Active/Expired), M1 (persist temp-table reuse под env-флагом).

## Что НЕ сделано

1. ❌ **Docker-образы не пересобраны.** `docker images deployments-* --format '{{.CreatedAt}}'` показывает `2026-04-23` для всех backend-сервисов. Build падал дважды на TLS-таймаутах docker hub (`alpine:3.19`, `node:20-alpine`).
2. ❌ **Login не протестирован.** Endpoint `POST http://72.56.232.202:18084/portal/v1/auth/login` отвечает `INTERNAL_ERROR: failed to create session` из-за отдельной инфра-проблемы (см. ниже).

## Срочный security-инцидент (отдельно от PR)

`docker exec redis redis-cli INFO replication` на sandbox показывает:

```
role:slave
master_host:175.24.232.83
master_port:26869
master_link_status:down
```

`175.24.232.83` — внешний IP в Tencent Cloud (CN). Это классическая атака на unauth Redis (REPLICAOF от внешнего хоста, чтобы превратить целевой Redis в slave и наблюдать/инжектить данные через master push). Redis в read-only-replica-режиме блокирует все writes, поэтому auth-service не может создать сессию.

**Минимальный фикс для разблокировки login:**
```bash
ssh sms-server "docker exec redis redis-cli REPLICAOF NO ONE"
```

**Долгосрочно (вне scope этого PR, требует отдельного решения):**
- Закрыть Redis-порт 6379 от внешнего интерфейса (firewall / docker bind на 127.0.0.1 / приватная сеть).
- Включить `requirepass` или ACL.
- Аудитнуть логи: `docker logs redis | grep -E 'REPLICAOF|MASTERAUTH'` — найти когда первый `REPLICAOF 175.24.232.83` пришёл, и откуда.
- Аудитнуть содержимое БД на признаки tampering (особенно api-key hashes и session-store).

## Шаги для завершения деплоя

Выполняй с компьютера, где есть SSH-доступ к `sms-server` (alias из `~/.ssh/config`, реальный IP `72.56.232.202`).

### 1. Остановить Redis-replication атаку

```bash
ssh sms-server "docker exec redis redis-cli REPLICAOF NO ONE && docker exec redis redis-cli INFO replication | grep ^role"
# Ожидается: role:master
```

### 2. Pre-pull базовых образов (обходит TLS-таймаут docker hub во время build)

```bash
ssh sms-server 'for img in alpine:3.19 golang:1.25-alpine node:20-alpine nginx:alpine; do
  for i in 1 2 3 4 5; do
    docker pull "$img" && break
    echo "retry $i for $img"; sleep 15
  done
done'
```

### 3. Detached rebuild (SSH-disconnect-safe)

SSH к sms-server разрывается на длинных операциях (~10 мин и больше), поэтому build надо запустить отвязанным от ssh-сессии через `setsid`:

```bash
ssh sms-server 'cat > /tmp/sms_rebuild_final.sh <<"SCRIPT"
#!/bin/bash
cd /opt/sms/deployments
exec > /tmp/sms_rebuild_final.log 2>&1
echo "=== START $(date) ==="
SVC="provider-service routing-service template-service client-service auth-service contact-service campaign-service billing-service tarification-service cascade-service messaging-service webhook-service analytics-service audit-service portal-gateway admin-gateway-1 admin-gateway-2 worker link-service network-analytics-service client-gateway-1 client-gateway-2 smpp-gateway pipeline-sender pipeline-status dlr-delivery pipeline-router pipeline-persist"
docker compose build $SVC
echo "=== BUILD EXIT $? $(date) ==="
docker compose up -d --no-deps $SVC
echo "=== UP EXIT $? $(date) ==="
echo done > /tmp/sms_rebuild_final.done
SCRIPT
chmod +x /tmp/sms_rebuild_final.sh
setsid bash /tmp/sms_rebuild_final.sh < /dev/null > /dev/null 2>&1 &
sleep 2 && pgrep -af /tmp/sms_rebuild_final.sh'
```

### 4. Подождать 7-10 минут и проверить

```bash
ssh sms-server 'ls /tmp/sms_rebuild_final.done 2>&1; grep -E "BUILD EXIT|UP EXIT" /tmp/sms_rebuild_final.log'
# Ожидается: /tmp/sms_rebuild_final.done существует, BUILD EXIT 0, UP EXIT 0
```

Если `BUILD EXIT 1` — смотреть `grep -E "ERROR|error:|TLS handshake" /tmp/sms_rebuild_final.log` и обходить конкретный fail.

### 5. Проверить свежесть образов

```bash
ssh sms-server "docker images --filter reference=deployments-auth-service --format '{{.CreatedAt}}'"
# Ожидается: дата близкая к сегодняшней, не 2026-04-23
```

### 6. Поднять весь стек (auto-restart всё что depends_on свежие образы)

```bash
ssh sms-server "cd /opt/sms/deployments && docker compose up -d 2>&1 | tail -20"
```

### 7. Smoke-test логина

```bash
curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"email":"aggregator@test.local","password":"Admin123!"}' \
  http://72.56.232.202:18084/portal/v1/auth/login | head -c 500
```

**Ожидается:** JSON с `access_token` или (если 2FA включено) `requires_2fa: true` + `login_ticket`.
**Если `failed to create session`:** Redis всё ещё replica — повторить шаг 1.

### 8. Проверить, что hard-cache работает (опционально, по желанию)

```bash
ssh sms-server "docker exec auth-service env | grep HARD_CACHE"
# Если HARD_CACHE_ENABLED не выставлен — кеши неактивны (default safe mode).
# Активация только для load-теста — выставить в docker-compose.override.yml:
#   HARD_CACHE_ENABLED=true
#   HARD_CACHE_TTL_SECONDS=60
# и пересоздать сервисы.
```

### 9. Проверить, что persist stage работает на default-режиме (DROP+CREATE)

```bash
ssh sms-server "docker logs deployments-pipeline-persist-1 --tail 20 2>&1 | grep -iE 'temp_table|create temp|drop table' | head -5"
```
Ошибок «column X does not exist» быть не должно — это означало бы что `PERSIST_TEMP_TABLE_REUSE=true` пробрался куда не надо.

## Важные ENV-флаги, прибавленные в этом PR (default off — safe)

Все три флага читаются на старте процесса. Без них — поведение идентично master до PR (за исключением миграций).

| Флаг | Default | Что включает | Где безопасно включать |
|---|---|---|---|
| `HARD_CACHE_ENABLED=true` | off | Process-local TTL cache: api-key→user, router sender, repos | Только под load-тест |
| `HARD_CACHE_TTL_SECONDS=N` | per-cache | Override для всех hard-cache TTL | Опционально |
| `KAFKA_FAST_PRODUCER=true` | off | WaitForLocal+Idempotent=false, batching | Только load-тест |
| `KAFKA_ASYNC_PUBLISH=true` | off | Async для inter-stage (sms.routed/dlr/failed); sms.outgoing форсит sync | Только load-тест |
| `PERSIST_TEMP_TABLE_REUSE=true` | off | CREATE TEMP TABLE IF NOT EXISTS вместо DROP+CREATE | Только load-тест и **только если не делается ALTER TABLE messages между рестартами persist-pod'ов** |

## Production-guard для миграций 116/118

Миграции 000116/000118 (sandbox-seed users + password reset) обёрнуты в `DO $$` блок с проверкой `current_setting('app.environment', true)`. На прод-БД нужен one-time setup:

```sql
ALTER DATABASE smpp_db SET app.environment = 'production';
```

Sandbox/dev/CI без выставленной GUC — миграции выполняются (default-permissive). На прод они тогда self-skip с `RAISE NOTICE`.

## Followup-PR'ы (из ревью, в backlog)

В описании PR #37 перечислены: M2 negative-cache TTL для operator/template (300s слишком много), M3 routerSenderCache key escape (`|` коллизия), M4 cache DI (env читается на старте), L2 ProviderRepository.GetByID без negative-cache, L4 AsyncProducer error-горутина без graceful note, L5 unit-тесты на async-путь.

## Контекст для следующего AI-агента

- Sandbox-сервер `sms-server` (`72.56.232.202`) — dev-окружение, можно деплоить/мигрировать свободно.
- SSH разрывается на длинных операциях (>5 мин обычно). Используй `setsid` + файл-маркеры для асинхронных задач.
- Pre-commit hook + GitHub Actions CI требуют `./scripts/check.sh` зелёным. Бейзлайн ESLint warnings — 69, текущее значение — 56. Не повышай.
- Любая модификация кода — через `/execute-with-review` (см. `skills/execute-with-review.md`). Прямой merge feature-ветки в master без PR-ревью запрещён (см. CLAUDE.md, секция «Mandatory code review»).
- Документация (этот файл) — может коммититься напрямую в master, как docs-only изменение (см. «Исключения» в CLAUDE.md).

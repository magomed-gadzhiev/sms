# API Review v2 — Umbrella Phase 0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Собрать единый inventory поверхностей (HTTP, gRPC-external, SMPP, gRPC-internal) для ревью v2, перепроверить блокеры B1-B3 и легаси-статус `cmd/api` (R1). Пометить v1 спек как superseded.

**Architecture:** Чистая разведка без правки кода. Один артефакт `docs/reports/2026-04-21-api-review-v2-surface.md` с таблицами + findings-секцией. Ветка `review/api-v2-phase-0`, один PR к master.

**Tech Stack:** Go 1.24 read-only, grep, markdown.

**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`

---

## File Structure

Файлы, создаваемые или модифицируемые:

- `docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md` — **modify**: добавить в заголовок `**SUPERSEDED by 2026-04-21-api-review-v2-umbrella.md**`.
- `docs/reports/2026-04-21-api-review-v2-surface.md` — **create**: единый inventory + findings.

Кода не пишем.

---

## Known pre-state

Рабочий каталог содержит кучу untracked файлов (скриншоты, тестовые файлы из других сессий). Они НЕ удаляются, НЕ комитятся. Субагенты получают явный file-allowlist и обязаны игнорировать остальное.

---

## Task 1: Setup — ветка, SUPERSEDED marker, scaffold

**Files allowed to touch:**
- Create: `docs/reports/2026-04-21-api-review-v2-surface.md`
- Modify: `docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md` (добавить ОДНУ строку в заголовке)

**Files explicitly forbidden:** любые другие (включая untracked).

- [ ] **Step 1: Ветка**

Контроллер (не субагент):
```bash
git checkout master
git pull --ff-only
git checkout -b review/api-v2-phase-0
```

Expected: `Switched to a new branch 'review/api-v2-phase-0'`.

- [ ] **Step 2: SUPERSEDED marker на v1**

В `docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md` найти строку `**Статус:** утверждён в брейншторме, ждёт user-review перед переходом к плану`.

Заменить на:
```
**Статус:** SUPERSEDED by `2026-04-21-api-review-v2-umbrella.md` (2026-04-21). Оставлен как история решений, исполнять нельзя.
```

- [ ] **Step 3: Scaffold surface.md**

Create `docs/reports/2026-04-21-api-review-v2-surface.md`:

```markdown
# API Review v2 — Surface Inventory

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-review-v2-umbrella.md`
**Revision (master HEAD):** <ХЭШ> — заполнить в Task 8

## 1. HTTP Endpoints (cmd/client-gateway)

_TODO: Task 2_

## 2. gRPC External (cmd/client-gateway CLIENT_GRPC_PORT)

_TODO: Task 3_

## 3. SMPP (cmd/smpp-gateway)

### 3.1. PDU Commands

_TODO: Task 4_

### 3.2. SMPP TLVs

_TODO: Task 4_

### 3.3. smppv1 control-plane RPC

_TODO: Task 4_

## 4. gRPC Internal (top-5 hot-path services)

_TODO: Task 5_

## 5. Findings

### F1. R1 — cmd/api legacy status

_TODO: Task 6_

### F2. B1 — Integration test infrastructure

_TODO: Task 7_

### F3. B2 — AuthAdapter user_id=client_id kludge

_TODO: Task 7_

### F4. B3 — Dual-charge wiring in TarifyMessage

_TODO: Task 7_

## 6. Go/no-go для циклов 1/2/3

_TODO: Task 8_
```

- [ ] **Step 4: Commit**

```bash
git status --short
# expected: A docs/reports/... ; M docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md
git add docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md \
        docs/reports/2026-04-21-api-review-v2-surface.md
git commit -m "chore(api-review): phase 0 — scaffold surface inventory; mark v1 superseded

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: HTTP inventory

**Files allowed:** Modify only `docs/reports/2026-04-21-api-review-v2-surface.md`.

**Files read-only:** `internal/gateway/client/router/router.go`, `internal/gateway/client/handlers/*.go`.

- [ ] **Step 1: Извлечь все HandleFunc**

Прочитать `internal/gateway/client/router/router.go`. Выписать все вызовы `HandleFunc` с method+path в порядке регистрации.

- [ ] **Step 2: Для каждого — handler file:line и service-call**

Открыть handler-файл, найти определение функции (`file:line`). Внутри найти primary gRPC-клиентский вызов (например `h.messagingClient.SendMessage`). Response shape — тип Go или inline-map.

Health и docs endpoints включаются с пометкой `n/a (health)` / `n/a (docs)`.

- [ ] **Step 3: Заменить `_TODO: Task 2_` в секции 1**

Схема:
```
| Method | Path | Handler file:line | Service call | Response shape |
|---|---|---|---|---|
```

- [ ] **Step 4: Commit**

```bash
git status --short
git add docs/reports/2026-04-21-api-review-v2-surface.md
git commit -m "chore(api-review): phase 0 — HTTP inventory (client-gateway)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: gRPC external inventory

**Files allowed:** Modify only `docs/reports/2026-04-21-api-review-v2-surface.md`.

**Files read-only:** `internal/gateway/client/grpc/*.go`, `cmd/client-gateway/main.go`, `api/proto/*/` (только те что импортируются в client-gateway).

- [ ] **Step 1: Найти gRPC services в client-gateway**

Read `internal/gateway/client/grpc/` — все `*.go` файлы. Для каждого найти что он имплементирует:
- Искать `Unimplemented*Server` embedding или `func (s *XxxServer) RPCName(...)`.
- Сопоставить с proto-файлом в `api/proto/<service>v1/` — найти `service ... {}` и список `rpc`.

- [ ] **Step 2: Для каждого RPC**

- `service.RPC` — полное имя.
- Handler file:line в `internal/gateway/client/grpc/`.
- Какой внутренний gRPC клиент используется (например `h.messagingClient.SendMessage`) — аналогично Task 2.

Если proto содержит RPC, но серверной имплементации нет — строка с `not implemented`.

- [ ] **Step 3: Заменить `_TODO: Task 3_` в секции 2**

Схема:
```
| Service | RPC | Handler file:line | Downstream internal gRPC | Notes |
|---|---|---|---|---|
```

- [ ] **Step 4: Commit**

```bash
git status --short
git add docs/reports/2026-04-21-api-review-v2-surface.md
git commit -m "chore(api-review): phase 0 — gRPC external inventory

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: SMPP inventory (PDU + TLV + control-plane)

**Files allowed:** Modify only `docs/reports/2026-04-21-api-review-v2-surface.md`.

**Files read-only:** `internal/gateway/smpp/**/*.go`, `internal/smpp/protocol/constants.go`, `internal/smpp/protocol/pdu.go`, `cmd/smpp-gateway/main.go`, `api/proto/smppv1/`.

**NOT in scope:** `internal/smpp/server/` (legacy, per umbrella 1.2).

- [ ] **Step 1: PDU Commands**

В `internal/gateway/smpp/server/handler.go` и соседних файлах — найти switch/case по command_id. Для каждого PDU: handler file:line, service-call, notes (особенности, DLR-branching).

MO vs DLR — отдельные строки если код их различает, иначе одна с пометкой.

Заполнить секцию 3.1.

- [ ] **Step 2: TLV**

В `internal/smpp/protocol/constants.go` и `pdu.go` — найти TLV tag-константы. Для каждой:
- Hex value, symbolic name.
- Grep использования в `internal/gateway/smpp/` (NOT в `internal/smpp/server/` — legacy).
- Used in submit_sm / deliver_sm / ни там ни там.
- Mapped to field — Go-struct и поле куда маппится.

Если tag-констант нет совсем — так и записать: `ZERO TLV constants declared. Any TLV handling via magic hex literals.` + грепнуть magic literals типа `0x001E`, `0x0427` в `internal/gateway/smpp/` и `internal/smsc/`.

Заполнить секцию 3.2.

- [ ] **Step 3: smppv1 control-plane**

Прочитать `api/proto/smppv1/*.proto`. Найти service и RPC. Для каждого RPC — кто вызывает (в `cmd/smpp-gateway` или где-то ещё) и что делает.

Заполнить секцию 3.3.

- [ ] **Step 4: Commit**

```bash
git status --short
git add docs/reports/2026-04-21-api-review-v2-surface.md
git commit -m "chore(api-review): phase 0 — SMPP inventory (PDU, TLV, control-plane)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: gRPC internal top-5 inventory

**Files allowed:** Modify only `docs/reports/2026-04-21-api-review-v2-surface.md`.

**Files read-only:** `api/proto/messagingv1/`, `api/proto/billingv1/`, `api/proto/tarificationv1/`, `api/proto/cascadev1/`, `api/proto/webhookv1/`, и соответствующие `internal/services/<name>/grpc/` для server-side.

- [ ] **Step 1: Для каждого из 5 сервисов**

Услуги: `messagingv1`, `billingv1`, `tarificationv1`, `cascadev1`, `webhookv1`.

Для каждого:
1. Прочитать .proto — список RPC с input/output сообщениями.
2. Прочитать серверную имплементацию в `internal/services/<name>/grpc/` (или где лежит).
3. Для каждого RPC определить, как передаётся `client_id`:
   - **request field** (например `int64 client_id = 1;` в message) — записать имя поля.
   - **metadata key** (если вычитывается из `metadata.FromIncomingContext`) — записать ключ.
   - **не передаётся вообще** — записать `—`.

- [ ] **Step 2: Заполнить секцию 4**

Схема:
```
| Service | RPC | client_id transport | Caller(s) | Notes |
|---|---|---|---|---|
```

`client_id transport` — `request field: client_id` / `metadata: x-client-id` / `—` / `mixed`.

- [ ] **Step 3: Commit**

```bash
git status --short
git add docs/reports/2026-04-21-api-review-v2-surface.md
git commit -m "chore(api-review): phase 0 — gRPC internal top-5 inventory

messagingv1, billingv1, tarificationv1, cascadev1, webhookv1.
Колонка client_id transport показывает как tenant-контекст пробрасывается.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: R1 — cmd/api legacy status

**Files allowed:** Modify only `docs/reports/2026-04-21-api-review-v2-surface.md`.

**Files read-only:** `cmd/api/**`, `cmd/client-gateway/**`, `docker-compose*.yml`, любые deployment-манифесты в репо (`deploy/`, `k8s/`, `helm/` если есть), `scripts/server.sh`.

- [ ] **Step 1: Поиск признаков активности cmd/api**

1. В `docker-compose*.yml` — есть ли сервис, собирающий `cmd/api`? (обычно `dockerfile: cmd/api/Dockerfile` или `command: /app/api`).
2. То же для `cmd/client-gateway` — для сравнения.
3. Deployment манифесты: `ls deploy/ k8s/ helm/ 2>/dev/null` — если есть, прочитать имена и определить который из двух используется.
4. `scripts/server.sh` — какие сервисы стартует.
5. `git log --since=2026-03-01 --oneline -- cmd/api/` vs `cmd/client-gateway/` — какой активнее.

- [ ] **Step 2: Запись вывода в F1**

Форма:
```
### F1. R1 — cmd/api legacy status

**Deployment:** <список где упоминается cmd/api>.
**Сравнение с cmd/client-gateway:** <список где упоминается client-gateway>.
**Git activity:** commits за последние 2 месяца — cmd/api: N, cmd/client-gateway: M.
**Вывод:** <legacy подтверждён / legacy НЕ подтверждён — требуется пауза>.
```

Если вывод — «не подтверждён», четко указать что скоуп ревью должен быть расширен, и до пересмотра Cycle 1/2 не стартуют.

- [ ] **Step 3: Commit**

```bash
git status --short
git add docs/reports/2026-04-21-api-review-v2-surface.md
git commit -m "chore(api-review): phase 0 — R1 cmd/api legacy verification

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: B1/B2/B3 re-check

**Files allowed:** Modify only `docs/reports/2026-04-21-api-review-v2-surface.md`.

**Files read-only:** `.github/workflows/*`, `docker-compose*.yml`, `scripts/check.sh`, `internal/gateway/smpp/server/auth_adapter.go`, `internal/services/tarification/application/*.go`.

- [ ] **Step 1: B1 — integration CI infra**

1. `ls .github/workflows/` — файлы workflow.
2. Для каждого: есть ли `services:` с `postgres` или `redis`?
3. Есть ли job, запускающий `go test -tags=integration`?
4. В `scripts/check.sh` — поддержка integration mode?
5. Grep `//go:build integration` по `**/*.go` — сколько тестов на integration-теге сейчас.

Записать в F2.

**Решение:**
- Если postgres+redis есть в CI И `-tags=integration` запускается → B1 ЗАКРЫТ.
- Иначе → B1 ОТКРЫТ, Cycle 3 блокирован до закрытия (либо добавить инфру, либо downscope).

- [ ] **Step 2: B2 — AuthAdapter kludge**

1. Прочитать `internal/gateway/smpp/server/auth_adapter.go` полностью (файл небольшой).
2. Искать упоминания `client_id` vs `user_id`: совпадают ли, есть ли TODO/FIXME о временном коде, маппинг через БД или прямой cast.

Записать в F3.

**Решение:**
- Если костыль исчез (нет `user_id=client_id` или явный proper маппинг через client-service lookup) → B2 ЗАКРЫТ.
- Иначе → B2 ОТКРЫТ, SMPP C-visibility Cycle 3 блокирован.

- [ ] **Step 3: B3 — dual-charge wiring**

1. Grep `ChargeMessageDual` — все вызовы.
2. Прочитать `internal/services/tarification/application/tarification_service.go` (или аналогичное где определён `TarifyMessage`).
3. Найти тело `TarifyMessage` — какую операцию биллинга оно вызывает.
4. Прочитать `CommitCharge` (если существует) — вызывает ли `ChargeMessageDual`, за каким feature flag.

Записать в F4.

**Решение:**
- Если `TarifyMessage` вызывает `ChargeMessageDual` напрямую (без флага) → B3 ЗАКРЫТ.
- Если через `CommitCharge` с флагом, но флаг включён в конфиге по умолчанию → B3 ЧАСТИЧНО ЗАКРЫТ, Cycle 3 стартует с оговоркой про флаг.
- Если dual-charge не вызывается вовсе → B3 ОТКРЫТ.

- [ ] **Step 4: Commit**

```bash
git status --short
git add docs/reports/2026-04-21-api-review-v2-surface.md
git commit -m "chore(api-review): phase 0 — B1/B2/B3 re-check

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Финализация — HEAD hash, Go/no-go, PR

**Files allowed:** Modify `docs/reports/2026-04-21-api-review-v2-surface.md`.

- [ ] **Step 1: Заменить `<ХЭШ>` в заголовке**

```bash
git rev-parse HEAD
```

Взять вывод, заменить в surface.md строке `**Revision (master HEAD):** <ХЭШ>` на актуальный SHA.

- [ ] **Step 2: Заполнить секцию 6 — Go/no-go**

На основе F1-F4 записать решение по каждому циклу:

```
## 6. Go/no-go для циклов 1/2/3

### Cycle 1 (Contract: A1, A1-grpc, A2, A3)
<GO / NO-GO> — <обоснование>

### Cycle 2 (SMPP + tenant-glue: A6, A7)
<GO / NO-GO> — <обоснование>

### Cycle 3 (Subaccount e2e: C)
<GO / NO-GO> — <список блокеров из F2/F3/F4 если NO-GO>
```

Правила:
- Cycle 1: GO если R1 legacy подтверждён. Блокируется только R1.
- Cycle 2: GO если R1 подтверждён. Блокируется только R1.
- Cycle 3: GO только если R1 + B1 + B2 + B3 все закрыты (либо с явным downscope).

- [ ] **Step 3: Commit + push + PR**

```bash
git status --short
git add docs/reports/2026-04-21-api-review-v2-surface.md
git commit -m "docs(api-review): phase 0 — finalize revision hash and go/no-go

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"

git push -u origin review/api-v2-phase-0

gh pr create --title "API Review v2 Phase 0: umbrella surface inventory & blocker re-check" --body "$(cat <<'EOF'
## Summary

- Umbrella Phase 0 per spec `2026-04-21-api-review-v2-umbrella.md`.
- Single artifact: `docs/reports/2026-04-21-api-review-v2-surface.md` — HTTP + gRPC external + SMPP + gRPC internal (top-5) inventory.
- Findings F1-F4: R1 (cmd/api legacy status), B1-B3 (integration infra, auth_adapter, dual-charge wiring).
- V1 spec marked SUPERSEDED.
- Go/no-go per cycle captured.

## Test plan

- [ ] Inventory покрывает все HandleFunc в client-gateway/router/router.go.
- [ ] Top-5 gRPC services перечислены с колонкой client_id transport.
- [ ] F1 содержит явный вывод (legacy подтверждён / нет).
- [ ] F2/F3/F4 содержат явные решения (закрыт / открыт) с основанием.
- [ ] Go/no-go для Cycle 1/2/3 вытекает из findings.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-review

**Spec coverage:**
- Umbrella §1 (скоуп) — tasks охватывают все 4 транспорта ✅.
- Umbrella §2.5 (блокеры) — Task 7 ✅.
- Umbrella §3 (Phase 0 tasks 3.1-3.6) — tasks 2-7 ✅.
- Umbrella §6 (R1) — Task 6 ✅.
- Umbrella §1.7 (SUPERSEDED v1) — Task 1 Step 2 ✅.

**Placeholders:** _TODO_ только внутри каркаса артефакта (Task 1), заполняются последующими тасками.

**Type consistency:** схема HTTP-таблицы (5 cols) одинакова в Task 1 каркасе и Task 2 fill. gRPC-external схема — 5 cols, согласована. SMPP 3.1/3.2/3.3 — разные подсекции, у каждой своя схема, нет конфликта.

**Gap check:** не в скоупе — portal, admin, cmd/api, cmd/smpp-server, A4/A5, performance — в плане их нет ✅.

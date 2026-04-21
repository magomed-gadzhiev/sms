# API Review — Phase 0: Surface Inventory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Собрать карту всей поверхности внешнего API клиента (HTTP + SMPP), подготовить пустую матрицу покрытия и выяснить инфраструктурные блокеры (наличие OpenAPI, integration-тестов в CI, статус dual-charge Phase 2). Без этих данных дальнейшие фазы ревью писать наугад.

**Architecture:** Чистая разведка без правки кода. Результат — два документа в `docs/reports/`: inventory-таблица (endpoint/PDU → хендлер → service → БД) и matrix-скелет (все endpoint × role × axis комбинации в состоянии UNKNOWN). Плюс findings-раздел про обнаруженные блокеры.

**Tech Stack:** Go 1.24 read-only (читаем router.go, handler.go, proto), grep/markdown-таблицы, CSV для машиночитаемой матрицы.

**Spec:** `docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md`

**Branch:** `review/api-phase-0-inventory` (создать от master)

---

## File Structure

Файлы, создаваемые этим планом:

- `docs/reports/2026-04-21-api-surface-inventory.md` — единая inventory-таблица (HTTP + SMPP).
- `docs/reports/2026-04-21-api-coverage-matrix.md` — скелет матрицы (markdown).
- `docs/reports/2026-04-21-api-coverage-matrix.csv` — та же матрица в машиночитаемом виде.
- `docs/reports/2026-04-21-api-review-phase-0-findings.md` — отчёт о блокерах (OpenAPI, integration-инфра, dual-charge).

Кода в этой фазе не пишем. ε-критерий не применяется (нет code diff'а, всё — документы).

---

## Task 1: Создать ветку и scaffold отчётов

**Files:**
- Create: `docs/reports/2026-04-21-api-surface-inventory.md` (пустой каркас)
- Create: `docs/reports/2026-04-21-api-coverage-matrix.md` (пустой каркас)
- Create: `docs/reports/2026-04-21-api-review-phase-0-findings.md` (пустой каркас)

- [ ] **Step 1: Создать ветку от master**

Run:
```bash
git checkout master
git pull
git checkout -b review/api-phase-0-inventory
```

Expected: `Switched to a new branch 'review/api-phase-0-inventory'`

- [ ] **Step 2: Создать inventory-скелет**

Create `docs/reports/2026-04-21-api-surface-inventory.md`:

```markdown
# API Surface Inventory — 2026-04-21

**Дата снапшота:** 2026-04-21
**Источник:** `internal/gateway/client/router/router.go`, `internal/gateway/smpp/server/handler.go`, `internal/smpp/server/handler.go`, `internal/smpp/protocol/constants.go`
**Ревизия:** commit <ХЭШ> (заполнить в конце Task 7)

## HTTP Endpoints (client-gateway)

| Method | Path | Handler file:line | Service call | DB tables written | Response shape |
|---|---|---|---|---|---|
| _TODO: заполнить в Task 2_ | | | | | |

## SMPP Commands

| PDU / Event | Handler file:line | Service call | DB tables written | Notes |
|---|---|---|---|---|
| _TODO: заполнить в Task 3_ | | | | |

## SMPP Supported TLVs

| Tag (hex) | Symbolic name | Used in (submit/deliver) | Mapped to field |
|---|---|---|---|
| _TODO: заполнить в Task 4_ | | | |

## Auth entry points

| Transport | Method | Source of identity | File:line |
|---|---|---|---|
| _TODO: заполнить в Task 5_ | | | |
```

- [ ] **Step 3: Создать matrix-скелет**

Create `docs/reports/2026-04-21-api-coverage-matrix.md`:

```markdown
# API Coverage Matrix — 2026-04-21

**Snapshot date:** 2026-04-21
**Spec:** `docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md`
**Phase completed:** 0 (skeleton only — все строки в UNKNOWN)
**OK rows with test_file (ratchet baseline):** 0

## Legend

- `current_state`: `OK` / `DRIFT` / `MISSING` / `BROKEN` / `UNKNOWN`
- `severity`: `critical` (биллинг/auth) / `major` (контракт) / `minor` (косметика)
- `action`: `fixed-in-PR#<N>` / `plan:<path>` / `test-added:<path>` / `wontfix:<reason>`

## Matrix

| endpoint_or_pdu | transport | role | axis | current_state | test_file | gap | severity | action |
|---|---|---|---|---|---|---|---|---|
| _TODO: заполнить в Task 6 (cross-product inventory × roles × axes)_ | | | | | | | | |

## Machine-readable copy

См. `2026-04-21-api-coverage-matrix.csv` — тот же контент, одно сочетание на строку.
```

Create `docs/reports/2026-04-21-api-coverage-matrix.csv`:
```csv
endpoint_or_pdu,transport,role,axis,current_state,test_file,gap,severity,action
```
(один заголовок, тело заполним в Task 6)

- [ ] **Step 4: Создать findings-скелет**

Create `docs/reports/2026-04-21-api-review-phase-0-findings.md`:

```markdown
# Phase 0 Findings — 2026-04-21

**Scope:** инфраструктурная разведка перед фазами A1–C.

## Findings

### F1. OpenAPI location
_TODO: Task 5_

### F2. Integration test infrastructure
_TODO: Task 5_

### F3. Dual-charge Phase 2 status
_TODO: Task 5_

### F4. SMPP gateway vs protocol-layer split
_TODO: Task 5_

## Blockers for next phases

_TODO: Task 7_

## Go/no-go decision for Phase A1

_TODO: Task 7_
```

- [ ] **Step 5: Commit scaffold**

```bash
git add docs/reports/2026-04-21-api-surface-inventory.md \
        docs/reports/2026-04-21-api-coverage-matrix.md \
        docs/reports/2026-04-21-api-coverage-matrix.csv \
        docs/reports/2026-04-21-api-review-phase-0-findings.md
git commit -m "chore(api-review): phase 0 — scaffold inventory/matrix/findings

Пустые каркасы документов для Фазы 0. Контент добавляется в следующих коммитах.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

Expected: один коммит, 4 файла добавлены.

---

## Task 2: HTTP inventory

**Files:**
- Modify: `docs/reports/2026-04-21-api-surface-inventory.md` (секция "HTTP Endpoints")
- Read-only: `internal/gateway/client/router/router.go`, `internal/gateway/client/handlers/*.go`

- [ ] **Step 1: Прочитать router и выписать все HandleFunc**

Read `internal/gateway/client/router/router.go` полностью. Выписать все вызовы `HandleFunc` или аналогичные с их method+path.

Ожидаемый список (из предварительной разведки брейншторма):
- `GET /health`, `/health/live`, `/health/ready`
- `POST /sms/send`, `POST /sms/batch`, `GET /sms/status/{id}`, `GET /sms/history`, `GET /sms/scheduled`, `DELETE /sms/{id}`
- `GET /account/balance`, `GET /account/stats`
- `POST /webhooks`, `GET /webhooks`, `GET /webhooks/{id}`, `PUT /webhooks/{id}`, `DELETE /webhooks/{id}`
- `POST /lookup`, `POST /lookup/bulk`, `GET /lookup/history`
- `POST /templates`, `GET /templates`, `GET /templates/{id}`, `PUT /templates/{id}`, `DELETE /templates/{id}`, `GET /templates/{id}/audit`
- `POST /cascade/deliveries`, `GET /cascade/deliveries`, `GET /cascade/deliveries/{id}`, `GET /cascade/stats`
- `GET /docs/*` (redoc, swagger, grpc, openapi.yaml)

Точный список в реальном router.go может отличаться — источник истины именно router.go.

- [ ] **Step 2: Для каждого эндпоинта найти хендлер и service-call**

Для каждого `method+path` из Step 1:
1. Найти handler-функцию (например `smsHandlers.SendSMS`).
2. Открыть файл хендлера, записать `file:line` определения функции.
3. Внутри функции найти вызовы gRPC/service-клиентов (обычно `h.smsClient.Send...`, `h.<service>Client.<Method>`).
4. Записать в колонку `Service call`.

Для `DB tables written` — чаще всего прямых SQL в client-gateway нет, запись идёт через gRPC в сервис. В этом случае колонка = имя gRPC-метода (например `messagingv1.SendSMS`), а фактические таблицы смотрим в service-коде только если нужно для C-billing клетки в Task 6.

Для `Response shape` — структура, которую возвращает хендлер (имя Go-типа).

- [ ] **Step 3: Заполнить таблицу HTTP Endpoints**

Отредактировать `docs/reports/2026-04-21-api-surface-inventory.md`, заменить `_TODO_` в секции HTTP Endpoints на полную таблицу. Для строк, которые не удалось разобрать (например `/docs/*` — это static serve) — явно писать `n/a (docs)`.

- [ ] **Step 4: Commit**

```bash
git add docs/reports/2026-04-21-api-surface-inventory.md
git commit -m "chore(api-review): phase 0 — HTTP inventory

Таблица method+path → handler file:line → service call → DB/gRPC цель.
Заполнено на основе internal/gateway/client/router/router.go.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: SMPP command inventory

**Files:**
- Modify: `docs/reports/2026-04-21-api-surface-inventory.md` (секция "SMPP Commands")
- Read-only: `internal/smpp/server/handler.go`, `internal/gateway/smpp/server/handler.go`, `internal/smpp/protocol/constants.go`, `internal/smpp/protocol/pdu.go`

- [ ] **Step 1: Выписать обрабатываемые command_id**

Открыть `internal/smpp/server/handler.go`. Найти switch/case по `command_id` или аналогичный диспатч. Для каждого case записать:
- Symbolic name (например `bind_transmitter`, `submit_sm`, `deliver_sm`, `enquire_link`, `unbind`, `generic_nack`, `query_sm`).
- Hex-значение из `internal/smpp/protocol/constants.go`.

Повторить для `internal/gateway/smpp/server/handler.go` — там может быть **другой** слой диспатча (gateway-обёртка). Разделение двух файлов в отдельную колонку «layer» — добавить в таблицу.

- [ ] **Step 2: Для каждого command найти service-call**

Для каждой обработанной команды найти вниз по стеку:
1. Куда вызов идёт после парсинга PDU (обычно через `auth_adapter` → service-client).
2. Какой gRPC-метод или прямой service-call срабатывает (например `messagingv1.SendSMS`, `tarificationv1.TarifyMessage`).
3. Какие таблицы пишутся (тот же подход что в Task 2: обычно пишет сервис, не gateway).

- [ ] **Step 3: Обновить таблицу SMPP Commands**

Расширить схему таблицы в inventory до:
```
| PDU / Event | Layer | Handler file:line | Service call | DB tables written | Notes |
```

Где `Layer` = `protocol` (internal/smpp/) или `gateway` (internal/gateway/smpp/).

Заполнить все найденные в Step 1 + 2.

**Важно.** Отдельно выделить DLR-путь: `deliver_sm` — это не только приём входящих MO, но и путь доставки DLR. Если обработка различна — две строки (`deliver_sm (MO)` и `deliver_sm (DLR)`).

- [ ] **Step 4: Commit**

```bash
git add docs/reports/2026-04-21-api-surface-inventory.md
git commit -m "chore(api-review): phase 0 — SMPP command inventory

Таблица PDU → layer (protocol/gateway) → handler → service call.
Выделены отдельные строки для deliver_sm MO vs DLR.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: SMPP TLV inventory

**Files:**
- Modify: `docs/reports/2026-04-21-api-surface-inventory.md` (секция "SMPP Supported TLVs")
- Read-only: `internal/smpp/protocol/constants.go`, `internal/smpp/protocol/pdu_helper.go`, `internal/smpp/protocol/decoder.go`, `internal/smpp/protocol/encoder.go`

- [ ] **Step 1: Список всех объявленных TLV-тегов**

В `internal/smpp/protocol/constants.go` найти все константы с именами вида `Tag*` или `TLV_*` — hex-значение + symbolic name.

Типичный список SMPP v3.4, который должен быть:
- `tag_user_message_reference` (0x0204)
- `tag_message_payload` (0x0424)
- `tag_receipted_message_id` (0x001E)
- `tag_message_state` (0x0427)
- `tag_sar_msg_ref_num` (0x020C)
- `tag_sar_total_segments` (0x020E)
- `tag_sar_segment_seqnum` (0x020F)
- `tag_source_network_type` (0x000E) и др.

Записать ВСЕ объявленные — даже те, что не используются в хендлерах.

- [ ] **Step 2: Для каждого TLV — где реально читается/пишется**

Grep по проекту на использование каждой константы:

Run (пример):
```bash
grep -rn "TagUserMessageReference" internal/ --include="*.go"
```

Для каждого TLV определить:
- Используется в `submit_sm` (клиент → нас)? — колонка `Used in`.
- Используется в `deliver_sm` (нам → клиенту, включая DLR)? — та же колонка.
- На какое поле внутренней модели маппится (например `user_message_reference` → `messaging.Message.ClientRef`)? — колонка `Mapped to field`.

Если TLV объявлен, но ни один grep не нашёл использование — пометить `declared but unused`.

- [ ] **Step 3: Заполнить таблицу TLV**

Обновить секцию "SMPP Supported TLVs" в inventory. Схема:
```
| Tag (hex) | Symbolic name | Declared in | Used in submit_sm | Used in deliver_sm | Mapped to field | Notes |
```

- [ ] **Step 4: Commit**

```bash
git add docs/reports/2026-04-21-api-surface-inventory.md
git commit -m "chore(api-review): phase 0 — SMPP TLV inventory

Все объявленные TLV-теги, их использование в submit/deliver,
маппинг на внутренние поля. Declared-but-unused помечены отдельно.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Инфраструктурная разведка (findings F1–F4)

**Files:**
- Modify: `docs/reports/2026-04-21-api-review-phase-0-findings.md`
- Read-only: `go.mod`, `scripts/check.sh`, `.github/workflows/*`, `docs/openapi.yaml` (если есть), `internal/services/tarification/application/*.go`, `api/proto/billing/billing.proto`

- [ ] **Step 1: F1 — OpenAPI location и происхождение**

Найти `openapi.yaml` или `openapi.json` в репо:
```bash
find . -iname "openapi*" -not -path "*/node_modules/*" -not -path "*/.git/*" 2>/dev/null
```

Expected: один или несколько файлов, вероятно `docs/openapi.yaml` (потому что в router'е есть `/docs/openapi.yaml` handler).

Для каждого найденного файла:
1. Открыть, прочитать первые 50 строк.
2. Определить: это **рукописная спека** или **сгенерированная из кода** (ищем комментарий `# generated by ...`, `# DO NOT EDIT`, или заголовок с упоминанием swag/swaggo/openapi-generator).
3. Сравнить дату последнего изменения с последним изменением хендлеров: `git log -1 --format=%ai <openapi file>` vs `git log -1 --format=%ai internal/gateway/client/handlers/`.

Записать в F1:
- Путь к файлу.
- Рукописная / сгенерированная / гибрид.
- Возраст относительно хендлеров.
- **Вывод для Фазы A1:** если генерируется из кода — A1 тривиальна (drift невозможен), можно пропустить. Если рукописная — A1 обязательна.

- [ ] **Step 2: F2 — Integration test infra**

Проверить наличие Postgres/Redis в CI:
```bash
ls .github/workflows/ 2>/dev/null
grep -l "postgres\|redis" .github/workflows/*.yml 2>/dev/null
```

Проверить `scripts/check.sh` и `docker-compose*.yml`:
```bash
grep -l "postgres\|redis" docker-compose*.yml 2>/dev/null
```

Проверить наличие build-тега `integration` в существующих тестах:
```bash
grep -rn "//go:build integration\|// +build integration" --include="*.go" | head -20
```

Записать в F2:
- Есть ли postgres/redis services в CI workflows.
- Используется ли уже build-tag `integration` в репо.
- **Вывод для Фазы C:** если инфры нет — Фаза C упрощается до unit+fakes, integration-слой вырезается из Секции 4 спека.

- [ ] **Step 3: F3 — Dual-charge Phase 2 status**

По памяти (project_aggregator_decisions.md): требуется миграция `000098_aggregator_margin_log`, RPC `ChargeMessageDual`, вызов из `TarifyMessage`.

Проверить каждый пункт:
```bash
ls migrations/ 2>/dev/null | grep -i aggregator
grep -rn "ChargeMessageDual" --include="*.go" --include="*.proto"
grep -n "TarifyMessage" internal/services/tarification/application/*.go
```

Для найденного `TarifyMessage` прочитать — вызывает ли внутри `ChargeMessageDual` или обычный `Charge`.

Записать в F3:
- Миграция `000098` присутствует / отсутствует.
- RPC `ChargeMessageDual` определён в .proto / нет.
- `TarifyMessage` вызывает `ChargeMessageDual` / `Charge` / что-то ещё.
- **Вывод для Фазы C:** если не доделан, C-billing для `subaccount_parent` заведомо красный — отдельный план (memory его уже предписывает).

- [ ] **Step 4: F4 — SMPP gateway vs protocol split**

Открыть `internal/gateway/smpp/server/handler.go` и `internal/smpp/server/handler.go`. Определить роль каждого:
- gateway/smpp/server/handler.go — скорее всего: auth, session-to-client mapping, route to services.
- internal/smpp/server/handler.go — скорее всего: PDU dispatch, чистый протокольный уровень.

Проверить, **где именно** пробрасывается субаккаунтный контекст (client_id, parent_client_id). Grep:
```bash
grep -n "is_reseller\|parent_client" internal/gateway/smpp/server/*.go internal/smpp/server/*.go
```

В брейншторме уже выяснили: `internal/smpp/` его не знает, `internal/gateway/smpp/auth_adapter_test.go` — знает. Подтвердить.

Записать в F4:
- Ответственность каждого слоя в одной строке.
- Где `client_id` добавляется в контекст (файл:строка).
- Где `parent_client_id` / `is_reseller` читается (файл:строка).
- **Вывод для Фаз A6 и C:** A6 касается только протокольного уровня, C — только gateway-уровня. Разделение чёткое.

- [ ] **Step 5: Commit findings**

```bash
git add docs/reports/2026-04-21-api-review-phase-0-findings.md
git commit -m "chore(api-review): phase 0 — infra findings F1-F4

F1: openapi location/generation, F2: integration infra in CI,
F3: dual-charge Phase 2 status, F4: smpp gateway/protocol split.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Matrix skeleton с UNKNOWN строками

**Files:**
- Modify: `docs/reports/2026-04-21-api-coverage-matrix.md`
- Modify: `docs/reports/2026-04-21-api-coverage-matrix.csv`

- [ ] **Step 1: Построить cross-product**

На основе inventory (Task 2+3):
- **endpoint_or_pdu** — все HTTP-эндпоинты (кроме `/docs/*` и `/health*` — эти исключаем как n/a) + все SMPP PDU (включая `deliver_sm (DLR)`).
- **role** — ровно три: `client`, `subaccount_child`, `subaccount_parent`.
- **axis** — для HTTP: A1, A2, A3, C-billing, C-visibility, C-dlr-routing. Для SMPP: A2, A3, A6, C-billing, C-visibility, C-dlr-routing. (A1 к SMPP не применим, A6 — только SMPP.)

Правила применимости:
- C-billing — только write-операции (submit, cascade create, bulk lookup, template create). Read/GET без C-billing строки.
- C-visibility — только list/get-операции (history, list templates, list webhooks). Write без C-visibility (кроме случаев когда write затрагивает чужой ресурс).
- C-dlr-routing — только `submit_sm` и `deliver_sm (DLR)` + HTTP `POST /sms/send` и webhook-delivery-endpoint.
- A3 — только write-операции.

Применить правила, чтобы не плодить нерелевантные строки (например, нет смысла в `C-billing` для `GET /account/balance`).

- [ ] **Step 2: Сгенерировать строки**

Для каждой релевантной (endpoint, role, axis) тройки добавить строку:
```
| <endpoint> | <transport> | <role> | <axis> | UNKNOWN | | phase 0 skeleton | | |
```

Пример нескольких строк:
```
| POST /sms/send | HTTP | client | A1 | UNKNOWN | | phase 0 skeleton | | |
| POST /sms/send | HTTP | client | A2 | UNKNOWN | | phase 0 skeleton | | |
| POST /sms/send | HTTP | client | A3 | UNKNOWN | | phase 0 skeleton | | |
| POST /sms/send | HTTP | client | C-billing | UNKNOWN | | phase 0 skeleton | | |
| POST /sms/send | HTTP | client | C-visibility | UNKNOWN | | phase 0 skeleton | | |
| POST /sms/send | HTTP | client | C-dlr-routing | UNKNOWN | | phase 0 skeleton | | |
| POST /sms/send | HTTP | subaccount_child | A1 | UNKNOWN | | phase 0 skeleton | | |
... и т.д.
```

Заполнить markdown-таблицу в `.md` и дублировать в `.csv`.

- [ ] **Step 3: Подсчитать итоги**

В заголовке matrix.md обновить:
- `OK rows with test_file (ratchet baseline): 0` — остаётся 0.
- Добавить `Total rows: <N>` — посчитать фактически полученное число.

Грубый таргет из спека — около 245. Если получилось сильно меньше (<150) или больше (>350) — проверить логику применимости.

- [ ] **Step 4: Commit matrix skeleton**

```bash
git add docs/reports/2026-04-21-api-coverage-matrix.md \
        docs/reports/2026-04-21-api-coverage-matrix.csv
git commit -m "chore(api-review): phase 0 — matrix skeleton (UNKNOWN rows)

Cross-product endpoint×role×axis с правилами применимости
(write-ops для A3/C-billing, list-ops для C-visibility и т.п.).
Все строки в UNKNOWN — заполнение в следующих фазах.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Финализация Фазы 0 — blockers и go/no-go

**Files:**
- Modify: `docs/reports/2026-04-21-api-review-phase-0-findings.md` (секции Blockers и Go/no-go)
- Modify: `docs/reports/2026-04-21-api-surface-inventory.md` (указать итоговый commit hash в заголовке)

- [ ] **Step 1: Зафиксировать commit hash в inventory**

В `docs/reports/2026-04-21-api-surface-inventory.md` заменить `<ХЭШ>` в заголовке на текущий HEAD:
```bash
git rev-parse HEAD
```
(это будет ХЭШ предыдущего коммита — inventory снапшотит состояние на момент Task 4).

- [ ] **Step 2: Заполнить раздел Blockers**

В `findings.md` перечислить блокеры, если есть. Каждый блокер — формата:

```
### B1: <название>
**Фаза, которую блокирует:** A1 / A3 / C / ...
**Что не так:** <конкретика из F1-F4>
**Решение:** <что нужно сделать до старта той фазы>
```

Примеры возможных блокеров:
- Если F2 показал, что integration-инфры нет — блокер на Фазу C (надо либо добавить инфру, либо урезать Фазу C до unit+fakes).
- Если F3 показал, что dual-charge не доделан — не блокер, но предопределяет, что C-billing/subaccount_parent будет красным. Это нужно явно зафиксировать.
- Если F1 показал, что openapi сгенерирован — Фаза A1 превращается в «запустить генератор и проверить чистоту», 10 минут вместо плана на 2 дня.

- [ ] **Step 3: Go/no-go решение**

В секции "Go/no-go decision for Phase A1" записать явное решение:
- **GO** — если блокеров нет или они не касаются A1.
- **NO-GO** — если есть блокер, требующий action до начала A1. Указать что именно.

Если GO — дополнительно указать **объём A1**: количество HTTP-эндпоинтов из inventory, которые нужно сверить с openapi. Это даст writing-plans для Фазы A1 конкретную цифру задач.

- [ ] **Step 4: Commit финализации**

```bash
git add docs/reports/2026-04-21-api-surface-inventory.md \
        docs/reports/2026-04-21-api-review-phase-0-findings.md
git commit -m "docs(api-review): phase 0 — blockers and go/no-go for A1

Inventory заморожен на текущем master hash.
Findings содержат явное go/no-go решение для следующей фазы.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 5: Push и PR**

```bash
git push -u origin review/api-phase-0-inventory
```

Затем создать PR к master через gh CLI (если есть) или вручную:
```bash
gh pr create --title "API Review Phase 0: surface inventory & infra findings" --body "$(cat <<'EOF'
## Summary

- Inventory HTTP + SMPP поверхности клиентского API.
- Matrix skeleton (все ~245 строк в UNKNOWN — заполняется в фазах A1-C).
- Findings F1-F4: OpenAPI происхождение, integration infra в CI, статус dual-charge Phase 2, разделение smpp gateway vs protocol.
- Go/no-go решение для Фазы A1.

Spec: `docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md`

## Test plan

- [ ] Проверить, что inventory покрывает все HandleFunc из router.go (number match).
- [ ] Проверить, что все объявленные TLV-константы попали в TLV-таблицу.
- [ ] Проверить, что matrix-skeleton имеет разумный размер (~200-300 строк).
- [ ] Прочитать findings F1-F4 — факты согласуются с реальностью.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: PR создан, URL возвращён.

---

## После завершения Task 7

Когда PR Фазы 0 влит в master, **написать план Фазы A1** на основе findings:
- Если F1 показал генерируемый openapi — краткий план (1-2 task'а) на прогон генератора и diff.
- Если F1 показал рукописный openapi — полный план сверки (по одной task-группе на ресурс: sms, account, webhooks, lookup, templates, cascade).

Команда для следующего плана:
```
Используй superpowers:writing-plans для написания плана Фазы A1 на основе docs/reports/2026-04-21-api-review-phase-0-findings.md
```

---

## Self-review

**Spec coverage:**
- Секция 1 спека (скоуп/границы) — не реализуется кодом, проверяется content'ом документов ✅.
- Секция 2, Фаза 0 (inventory) — Task 1-4 ✅.
- Секция 3 (матрица) — Task 1 (каркас) + Task 6 (скелет с UNKNOWN) ✅. Живое заполнение — в следующих фазах, это ожидаемо.
- Секция 4 (тесты) — тестов в Фазе 0 не пишем, только фиксируем наличие инфры в F2 ✅.
- Секция 5 (артефакты/коммиты) — план использует `chore(api-review)` / `docs(api-review)` префиксы, по одной фазе = одна ветка = один PR ✅.
- Секция 6, риск про integration infra и openapi generation — F2 и F1 напрямую ✅.

**Placeholders:** TBD/TODO только внутри документов-каркасов (Task 1), заполняются в последующих тасках. В самом плане нет placeholder'ов.

**Type consistency:** схема матрицы одинакова в Task 1 и Task 6 (9 колонок). Схема inventory HTTP-таблицы одинакова в Task 1 и Task 2 (6 колонок). SMPP-таблица расширена с 5 до 6 колонок в Task 3 (добавлен `Layer`) — это явно указано в Step 3 Task 3.

**Gap check:** A4 (пагинация), A5 (версионирование), hard security — исключены спеком, в плане их нет ✅.

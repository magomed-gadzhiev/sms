# API Client & Subaccount Review v2 — Umbrella Spec

**Дата:** 2026-04-21
**Статус:** утверждён в брейншторме, ждёт user-review перед переходом к writing-plans
**Supersedes:** `docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md` (v1)
**Причина пересборки:** v1 упустил внешний gRPC (`cmd/client-gateway` открывает `CLIENT_GRPC_PORT`) и внутренний gRPC как отдельный контрактный слой; плюс прошлый исполнительный цикл пострадал от субагентских инцидентов с working tree.

## 1. Скоуп и декомпозиция

### 1.1. Транспорты в скоупе

- **HTTP внешний** — `cmd/client-gateway` → `internal/gateway/client/*`.
- **gRPC внешний** — тот же `cmd/client-gateway`, открывает `CLIENT_GRPC_PORT` через `internal/gateway/client/grpc/`.
- **SMPP внешний** — `cmd/smpp-gateway` → `internal/gateway/smpp/*` + `smppv1` proto (контрол-плейн).
- **gRPC внутренний** — межсервисные контракты в `api/proto/*`. Критичен потому что субаккаунтный контекст пробрасывается именно там.

### 1.2. Deprecated, вне скоупа

- `cmd/api` — legacy monolithic (Kafka producer + прямые DB repo).
- `cmd/smpp-server` — legacy protocol server без Redis/gRPC-control.
- `cmd/portal-gateway`, `cmd/admin-gateway` — другие интерфейсы.

Статус «legacy» — принят по архитектурным сигналам (`cmd/api` пишет в Kafka напрямую, `cmd/client-gateway` — через gRPC к микросервисам). **Должен быть подтверждён в Umbrella Phase 0** (R1), иначе скоуп пересматривается.

### 1.3. Роли

- `client` — обычный клиент (`is_reseller = false`, `parent_client_id IS NULL`).
- `subaccount_child` — ребёнок (`is_reseller = false`, `parent_client_id` задан).
- `subaccount_parent` — агрегатор (`is_reseller = true`).

Дискриминатор ролей — `is_reseller` + `parent_client_id`, согласно memory `project_aggregator_decisions.md` (2026-04-20). `account_type` запрещён.

### 1.4. Оси ревью

- **A1** — HTTP OpenAPI ↔ код соответствие.
- **A1-grpc** — gRPC proto ↔ реализация (внешний + внутренний).
- **A2** — единый формат ошибок: HTTP envelope + gRPC status.Code/details + SMPP ESME_*.
- **A3** — идемпотентность write-операций: HTTP `Idempotency-Key`, gRPC metadata `idempotency-key`, SMPP дедуп.
- **A6** — SMPP-контракт (TLV, data_coding, длинные сообщения, DLR, registered_delivery).
- **A7** — gRPC контракт tenant-контекста (`client_id`/`parent_client_id`/`is_reseller` не теряются на границе сервисов).
- **C** — сквозная корректность субаккаунта: C-billing, C-visibility, C-dlr-routing через все транспорты.

### 1.5. Декомпозиция на циклы

| Цикл | Оси | Артефакты | Ветка |
|---|---|---|---|
| **1. Контракт** | A1, A1-grpc, A2, A3 | spec, plan, findings, updated openapi, `api-error-contract.md` | `review/api-contract` |
| **2. SMPP и tenant-glue** | A6, A7 | spec, plan, findings, `smpp-contract.md`, `tenant-context-audit.md` | `review/api-smpp-tenant` |
| **3. Сквозная корректность** | C | spec, plan, findings, integration-тесты | `review/api-subaccount-e2e` |

**Порядок:** Cycle 1 и Cycle 2 параллельно (непересекающиеся файлы). Cycle 3 — после 1 и 2, опирается на их findings.

### 1.6. Вне скоупа (явно)

- Portal HTTP API (`internal/gateway/portal/`).
- Admin gateway.
- Общий security review (auth/scopes/rate-limiting кроме tenant-isolation в рамках C).
- A4 (пагинация), A5 (версионирование API).
- Performance / нагрузочное.

### 1.7. Старый спек v1

`docs/superpowers/specs/2026-04-21-api-client-subaccount-review-design.md` остаётся в репе с пометкой `SUPERSEDED by 2026-04-21-api-review-v2-umbrella.md` (добавляется отдельным коммитом в Umbrella Phase 0). История решений сохраняется, исполнять v1 запрещено.

## 2. Общие инварианты

Применяются ко **всем** циклам, не дублируются в cycle-спеках.

### 2.1. Форма γ

**Мелочь** = фиксим в PR цикла. Критерий ε:
- ≤50 строк diff'а.
- 1 файл.
- Без миграций.
- Без новых зависимостей.

**Крупное** = отдельный план через `superpowers:writing-plans`.

**Хард-гейт поверх ε (автоматически крупное):**
- Любой diff в `internal/services/billing/` или `internal/services/tarification/`.
- Любое изменение `.proto` файлов.
- Любое изменение tenant-injection кода (`internal/gateway/smpp/server/auth_adapter.go` и аналоги).

### 2.2. Ветка, worktree, субагенты

Прямая реакция на инцидент прошлой сессии (субагент утащил network-stats фиксы в API-review коммит).

**Правила для контроллера:**
- Перед каждым cycle-ветвлением — `git status` чист (stash/commit/gitignore untracked).
- Перед dispatch каждого субагентского task'а — контроллер читает `git status` и передаёт субагенту явный список файлов, которые он имеет право создать/тронуть.
- Контроллер готовит ветку сам. Субагент не делает `git checkout master` / `git pull` / `git branch`.

**Правила для субагента:**
- Начать работу с `git status`. Если есть лишнее untracked — BLOCKED.
- Перед коммитом — повторный `git status`. Коммитим только явно разрешённые файлы.
- Никогда `git add .` или `-A`. Только явные пути.
- Если Device Guard роняет pre-commit hook — BLOCKED, без `--no-verify`.

### 2.3. Review-gate

Унаследован из CLAUDE.md и memory `feedback_review_gate_no_skip`.

- **Pure-doc task** (inventory, matrix, findings) → только spec-compliance-reviewer.
- **Code task** → spec-compliance + code-quality (оба обязательны).
- **Mixed task** → оба, code-quality с подсказкой про малый объём кода.
- Post-facto review запрещён. Review — ДО следующего task'а.

### 2.4. Покрытие тестами как deliverable

Каждый цикл обязан:
- Cycle 1: unit на error-mapping, unit на idempotency middleware, guard-тест openapi ↔ router.
- Cycle 2: unit на TLV-parsing (если появятся константы), unit на tenant-propagation в каждом gRPC handler.
- Cycle 3: integration-тесты с Postgres+Redis (зависит от R3 / B1).

Ratchet: заголовок каждой матрицы хранит `OK rows with test_file: N`. Следующая фаза не уменьшает N. Monotonic.

### 2.5. Блокеры Cycle 3 (унаследованы из v1 Phase 0 findings)

Могли устареть, перепроверяются в Umbrella Phase 0:

- **B1** — нет integration-инфры в CI (postgres/redis service-containers).
- **B2** — AuthAdapter `user_id=client_id` kludge в `internal/gateway/smpp/server/auth_adapter.go`.
- **B3** — dual-charge (`ChargeMessageDual`) вызывается только из `CommitCharge` за feature flag, не из `TarifyMessage`.

Если блокер подтверждается — закрывается отдельным мини-планом до старта Cycle 3.

## 3. Umbrella Phase 0 — общие разведданные

Единая разведка до старта любого цикла. Артефакт: `docs/reports/2026-04-21-api-review-v2-surface.md`.

### 3.1. HTTP inventory (client-gateway)

Все `HandleFunc` в `internal/gateway/client/router/router.go` → таблица `method+path → handler file:line → service call → response shape`. Колонка «покрыто тестом?» — grep по `*_test.go` в handlers.

Текущее покрытие (к дате брейншторма): есть `account_test`, `lookup_test`, `sms_test`; нет `cascade_test`, `webhooks_test`, `templates_test`.

### 3.2. gRPC inventory (внешний)

Все gRPC services в `internal/gateway/client/grpc/*.go` → таблица `service.RPC → handler file:line → внутренний gRPC клиент который вызывается`.

Из `cmd/client-gateway/main.go` видно: `messagingv1` — единственный явный внешний proto (верифицировать в Phase 0, могут быть ещё).

### 3.3. SMPP inventory

Только новый слой: `cmd/smpp-gateway` + `internal/gateway/smpp/`. Legacy `internal/smpp/server/` не трогаем.

Таблицы:
- PDU → handler file:line → service call → notes (layer, DLR-ветвление).
- TLV-константы из `internal/smpp/protocol/constants.go` + реальное использование (grep по коду).
- `smppv1` RPC (control-plane) → handler → что делает.

### 3.4. gRPC contract inventory (внутренний)

Топ-5 сервисов hot-path:
- `messagingv1` — отправка сообщений.
- `billingv1` — списание/баланс.
- `tarificationv1` — расчёт цены.
- `cascadev1` — каскадная доставка.
- `webhookv1` — колбэки.

Для каждого RPC в этих сервисах: `RPC → как передаётся client_id (request field / metadata / не передаётся) → caller → implementer`.

Остальные сервисы (`analyticsv1`, `templatev1`, `routingv1`, и т.д.) — помечаются `scope:out` без анализа.

### 3.5. Перепроверка блокеров B1-B3

- **B1:** `.github/workflows/*` — есть ли postgres/redis services.
- **B2:** grep по `auth_adapter.go` и соседним — остался ли `user_id=client_id`.
- **B3:** прочитать `TarifyMessage` в `tarification_service.go` — вызывает ли `ChargeMessageDual`.

### 3.6. Подтверждение R1 (cmd/api legacy status)

Проверить deployment-конфигурацию или прод-логи: активен ли `cmd/api` в проде. Если да — пауза, скоуп пересматривается.

### 3.7. Что Phase 0 НЕ делает

- Не заполняет матрицу покрытия (это артефакт конкретного цикла).
- Не пишет findings про «что сломано».
- Не создаёт фиксов.

### 3.8. Передача управления циклам

После Phase 0 umbrella-спек обновляется статусами (R1 подтверждён/нет, B1-B3 закрыты/нет). Только тогда стартуют Cycle 1 и/или 2. Cycle 3 ждёт 1+2 и B1-B3.

## 4. Матрица покрытия — формат и размеры

Одна матрица на цикл (не общая умбрелла-матрица).

### 4.1. Схема строки (одинакова во всех циклах)

```
| surface | transport | role | axis | current_state | test_file | gap | severity | action |
```

- `surface` — `POST /api/v1/sms/send`, `messagingv1.MessagingService.SendMessage`, `SMPP submit_sm`, ...
- `transport` — `HTTP` / `gRPC-external` / `gRPC-internal` / `SMPP`.
- `role` — `client` / `subaccount_child` / `subaccount_parent`.
- `axis` — конкретная ось скоупа цикла.
- `current_state` — `OK` / `DRIFT` / `MISSING` / `BROKEN` / `UNKNOWN`. `UNKNOWN` не маскировать под `OK`.
- `test_file` — путь или пусто.
- `gap` — описание.
- `severity` — `critical` (биллинг/auth) / `major` (контракт) / `minor`.
- `action` — `fixed-in-PR#<N>` / `plan:<path>` / `test-added:<path>` / `wontfix:<reason>`.

### 4.2. Матрица Cycle 1 (A1, A1-grpc, A2, A3)

Правила применимости:
- **A1** — только HTTP.
- **A1-grpc** — только gRPC-external.
- **A2** — все 4 транспорта.
- **A3** — только write-операции (HTTP POST, gRPC unary writes, SMPP submit_sm, gRPC-internal writes).

Ожидаемый размер: ~300 строк.

Файлы: `docs/reports/2026-04-21-cycle1-contract-matrix.md` + `.csv`.

### 4.3. Матрица Cycle 2 (A6, A7)

Правила применимости:
- **A6** подоси: `A6-tlv`, `A6-datacoding`, `A6-long-msg`, `A6-dlr`, `A6-registered-delivery` — отдельные axis-значения.
- **A6** — только SMPP.
- **A7** — только gRPC (external + internal), только RPC с tenant-фактором (исключаем `Health`, `ListOperators` без клиентского фильтра и т.п.).

Ожидаемый размер: ~180 строк.

Файлы: `docs/reports/2026-04-21-cycle2-smpp-glue-matrix.md` + `.csv`.

### 4.4. Матрица Cycle 3 (C-billing, C-visibility, C-dlr-routing)

Правила применимости:
- `C-billing` — write-операции, создающие charge.
- `C-visibility` — read/list операции.
- `C-dlr-routing` — submit_sm, deliver_sm (DLR), POST /webhooks.

Ожидаемый размер: ~90 строк.

Файлы: `docs/reports/2026-04-21-cycle3-subaccount-matrix.md` + `.csv`.

### 4.5. Снапшот и ratchet

Матрица — снапшот на дату ревью, не живой документ. Обновляется только при следующем ревью этой оси. Живая защита — тесты, на которые она ссылается.

## 5. Execution flow — ритуал цикла

### 5.1. Жизненный цикл цикла

1. **Phase 0** — matrix skeleton. Все строки в `UNKNOWN`, пустые findings. Один коммит, spec-review only.
2. **Phase 1..N** — одна фаза = одна ось (или подось) из скоупа цикла. Каждая фаза:
   - Анализ: субагент обновляет клетки матрицы.
   - Мелкие фиксы (ε + хард-гейт) в том же PR.
   - Крупные находки → отдельный план через `writing-plans`, клетка action=`plan:<file>`.
   - Тесты на закрытые мелочи; `test_file` обновляется.
   - Review-gate на каждый коммит.
3. **Phase F** — финализация: update OK-count, сводный `<cycle>-findings.md`, push, PR к master.
4. umbrella-спек обновляется: цикл `completed: YYYY-MM-DD`, счётчики.

### 5.2. Жёсткие правила субагентов

См. 2.2. Повторяются буквально в каждом dispatch prompt'е.

### 5.3. Review-gate

См. 2.3.

### 5.4. Один PR на весь цикл

- Ветка `review/<cycle-name>` — одна на цикл.
- Серия фазовых коммитов внутри.
- Один PR к master.

**Изменение от v1:** в v1 было «каждая фаза — отдельный PR». Это породило дрейф состояния ветки в прошлой сессии. v2 возвращается к одному PR на цикл, с оговоркой что внутри цикла 3-4 фазы, не 7+.

### 5.5. Параллельность циклов

- Cycle 1 и 2 — параллельно (разные файлы в скоупе).
- Cycle 3 — последовательно после 1 и 2.

### 5.6. Завершение цикла

- PR влит в master.
- umbrella-спек помечает цикл completed.
- Крупные планы цикла — файлы в `docs/superpowers/plans/`, но не обязательно выполнены. Берутся пользователем по приоритету.

## 6. Риски

- **R1** — `cmd/api` vs `cmd/client-gateway` legacy-статус не подтверждён фактом, только сигналом. Первая проверка в Phase 0.
- **R2** — gRPC-internal inventory может разрастись. Митигация: топ-5 сервисов.
- **R3** — integration-инфра для Cycle 3. Зависит от B1.
- **R4** — субагент ломает working tree. Митигация в 2.2, но не железобетонная. Stop-the-line при повторе.
- **R5** — ratchet бросается. Митигация: explicit check в code-review финальных коммитов.

## 7. Чего не делаем

- Не закрываем dual-charge Phase 2 в рамках ревью (отдельная работа, есть в memory).
- Не мигрируем legacy (`cmd/api`, `cmd/smpp-server`).
- Не добавляем новые gRPC services / proto.
- Не пишем release-notes.

## 8. Выход к writing-plans

Следующий шаг — план только **Umbrella Phase 0** (inventory + R1/B1-B3 check). Планы Cycle 1/2/3 пишутся отдельно, каждый со своим брейнштормом, когда до них доходит очередь.

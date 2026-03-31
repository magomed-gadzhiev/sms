# Tasks: Multichannel Cascade Delivery

**Input**: Design documents from `/specs/012-multichannel-cascade/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Organization**: Задачи сгруппированы по user story для независимой реализации и тестирования каждой истории.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Можно выполнять параллельно (разные файлы, нет зависимостей)
- **[Story]**: К какой user story относится задача (US1–US5)
- Каждая задача содержит точный путь к файлу

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Базовая инфраструктура нового сервиса — proto, entry point, конфиг, compose

- [X] T001 Скопировать контракт `specs/012-multichannel-cascade/contracts/cascade.proto` в `api/proto/cascade/cascade.proto` и сгенерировать Go-код в `api/proto/cascadev1/` (команда: `protoc --go_out=... --go-grpc_out=...`)
- [X] T002 Создать entry point `cmd/services/cascade-service/main.go` по паттерну `cmd/services/messaging-service/main.go` — инициализация Viper-конфига, zerolog, pgx-пула, Redis, Sarama, gRPC-сервера; порт `:9110`
- [X] T003 [P] Добавить четыре новых Kafka-топика (`cascade.start`, `cascade.attempt.send`, `cascade.attempt.result`, `cascade.billing`) и адрес `cascade_service_addr: :9110` в `configs/config.example.yaml`
- [X] T004 [P] Добавить сервис `cascade-service` в `deployments/docker-compose.yml` (образ, переменные окружения, depends_on: postgres, kafka, redis, routing-service, tarification-service, billing-service)

**Checkpoint**: Структура сервиса создана — можно двигаться к домену и миграциям

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Доменные сущности, репозитории и миграции — всё, что блокирует реализацию любой user story

**⚠️ CRITICAL**: Ни одна user story не может начаться до завершения этой фазы

- [X] T005 Создать миграцию `migrations/000067_cascade_channels.up.sql` — таблица `delivery_channels` (id UUID PK, channel_type TEXT UNIQUE NOT NULL, name, description, config JSONB, active BOOLEAN, created_at/updated_at TIMESTAMPTZ) + соответствующий `.down.sql`
- [X] T006 Создать миграцию `migrations/000068_delivery_strategies.up.sql` — таблицы `delivery_strategies` (id UUID PK, name TEXT UNIQUE, description, mode TEXT, active BOOLEAN, timestamps) и `delivery_strategy_steps` (id UUID PK, strategy_id FK, channel_id FK, step_order INT, timeout_s INT, billable BOOLEAN, UNIQUE(strategy_id, step_order), UNIQUE(strategy_id, channel_id)) + `.down.sql`
- [X] T007 Создать миграцию `migrations/000069_deliveries.up.sql` — партиционированная таблица `deliveries` (monthly by created_at: id UUID, client_id, message_id, strategy_id, recipient, text, sender_name, status TEXT, current_step INT, delivered_via TEXT, total_cost NUMERIC(12,4), currency TEXT DEFAULT 'RUB', request_id, timestamps) и `delivery_attempts` (monthly: id UUID, delivery_id, channel_id, channel_type TEXT, step_order, status TEXT, provider_ref, cost NUMERIC(12,4), currency DEFAULT 'RUB', error_message, sent_at, result_at, created_at) + индексы по delivery_id, status, created_at + `.down.sql`
- [X] T008 Создать миграцию `migrations/000070_operator_channel_support.up.sql` — таблица `operator_channel_support` (id UUID PK, operator_id UUID FK → operators(id), channel_type TEXT, supported BOOLEAN DEFAULT true, notes TEXT, updated_at TIMESTAMPTZ, updated_by UUID, UNIQUE(operator_id, channel_type)) + seed для SMS у всех существующих операторов + `.down.sql`
- [X] T009 [P] Реализовать доменные сущности в `internal/services/cascade/domain/`:
  - `channel.go` — интерфейс `Channel` (методы `Type() ChannelType`, `Send(ctx, attempt, cfg) error`), константы `ChannelType` (sms, flash_call, reverse_call, messenger)
  - `channel_config.go` — сущность `ChannelConfig` (ID, ChannelType, Name, Description, Config map, Active)
  - `strategy.go` — сущности `DeliveryStrategy` (ID, Name, Mode sequential|parallel, Steps, Active) и `StrategyStep` (ChannelID, StepOrder, TimeoutS, Billable)
  - `delivery.go` — сущность `Delivery` (ID, ClientID, StrategyID, Recipient, Text, Status, CurrentStep, DeliveredVia, TotalCost, Currency) + state machine методы (Start, MarkDelivered, MarkFailed, MarkCancelled) + константы `DeliveryStatus`
  - `attempt.go` — сущность `DeliveryAttempt` (ID, DeliveryID, ChannelID, ChannelType, StepOrder, Status, ProviderRef, Cost, Currency, ErrorMessage, SentAt, ResultAt) + константы `AttemptStatus` (pending, sent, delivered, failed, timeout, skipped, late_duplicate)
  - `operator_support.go` — сущность `OperatorChannelSupport` (OperatorID, ChannelType, Supported, Notes)
  - `errors.go` — доменные ошибки (ErrDeliveryNotFound, ErrStrategyNotFound, ErrChannelNotFound, ErrInvalidTransition, ErrLateDuplicate)
  - `repository.go` — интерфейсы `ChannelRepository`, `StrategyRepository`, `DeliveryRepository`, `AttemptRepository`, `OperatorSupportRepository`
- [X] T010 [P] Реализовать Kafka event structs в `internal/services/cascade/infrastructure/kafka/events.go` — `CascadeStartEvent`, `CascadeAttemptSendCommand`, `CascadeAttemptResultEvent`, `CascadeBillingCommand` (все с полем `SchemaVersion string = "1"`, json-теги)
- [X] T011 Реализовать postgres-репозитории в `internal/services/cascade/infrastructure/postgres/`:
  - `channel_repo.go` — реализует `ChannelRepository` (List, Get, Create, Update, Toggle)
  - `strategy_repo.go` — реализует `StrategyRepository` (List, Get, Create, Update, Delete, ListActive); включает JOIN с `delivery_strategy_steps` при чтении
  - `delivery_repo.go` — реализует `DeliveryRepository` (Create, Get, Update status/cost/step, List с фильтрами, Stats); учитывает партиционированность
  - `attempt_repo.go` — реализует `AttemptRepository` (Create, Get, UpdateStatus, ListByDelivery, FindPendingTimedOut)
  - `operator_support_repo.go` — реализует `OperatorSupportRepository` (List, Upsert)
- [X] T012 [P] Реализовать Kafka producer в `internal/services/cascade/infrastructure/kafka/producer.go` — методы `PublishAttemptSend(cmd CascadeAttemptSendCommand)`, `PublishAttemptResult(evt CascadeAttemptResultEvent)`, `PublishBilling(cmd CascadeBillingCommand)` с использованием `sarama.SyncProducer`

**Checkpoint**: Фундамент готов — все user stories могут реализовываться параллельно

---

## Phase 3: User Story 1 — Отправка OTP через каскад каналов (Priority: P1) 🎯 MVP

**Goal**: Клиент отправляет запрос на каскадную доставку; система выполняет flash call → fallback SMS через Kafka event-driven оркестрацию; webhook от провайдера подтверждает доставку

**Independent Test**: Отправить `POST /cascade/deliveries` с `strategy_id = "flash-call-then-sms"`, убедиться что сначала идёт flash call attempt, при таймауте — SMS attempt, при успехе SMS — delivery.status = "delivered", delivered_via = "sms"

- [X] T013 [P] [US1] Реализовать SMS-адаптер в `internal/services/cascade/channels/sms/adapter.go` — публикует `CascadeAttemptSendCommand` в топик `cascade.attempt.send` с `channel_type = "sms"`; реализует интерфейс `domain.Channel`
- [X] T014 [P] [US1] Реализовать Flash Call адаптер в `internal/services/cascade/channels/flash_call/adapter.go` — вызывает HTTP-провайдера с параметрами из `ChannelConfig`, возвращает `provider_ref`; реализует интерфейс `domain.Channel`; не ждёт результата синхронно (результат приходит через webhook → Kafka)
- [X] T015 [US1] Реализовать `internal/services/cascade/application/cascade_service.go` — методы `StartCascade(ctx, delivery)` (создаёт Delivery, публикует `CascadeStartEvent`), `ProcessAttemptResult(ctx, result)` (обновляет статус attempt, принимает решение о следующем шаге или завершении каскада), `HandleLateDuplicate(ctx, attemptID)` (помечает attempt как `late_duplicate`, не тарифицирует повторно); содержит карту `channelAdapters map[ChannelType]Channel`
- [X] T016 [US1] Реализовать `internal/services/cascade/application/scheduler.go` — поллинг pending attempts каждые 5 секунд (паттерн Start/Stop), запрос к `AttemptRepository.FindPendingTimedOut`, публикация `CascadeAttemptResultEvent{status: "timeout"}` для каждой просроченной попытки через Kafka producer
- [X] T017 [US1] Реализовать Kafka orchestrator consumer в `internal/services/cascade/infrastructure/kafka/orchestrator.go` — подписывается на топики `cascade.start` и `cascade.attempt.result`; batch consumer (batch=100, timeout=50ms); вызывает `CascadeService.StartCascade` и `CascadeService.ProcessAttemptResult`; идемпотентность: проверяет текущий статус delivery/attempt перед обработкой
- [X] T018 [US1] Реализовать gRPC-обработчик `internal/services/cascade/grpc/handler.go` — метод `CreateDelivery`: валидирует запрос, проверяет стратегию, создаёт `Delivery` в БД, публикует `CascadeStartEvent`; возвращает `DeliveryResponse`
- [X] T019 [US1] Собрать `cmd/services/cascade-service/main.go` — инициализировать все компоненты (pgx-пул, Redis, Sarama producer + consumer, SMS adapter, Flash Call adapter, CascadeService, Scheduler, gRPC-сервер с CascadeService handler), запустить Scheduler.Start() и Orchestrator consumer
- [X] T020 [US1] Реализовать webhook-обработчик flash call в `internal/gateway/portal/handlers/cascade_webhook.go` — `POST /webhooks/cascade/flash-call/{attempt_id}`: верифицирует HMAC-подпись из заголовка `X-Flash-Signature` (secret из channel config), публикует `CascadeAttemptResultEvent{status: "delivered"|"failed"}` в Kafka-топик `cascade.attempt.result`; возвращает 409 если attempt уже финализирован
- [X] T021 [US1] Добавить в `internal/gateway/client/handlers/cascade.go` обработчик `POST /cascade/deliveries` — принимает `{strategy_id, recipient, text, sender_name?, message_id?}`, вызывает gRPC `CascadeService.CreateDelivery`, возвращает `201 {id, status, strategy_id, recipient, created_at}`; проверяет баланс через billing-service (402 при недостатке)
- [X] T022 [US1] Примонтировать маршруты каскада в `internal/gateway/client/router/router.go` — `POST /cascade/deliveries`; gRPC-клиент к cascade-service на адресе из конфига
- [X] T023 [US1] Примонтировать webhook-маршрут в `internal/gateway/portal/router/router.go` — `POST /webhooks/cascade/flash-call/{attempt_id}`

**Checkpoint**: US1 полностью функционально и независимо тестируемо — каскадная доставка работает end-to-end

---

## Phase 4: User Story 2 — Управление каналами и стратегиями (Priority: P2)

**Goal**: Администратор добавляет каналы (flash call, SMS), создаёт стратегии каскада через UI; стратегия становится доступна клиентам

**Independent Test**: Зайти в admin UI → добавить канал flash_call → создать стратегию "flash-call-then-sms" → убедиться что стратегия появляется при `GET /admin/delivery-strategies`

- [X] T024 [P] [US2] Реализовать `internal/services/cascade/application/channel_service.go` — методы `List`, `Get`, `Create` (валидирует уникальность channel_type), `Update`, `Toggle`; использует `ChannelRepository`
- [X] T025 [P] [US2] Реализовать `internal/services/cascade/application/strategy_service.go` — методы `List(activeOnly)`, `Get`, `Create` (валидирует channel_ids существуют, step_order уникальны), `Update` (запрещает изменение steps при наличии active deliveries → 409), `Delete` (аналогично); использует `StrategyRepository`
- [X] T026 [US2] Добавить в `internal/services/cascade/grpc/handler.go` реализацию `ChannelAdminService` (ListChannels, GetChannel, CreateChannel, UpdateChannel, ToggleChannel) и `StrategyAdminService` (ListStrategies, GetStrategy, CreateStrategy, UpdateStrategy, DeleteStrategy, GetOperatorChannelSupport, UpdateOperatorChannelSupport)
- [X] T027 [US2] Реализовать admin-обработчики каналов в `internal/gateway/portal/handlers/cascade_channels.go` — `GET /admin/channels`, `GET /admin/channels/{id}`, `POST /admin/channels`, `PUT /admin/channels/{id}`, `PUT /admin/channels/{id}/toggle`; вызывает gRPC `ChannelAdminService`
- [X] T028 [US2] Реализовать admin-обработчики стратегий и OCS-матрицы в `internal/gateway/portal/handlers/cascade_strategies.go` — `GET /admin/delivery-strategies`, `GET /admin/delivery-strategies/{id}`, `POST /admin/delivery-strategies`, `PUT /admin/delivery-strategies/{id}`, `DELETE /admin/delivery-strategies/{id}`, `GET /admin/operator-channel-support`, `PUT /admin/operator-channel-support`; вызывает gRPC `StrategyAdminService`
- [X] T029 [US2] Примонтировать admin-маршруты в `internal/gateway/portal/router/router.go` — `/admin/channels/*`, `/admin/delivery-strategies/*`, `/admin/operator-channel-support`; роль `admin` required; gRPC-клиент к cascade-service
- [X] T030 [P] [US2] Создать TypeScript API-клиент в `portal-frontend/src/api/cascade.ts` — функции `listChannels()`, `createChannel(data)`, `updateChannel(id, data)`, `toggleChannel(id, active)`, `listStrategies()`, `createStrategy(data)`, `updateStrategy(id, data)`, `deleteStrategy(id)`, `getOperatorChannelSupport(operatorId?)`, `updateOperatorChannelSupport(data)`
- [X] T031 [P] [US2] Создать `portal-frontend/src/pages/channels/ChannelsPage.tsx` — таблица каналов с колонками: тип, название, статус (активен/отключён), действия (редактировать, toggle); кнопка "Добавить канал"
- [X] T032 [P] [US2] Создать `portal-frontend/src/pages/channels/ChannelFormModal.tsx` — модальная форма создания/редактирования канала (поля: channel_type select, name, description, config JSON textarea)
- [X] T033 [P] [US2] Создать `portal-frontend/src/pages/delivery-strategies/DeliveryStrategiesPage.tsx` — таблица стратегий (название, режим sequential|parallel, кол-во шагов, статус); кнопка "Добавить стратегию"; кнопка "OCS матрица"
- [X] T034 [P] [US2] Создать `portal-frontend/src/pages/delivery-strategies/StrategyFormModal.tsx` — форма создания/редактирования стратегии (name, description, mode radio, динамический список шагов: выбор канала, step_order, timeout_s, billable checkbox)
- [X] T035 [P] [US2] Создать `portal-frontend/src/pages/delivery-strategies/OperatorSupportMatrix.tsx` — таблица operator × channel_type с чекбоксами; заголовки строк — операторы, столбцы — типы каналов; кнопка сохранения построчно
- [X] T036 [US2] Добавить маршруты в `portal-frontend/src/App.tsx` — `/admin/channels`, `/admin/delivery-strategies`; доступны только для роли admin

**Checkpoint**: US2 полностью функционально — администратор может управлять каналами и стратегиями через UI

---

## Phase 5: User Story 3 — Мониторинг доставки через каскад (Priority: P2)

**Goal**: Клиент видит полную историю доставки с детализацией по попыткам: канал, статус, стоимость, временные метки

**Independent Test**: После отправки через каскад открыть `GET /cascade/deliveries/{id}` и убедиться что ответ содержит все попытки с `channel_type`, `status`, `cost`, `sent_at`, `result_at`

- [X] T037 [US3] Реализовать `internal/services/cascade/application/delivery_service.go` — методы `GetDelivery(ctx, deliveryID, clientID)`, `ListDeliveries(ctx, filter)`, `GetStats(ctx, clientID, strategyID, dateFrom, dateTo)` (считает total, delivered_count, failed_count, rate, per-channel stats, avg_cost, total_cost); использует `DeliveryRepository` и `AttemptRepository`
- [X] T038 [US3] Добавить в `internal/services/cascade/grpc/handler.go` методы `GetDelivery`, `ListDeliveries`, `GetDeliveryStats` — делегируют в `DeliveryService`; GetDelivery возвращает полный список attempts; ListDeliveries поддерживает фильтры (strategy_id, status, date range, pagination)
- [X] T039 [US3] Добавить в `internal/gateway/client/handlers/cascade.go` обработчики: `GET /cascade/deliveries` (фильтры: strategy_id, status, date_from, date_to, page, page_size), `GET /cascade/deliveries/{id}` (включает attempts), `GET /cascade/stats`; добавить маршруты в `internal/gateway/client/router/router.go`
- [X] T040 [P] [US3] Создать `portal-frontend/src/pages/cascade-history/CascadeHistoryPage.tsx` — таблица доставок с фильтрами (стратегия, статус, период); колонки: ID, получатель, стратегия, статус, канал доставки, стоимость, дата; клик → детали
- [X] T041 [P] [US3] Создать `portal-frontend/src/pages/cascade-history/CascadeDeliveryDetail.tsx` — детальный вид: хедер (статус, delivered_via, total_cost, recipient), timeline всех попыток (канал, статус, стоимость, sent_at, result_at, error_message)
- [X] T042 [US3] Добавить маршруты в `portal-frontend/src/App.tsx` — `/cascade/history`, `/cascade/history/:id`; добавить функции `listDeliveries(filter)`, `getDelivery(id)`, `getDeliveryStats(filter)` в `portal-frontend/src/api/cascade.ts`

**Checkpoint**: US3 функционально — клиент видит полную историю и детали каждой каскадной доставки

---

## Phase 6: User Story 4 — Проверка доступности канала для получателя (Priority: P3)

**Goal**: Перед каждым шагом каскада система проверяет поддержку канала оператором получателя; недоступные каналы пропускаются мгновенно без ожидания таймаута

**Independent Test**: Настроить `operator_channel_support` так чтобы оператор МТС не поддерживал flash_call; отправить на МТС-номер через стратегию "flash_call → SMS"; убедиться что attempt flash_call имеет статус "skipped", SMS попытка сделана сразу

- [X] T043 [US4] Реализовать `internal/services/cascade/application/reachability_service.go` — метод `CheckReachability(ctx, msisdn, channelType) (bool, error)`: 1) проверить Redis-кеш по ключу `reachability:{msisdn}:{channel_type}` (TTL 1h), 2) при cache miss — вызвать `routing-service` gRPC `LookupMSISDN` для определения оператора (MCCMNC), 3) запросить `OperatorSupportRepository` для проверки поддержки channel_type оператором, 4) записать результат в Redis
- [X] T044 [US4] Интегрировать reachability check в `internal/services/cascade/application/cascade_service.go` — перед созданием attempt для каждого шага вызывать `ReachabilityService.CheckReachability`; если канал недоступен — создать attempt со статусом `skipped`, сразу перейти к следующему шагу (не публиковать `CascadeAttemptSendCommand`)

**Checkpoint**: US4 функционально — недоступные каналы пропускаются без таймаута

---

## Phase 7: User Story 5 — Биллинг каскадных доставок (Priority: P3)

**Goal**: Клиент тарифицируется только за тарифицируемые попытки; late_duplicate не тарифицируется повторно; биллинг публикуется через Kafka

**Independent Test**: Отправить через каскад flash_call (таймаут) → SMS (success); проверить что `cascade.billing` содержит только один event для SMS attempt; flash_call attempt не тарифицирован

- [X] T045 [US5] Реализовать `internal/services/cascade/application/billing_integration.go` — метод `BillAttempt(ctx, attempt, delivery)`: вызывает gRPC `tarification-service.TarifyMessage` с `channel_type` для получения тарифа, затем gRPC `billing-service.Charge`, обновляет `attempt.cost` и `delivery.total_cost`; пропускает попытки с `status = "skipped"` или `status = "late_duplicate"`; использует `attempt.id` как idempotency_key
- [X] T046 [US5] Добавить consumer для `cascade.billing` топика в `internal/services/cascade/infrastructure/kafka/` — читает `CascadeBillingCommand`, вызывает `BillingIntegration.BillAttempt`; интегрировать в `cmd/services/cascade-service/main.go`
- [X] T047 [US5] Обновить `internal/services/cascade/application/cascade_service.go` — при завершении каскада (delivered или failed) опубликовать `CascadeBillingCommand` для каждой тарифицируемой попытки (billable=true в strategy step); при обнаружении late_duplicate вызвать `HandleLateDuplicate` и НЕ публиковать billing command для дублирующего attempt

**Checkpoint**: US5 функционально — корректный биллинг без двойной тарификации

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Observability, метрики, финальная валидация

- [X] T048 [P] Добавить Prometheus-метрики в `cmd/services/cascade-service/main.go` и сервисный слой: `cascade_deliveries_total{status}` (counter), `cascade_attempts_total{channel_type, status}` (counter), `cascade_delivery_duration_seconds` (histogram), `cascade_active_deliveries` (gauge); endpoint `/metrics`
- [X] T049 [P] Добавить `/health` (liveness) и `/health/ready` (readiness: pgx ping + Redis ping + Kafka producer ping) endpoints в `cmd/services/cascade-service/main.go`
- [X] T050 [P] Добавить структурированное zerolog-логирование во все слои cascade-service: каждая state transition (delivery + attempt), каждый reachability check, каждый billing event — с полями `delivery_id`, `attempt_id`, `channel_type`, `request_id`
- [X] T051 Провести валидацию по `specs/012-multichannel-cascade/quickstart.md` — выполнить пример создания стратегии и отправки каскада через curl, убедиться в корректных ответах

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Нет зависимостей — старт немедленно
- **Foundational (Phase 2)**: Зависит от Phase 1 — БЛОКИРУЕТ все user stories
- **User Stories (Phase 3–7)**: Все зависят от завершения Phase 2
  - US1 (Phase 3): Базовый поток — первый приоритет
  - US2 (Phase 4): Независим от US1, но логично после него (нужны каналы для US1-тестирования)
  - US3 (Phase 5): Зависит от US1 (нужны deliveries для отображения)
  - US4 (Phase 6): Зависит от US1 (интегрируется в cascade_service.go)
  - US5 (Phase 7): Зависит от US1 (интегрируется в cascade_service.go)
- **Polish (Phase 8)**: После желаемых user stories

### User Story Dependencies

- **US1 (P1)**: Старт после Phase 2 — нет зависимостей от других US
- **US2 (P2)**: Старт после Phase 2 — независим от US1 по коду, но для полного теста нужна US1
- **US3 (P2)**: Требует US1 (delivery records), иначе нечего показывать
- **US4 (P3)**: Требует US1 (интеграция в cascade_service.go)
- **US5 (P3)**: Требует US1 (интеграция в cascade_service.go)

### Within Each User Story

- Доменные сущности → репозитории → application services → gRPC handler → gateway handler → frontend
- Файлы без зависимостей (помечены [P]) можно реализовывать параллельно

---

## Parallel Execution Examples

### Phase 2 (Foundational) — можно параллельно:

```
Task T009: Domain entities (domain/*.go)
Task T010: Kafka event structs (infrastructure/kafka/events.go)
Task T012: Kafka producer (infrastructure/kafka/producer.go)
--- после T009, T010 ---
Task T011: Postgres repos (infrastructure/postgres/*.go)
```

### Phase 3 (US1) — частично параллельно:

```
# Параллельно:
Task T013: SMS adapter (channels/sms/adapter.go)
Task T014: Flash Call adapter (channels/flash_call/adapter.go)
# После T013, T014:
Task T015: CascadeService (application/cascade_service.go)
Task T016: Scheduler (application/scheduler.go)
# После T015, T016:
Task T017: Kafka Orchestrator (infrastructure/kafka/orchestrator.go)
Task T018: gRPC handler CreateDelivery
```

### Phase 4 (US2) — частично параллельно:

```
# Параллельно:
Task T024: ChannelService
Task T025: StrategyService
Task T030: Frontend API client (cascade.ts)
Task T031+T032: ChannelsPage + ChannelFormModal
Task T033+T034+T035: StrategiesPage + StrategyFormModal + OCSMatrix
# После T024, T025:
Task T026: gRPC handler admin methods
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Завершить Phase 1: Setup (T001–T004)
2. Завершить Phase 2: Foundational (T005–T012) — КРИТИЧНО
3. Завершить Phase 3: US1 (T013–T023)
4. **СТОП и ВАЛИДАЦИЯ**: `POST /cascade/deliveries` → проверить flash call → fallback SMS → webhook → delivered
5. Деплой/демо при готовности

### Incremental Delivery

1. Setup + Foundational → база готова
2. US1 → каскадная доставка работает (MVP!)
3. US2 → admin UI для управления каналами/стратегиями
4. US3 → история и статистика в клиентском UI
5. US4 → оптимизация через reachability check
6. US5 → корректный биллинг

---

## Task Count Summary

| Phase | US | Tasks | Parallel Tasks |
|-------|----|-------|---------------|
| Phase 1: Setup | — | 4 | T003, T004 |
| Phase 2: Foundational | — | 8 | T009, T010, T012 |
| Phase 3: US1 (P1) | US1 | 11 | T013, T014 |
| Phase 4: US2 (P2) | US2 | 13 | T024, T025, T030–T035 |
| Phase 5: US3 (P2) | US3 | 6 | T040, T041 |
| Phase 6: US4 (P3) | US4 | 2 | — |
| Phase 7: US5 (P3) | US5 | 3 | — |
| Phase 8: Polish | — | 4 | T048, T049, T050 |
| **Total** | | **51** | **~18** |

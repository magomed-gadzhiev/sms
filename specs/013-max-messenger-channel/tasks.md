# Tasks: Max Messenger Channel

**Input**: Design documents from `/specs/013-max-messenger-channel/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Создание пакета max_messenger, миграции, domain-константы

- [x] T001 Add `ChannelMaxMessenger` constant and validation in `internal/services/cascade/domain/channel.go`
- [x] T002 [P] Create database migration `migrations/000071_max_messenger_channel.up.sql` — seed `delivery_channels` row (inactive) and `operator_channel_support` entries for all operators
- [x] T003 [P] Create rollback migration `migrations/000071_max_messenger_channel.down.sql`
- [x] T004 [P] Create Go structs for Max Messenger API payloads (SendRequest, SendResponse, CheckRegistrationRequest, CheckRegistrationResponse, WebhookPayload, ButtonItem) in `internal/services/cascade/channels/max_messenger/types.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Prometheus метрики и базовый скелет адаптера — блокируют все user stories

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T005 Create `MaxMessengerMetrics` struct (WebhooksReceived counter, ReachabilityChecks counter, SendLatency histogram, ActiveAttempts gauge) and `NewMaxMessengerMetrics()` registration in `internal/services/cascade/channels/max_messenger/metrics.go`
- [x] T006 Create skeleton `Adapter` struct with `NewAdapter(httpClient, logger, metrics)`, `Type()` returning `ChannelMaxMessenger`, and stub `Send()` returning `nil` in `internal/services/cascade/channels/max_messenger/adapter.go`
- [x] T007 Register `MaxMessengerAdapter` in cascade-service bootstrap — import package, create adapter instance, add to `channelAdapters` map in `cmd/services/cascade-service/main.go`

**Checkpoint**: Adapter compiled and registered, Max Messenger recognized as valid channel type

---

## Phase 3: User Story 1 — Отправка сообщения через Max Messenger в каскаде (Priority: P1) 🎯 MVP

**Goal**: Система отправляет сообщения через Max Bot API по MSISDN. Поддержка текста, изображений, inline keyboard. Retry при HTTP 429.

**Independent Test**: Создать доставку с одношаговой стратегией (только Max Messenger) и проверить, что HTTP-запрос к провайдеру отправлен корректно с правильным payload.

### Implementation for User Story 1

- [x] T008 [US1] Implement `Send()` method in adapter — extract delivery from context, build `SendRequest` payload (recipient, text, image_url, inline_keyboard, callback_url, external_id), make HTTP POST to `provider_url` with Bearer auth, parse response, record `SendLatency` metric in `internal/services/cascade/channels/max_messenger/adapter.go`
- [x] T009 [US1] Implement exponential retry logic for HTTP 429 in `Send()` — up to 3 retries with backoff 1s→2s→4s, log each retry with zerolog, record `rate_limited` in SendLatency metric; on retry exhaustion return error in `internal/services/cascade/channels/max_messenger/adapter.go`
- [x] T010 [US1] Implement `WithDelivery()` context helper for injecting delivery into context (follow flash_call pattern) in `internal/services/cascade/channels/max_messenger/adapter.go`
- [x] T011 [US1] Add config validation helper `ValidateConfig(cfg map[string]interface{}) error` — check required fields (provider_url, api_key, bot_id, webhook_secret, check_registration_url), validate HTTPS URLs, min 32 chars for webhook_secret in `internal/services/cascade/channels/max_messenger/adapter.go`

**Checkpoint**: Max Messenger adapter sends messages to provider API with retry. Cascade orchestrator can execute Max Messenger steps.

---

## Phase 4: User Story 2 — Проверка доступности получателя в Max Messenger (Priority: P1)

**Goal**: Перед отправкой система проверяет регистрацию MSISDN в Max. Незарегистрированные — skipped. Результат кешируется 1 час. Fail-open при ошибках.

**Independent Test**: Отправить запрос проверки для незарегистрированного номера и убедиться, что шаг получает статус `skipped`.

### Implementation for User Story 2

- [x] T012 [US2] Implement `CheckRegistration(ctx, msisdn, cfg) (bool, error)` — HTTP POST to `check_registration_url`, parse response, return registered status, handle errors with fail-open (return true on error), record ReachabilityChecks metric in `internal/services/cascade/channels/max_messenger/reachability.go`
- [x] T013 [US2] Integrate Max Messenger reachability into `ReachabilityService.CheckReachability()` — for `ChannelMaxMessenger` type, check Redis cache first (`reachability:{msisdn}:max_messenger`, TTL 1h), if miss call `CheckRegistration()`, cache result; keep fail-open on Redis/API errors in `internal/services/cascade/application/reachability_service.go`

**Checkpoint**: Cascade skips Max Messenger step for unregistered recipients. Cache reduces API calls. Fail-open ensures delivery attempts continue on API errors.

---

## Phase 5: User Story 3 — Администрирование канала Max Messenger (Priority: P2)

**Goal**: Администратор может настроить канал Max Messenger (URL, ключи, бот), включить/отключить его, добавить в стратегии каскадной доставки.

**Independent Test**: Через админ-интерфейс создать/отредактировать канал Max Messenger, задать конфигурацию, включить в стратегию.

### Implementation for User Story 3

- [x] T014 [US3] Add `max_messenger` option to channel type dropdown in `portal-frontend/src/pages/channels/ChannelFormModal.tsx`
- [x] T015 [US3] Add config validation for `max_messenger` channel type in admin channel CRUD handler — call `ValidateConfig()` on create/update, return 400 with field-level errors on validation failure in `internal/gateway/portal/handlers/cascade_channels.go` (or equivalent portal handler)

**Checkpoint**: Admin can create, configure, activate/deactivate Max Messenger channel and include it in cascade strategies via existing UI.

---

## Phase 6: User Story 4 — Получение статуса доставки через webhook (Priority: P2)

**Goal**: Принимать webhook от Max API, верифицировать HMAC-SHA256, обновлять статус attempt, обрабатывать late_duplicate.

**Independent Test**: Отправить имитированный webhook-запрос с валидной подписью и проверить, что статус attempt обновился.

### Implementation for User Story 4

- [x] T016 [US4] Implement `WebhookHandler` HTTP handler — parse `WebhookPayload` from body, verify HMAC-SHA256 signature from `X-Max-Signature` header against `webhook_secret` from channel config, return 401 on invalid signature, record WebhooksReceived metric in `internal/services/cascade/channels/max_messenger/webhook_handler.go`
- [x] T017 [US4] Implement status mapping and Kafka event publishing in `WebhookHandler` — map Max status (delivered/read→delivered, error→failed), load attempt from DB, check if delivery is terminal (mark late_duplicate if so), publish `CascadeAttemptResultEvent` to Kafka, return 200 in `internal/services/cascade/channels/max_messenger/webhook_handler.go`
- [x] T018 [US4] Register webhook HTTP route `POST /webhooks/cascade/max_messenger` in cascade-service HTTP server — wire WebhookHandler with channel repository (to load webhook_secret), attempt repository, and Kafka producer in `cmd/services/cascade-service/main.go`

**Checkpoint**: Webhook callbacks from Max API update attempt status. Late duplicates handled correctly. HMAC signature verified.

---

## Phase 7: User Story 5 — Тарификация доставки через Max Messenger (Priority: P3)

**Goal**: Попытки через Max Messenger тарифицируются по billable-флагу стратегии. Skipped и late_duplicate не тарифицируются.

**Independent Test**: Создать доставку с billable Max Messenger шагом и проверить, что после завершения attempt формируется billing event.

### Implementation for User Story 5

- [x] T019 [US5] Verify existing `BillingIntegration.BillAttempt()` handles `channel_type=max_messenger` correctly — confirm no channel-specific logic needed (billing is generic); if Max Messenger requires special tariff plan matching, add it in `internal/services/cascade/application/billing_integration.go`
- [x] T020 [US5] Verify billing exclusions work for Max Messenger — confirm skipped attempts (reachability=false) and late_duplicate attempts do NOT trigger billing events; trace through `publishBillingForDelivery()` in `internal/services/cascade/application/cascade_service.go`

**Checkpoint**: Max Messenger attempts billed correctly. Skipped and late_duplicate excluded from billing.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Конфигурация, метрики, логирование, edge cases

- [x] T021 [P] Add Max Messenger channel configuration section to `configs/config.example.yaml` — document provider_url, api_key, bot_id, webhook_secret, check_registration_url fields with example values
- [x] T022 [P] Add structured zerolog logging throughout adapter — log send attempts (attempt_id, delivery_id, recipient masked), webhook events, reachability checks, retries with request_id propagation in `internal/services/cascade/channels/max_messenger/adapter.go`
- [x] T023 Verify cascade timeout handling for Max Messenger — confirm scheduler correctly finds pending/sent Max Messenger attempts that exceed step timeout and publishes timeout events; no code change expected (generic scheduler)
- [x] T024 Run `quickstart.md` validation — apply migration, deploy cascade-service, configure channel via admin UI, create strategy with Max Messenger step, send test delivery, verify metrics at `/metrics` endpoint

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on T001 (domain constant) — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 — core sending capability
- **US2 (Phase 4)**: Depends on Phase 2 — can run in parallel with US1
- **US3 (Phase 5)**: Depends on Phase 2 — can run in parallel with US1, US2
- **US4 (Phase 6)**: Depends on Phase 2 — can run in parallel with US1, US2, US3
- **US5 (Phase 7)**: Depends on US1 + US4 (needs working send + webhook to verify billing flow)
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: No story dependencies — standalone sending capability
- **US2 (P1)**: No story dependencies — standalone reachability check
- **US3 (P2)**: No story dependencies — standalone admin UI
- **US4 (P2)**: No story dependencies — standalone webhook handling
- **US5 (P3)**: Verification task — requires US1 + US4 for end-to-end billing flow

### Within Each User Story

- Types/structs before service logic
- Service logic before HTTP handlers
- HTTP handlers before route registration

### Parallel Opportunities

- T002, T003, T004 can run in parallel (different files)
- US1, US2, US3, US4 can all start in parallel after Phase 2
- T014, T015 (US3) can run in parallel
- T021, T022 (Polish) can run in parallel

---

## Parallel Example: After Phase 2

```text
# All four user stories can start simultaneously:
Stream A (US1): T008 → T009 → T010 → T011
Stream B (US2): T012 → T013
Stream C (US3): T014 + T015 (parallel)
Stream D (US4): T016 → T017 → T018
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T007)
3. Complete Phase 3: User Story 1 (T008-T011)
4. **STOP and VALIDATE**: Send test message through Max Messenger adapter
5. Deploy if ready — cascade can now use Max Messenger for sending

### Incremental Delivery

1. Setup + Foundational → Max Messenger recognized as channel type
2. Add US1 → Messages sent through Max → Deploy (MVP!)
3. Add US2 → Unregistered recipients skipped → Deploy
4. Add US3 → Admin can configure channel → Deploy
5. Add US4 → Webhook status updates work → Deploy
6. Add US5 → Billing verified → Deploy
7. Polish → Production-ready

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (Send) + US5 (Billing verification)
   - Developer B: US2 (Reachability) + US4 (Webhook)
   - Developer C: US3 (Admin UI)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Паттерн адаптера: следовать `flash_call/adapter.go` (HTTP-based)
- Все monetary values в RUB (Constitution V)
- Webhook secret минимум 32 символа
- Fail-open при недоступности Max API / Redis
- Commit after each task or logical group

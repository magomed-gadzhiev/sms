# Tasks: Number Lookup (HLR/MNP) + Smart Routing

**Input**: Design documents from `/specs/004-hlr-smart-routing/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Proto definitions, database migrations, project structure

- [x] T001 Extend routing proto with HLR types, enums (NumberStatus, NumberType), and messages (NumberLookupRequest/Response, HLRProvider, SmartRouteWeight, LookupLogEntry) in api/proto/routing/routing.proto
- [x] T002 Add new RPC methods to routing proto: NumberLookup, BulkNumberLookup, CreateHLRProvider, UpdateHLRProvider, DeleteHLRProvider, GetHLRProvider, ListHLRProviders, SetSmartRouteWeights, GetSmartRouteWeights, ListSmartRouteWeights, DeleteSmartRouteWeights, GetLookupHistory in api/proto/routing/routing.proto
- [x] T003 Extend RouteMessageResponse in routing proto with fields: hlr_result, hlr_used, routing_score in api/proto/routing/routing.proto
- [x] T004 Regenerate Go code from updated routing proto via protoc in api/proto/routingv1/
- [x] T005 [P] Create migration for hlr_providers table with indexes (idx_hlr_providers_active, idx_hlr_providers_status) in migrations/000022_create_hlr_providers.up.sql and .down.sql
- [x] T006 [P] Create migration for smart_route_weights table with CHECK constraint (cost_weight + quality_weight = 1.00) and UNIQUE (operator_code, country_code) in migrations/000023_create_smart_route_weights.up.sql and .down.sql
- [x] T007 [P] Create migration for lookup_log partitioned table (RANGE by created_at, monthly) with indexes (idx_lookup_log_client, idx_lookup_log_msisdn, idx_lookup_log_request) and initial monthly partitions in migrations/000024_create_lookup_log.up.sql and .down.sql

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Domain entities, adapter interfaces, and infrastructure that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T008 [P] Create HLR domain entities: LookupResult (value object with MSISDN, operator MCC+MNC, status, country, number type, is_ported, original_operator, queried_at), NumberStatus enum, NumberType enum, HLRProviderConfig (entity with adapter_type, config, priority, supported_regions, cost_per_lookup, status, success_rate) in internal/services/routing/domain/hlr.go
- [x] T009 [P] Create HLRProviderAdapter interface (Lookup(ctx, msisdn) → LookupResult, Ping(ctx) → error, Name() string) and base HTTP REST adapter implementation in internal/services/routing/infrastructure/hlr_provider_adapter.go
- [x] T010 [P] Implement HLR cache in Redis: Get(msisdn) → LookupResult, Set(msisdn, result, ttl), Delete(msisdn), key pattern hlr:{msisdn}, JSON serialization, configurable TTL (default 24h) in internal/services/routing/infrastructure/hlr_cache.go
- [x] T011 [P] Implement HLR provider repository: Create, Update, Delete, GetByID, ListActive, GetByPriority (ordered by priority ASC, filtered by supported_regions and active=true) in internal/services/routing/infrastructure/hlr_provider_repo.go
- [x] T012 [P] Implement lookup log repository: Insert, ListByClient (with pagination, date range, msisdn and source filters), CountByClient in internal/services/routing/infrastructure/lookup_log_repo.go

**Checkpoint**: Foundation ready — HLR domain, cache, repositories, and adapter interface are in place

---

## Phase 3: User Story 1+2 — HLR-обогащённая маршрутизация + Кеширование (Priority: P1) 🎯 MVP

**Goal**: Перед отправкой SMS система выполняет синхронный HLR-lookup (с кешированием в Redis TTL 24h) для определения реального оператора и использует результат для маршрутизации. При недоступности HLR — fallback на prefix-routing. Номера со статусом invalid блокируются, absent — отправляются с пометкой.

**Independent Test**: Отправить SMS на перенесённый номер (MNP) — система определяет реального оператора и маршрутизирует через правильного провайдера. Повторная отправка на тот же номер использует кеш без HLR-запроса.

### Implementation for User Story 1+2

- [x] T013 [US1] Implement HLR service (orchestration layer): LookupNumber(ctx, msisdn, forceRefresh, clientID, requestID) → LookupResult — check Redis cache first, on miss query HLR provider by priority with failover, store result in cache, log to lookup_log, return result. Handle timeouts (200ms) and provider failures with fallback. In internal/services/routing/application/hlr_service.go
- [x] T014 [US1] Add HLR provider health monitoring: background goroutine (Start/Stop pattern per constitution) pinging providers every 30s, updating status (healthy ≥95%, degraded 80-95%, unhealthy <80%), updating success_rate in DB in internal/services/routing/application/hlr_service.go
- [x] T015 [US1] Integrate HLR lookup into RouteMessage flow in routing_service.go: before SelectProvider, call HLR service to get real operator, use HLR operator for provider selection instead of prefix-based operator, populate hlr_result/hlr_used/routing_score in RouteMessageResponse. Skip HLR for unsupported number formats (short codes, service numbers). In internal/services/routing/application/routing_service.go
- [x] T016 [US1] Implement number status handling in routing pipeline: block sending and return invalid status (no charge) for invalid numbers, send with absent marker for absent numbers, proceed normally for active/unknown. In internal/services/routing/application/routing_service.go
- [x] T017 [US1] Add Kafka consumer for message.delivery_failed events: on wrong_operator reason, invalidate HLR cache for the failed MSISDN (DEL hlr:{msisdn}). In internal/services/routing/infrastructure/hlr_cache_invalidator.go
- [x] T018 [US1] Add Kafka publisher for lookup.completed events (msisdn, operator, status, cached, latency, source) in internal/services/routing/infrastructure/hlr_event_publisher.go
- [x] T019 [US1] Add Prometheus metrics: hlr_lookup_duration_seconds (histogram), hlr_cache_hits_total / hlr_cache_misses_total (counters), hlr_provider_requests_total (counter with provider+status labels), hlr_provider_health (gauge with provider label) in internal/services/routing/infrastructure/hlr_metrics.go
- [x] T020 [US1] Register HLR service initialization in routing-service main: create HLR cache, provider repo, adapter instances, HLR service, start health monitor goroutine, wire into routing service. In cmd/services/routing-service/main.go
- [x] T021 [US1] Update routing gRPC server to pass HLR-enriched data in RouteMessage response (hlr_result, hlr_used, routing_score fields) in internal/services/routing/grpc/server.go

**Checkpoint**: HLR-enriched routing with caching is fully functional. SMS to ported numbers routes correctly. Cache reduces HLR queries. Fallback works when HLR unavailable.

---

## Phase 4: User Story 3 — API валидации номеров (Priority: P2)

**Goal**: Клиенты используют standalone API (single + bulk до 1000) для проверки номеров без отправки SMS. Тарификация per-number. Rate-limiting per-client.

**Independent Test**: Вызвать POST /api/v1/lookup с номером — получить оператор, статус, страну, тип. Вызвать POST /api/v1/lookup/bulk с 3 номерами — получить результаты для каждого.

### Implementation for User Story 3

- [x] T022 [US3] Implement NumberLookup and BulkNumberLookup gRPC handlers in routing gRPC server: validate E.164 format, call HLR service, enforce max 1000 numbers for bulk, return structured responses with per-number errors for bulk. In internal/services/routing/grpc/server.go
- [x] T023 [US3] Implement GetLookupHistory gRPC handler: query lookup_log_repo with pagination, date range, msisdn and source filters. In internal/services/routing/grpc/server.go
- [x] T024 [P] [US3] Create lookup HTTP handlers in client-gateway: POST /api/v1/lookup (single), POST /api/v1/lookup/bulk (batch), GET /api/v1/lookup/history — call routing gRPC NumberLookup/BulkNumberLookup/GetLookupHistory, map responses to JSON per contract. In internal/gateway/client/handlers/lookup.go
- [x] T025 [US3] Register lookup routes in client-gateway router with API key auth middleware. In internal/gateway/client/router/router.go
- [x] T026 [US3] Add lookup billing integration: before executing lookup, call tarification-service TarifyMessage with type=lookup to get per-number cost, check client balance via billing-service, reject with 402 if insufficient. After successful lookup, charge via billing-service ChargeMessage. For bulk: pre-check balance for full batch, charge per successful lookup. In internal/gateway/client/handlers/lookup.go
- [x] T027 [US3] Add rate-limiting for lookup API: per-client limits using Redis counters (existing rate-limit pattern), configurable per-client. Return 429 with retry_after on exceeded. In internal/gateway/client/handlers/lookup.go
- [x] T028 [US3] Add Prometheus metric: lookup_api_requests_total (counter with type=single|bulk label) in internal/gateway/client/handlers/lookup.go

**Checkpoint**: Standalone lookup API works. Clients can validate numbers, get billed per-number, and view history. Rate limiting protects the system.

---

## Phase 5: User Story 4 — Smart Routing с настраиваемыми весами (Priority: P2)

**Goal**: Провайдер выбирается по взвешенному score (cost_weight * normalized_cost + quality_weight * normalized_delivery_rate). Веса настраиваемы per-operator/per-region через admin API. Недоступные провайдеры исключаются.

**Independent Test**: Настроить два провайдера для одного оператора, задать веса cost=0.7 quality=0.3 — система выбирает более дешёвого. Изменить на cost=0.3 quality=0.7 — система выбирает более качественного.

### Implementation for User Story 4

- [x] T029 [P] [US4] Create SmartRoute domain entity: SmartRouteWeight (operator_code, country_code, cost_weight, quality_weight) with Validate() method (weights sum = 1.0), CalculateScore(cost, deliveryRate, weights) → float64 in internal/services/routing/domain/smart_route.go
- [x] T030 [P] [US4] Implement smart route weights repository: Create/Update (upsert), GetByOperatorAndCountry, ListAll, Delete. Return defaults (0.60/0.40) when no custom weights found. In internal/services/routing/infrastructure/smart_route_repo.go
- [x] T031 [US4] Implement smart routing service: SelectOptimalProvider(operator, country, candidateProviders) — get weights from repo (or defaults), normalize costs and delivery rates across candidates, calculate weighted score per provider, exclude unhealthy providers, return provider with highest score and the score value. In internal/services/routing/application/smart_routing_service.go
- [x] T032 [US4] Integrate smart routing into routing pipeline: replace existing SelectProvider logic (round_robin/least_loaded/cheapest) with smart routing when HLR result is available. Keep existing logic as fallback when HLR not used. In internal/services/routing/application/routing_service.go
- [x] T033 [US4] Implement smart route weight management gRPC handlers: SetSmartRouteWeights, GetSmartRouteWeights, ListSmartRouteWeights, DeleteSmartRouteWeights in internal/services/routing/grpc/server.go
- [x] T034 [P] [US4] Create admin HLR and weights HTTP handlers: POST/GET/PUT/DELETE /admin/v1/hlr/providers (CRUD HLR providers), GET /admin/v1/hlr/providers/{id}/health, POST/GET/DELETE /admin/v1/routing/weights in internal/gateway/admin/handlers/hlr.go
- [x] T035 [US4] Register admin HLR and weights routes in admin-gateway router with bearer auth middleware. In internal/gateway/admin/router/router.go
- [x] T036 [US4] Add Prometheus metric: smart_routing_score (histogram) for distribution of selected provider scores in internal/services/routing/infrastructure/hlr_metrics.go

**Checkpoint**: Smart routing with configurable weights works. Admin can manage HLR providers and tune weights per operator/region.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Portal integration, data lifecycle, operational readiness

- [x] T037 [P] Create portal lookup HTTP handlers: GET /portal/v1/lookup/history (client's lookup history with pagination), GET /portal/v1/lookup/stats (lookup usage summary) in internal/gateway/portal/handlers/lookup.go
- [x] T038 Register portal lookup routes in portal-gateway router with session auth middleware. In internal/gateway/portal/router/router.go
- [x] T039 [P] Add lookup_log partition management: SQL function or cron job to create next month's partition and drop partitions older than 90 days in migrations/000NNN_lookup_log_partition_management.up.sql
- [x] T040 Add HLR configuration to routing-service environment variables: HLR_CACHE_TTL, HLR_PROVIDER_TIMEOUT, HLR_HEALTH_CHECK_INTERVAL, SMART_ROUTE_DEFAULT_COST_WEIGHT, SMART_ROUTE_DEFAULT_QUALITY_WEIGHT with sensible defaults in cmd/services/routing-service/main.go
- [ ] T041 Add routing-service HLR dependencies (Redis, HLR provider connectivity) to Docker Compose health checks in deployments/docker-compose.yml
- [ ] T042 Run quickstart.md validation: verify end-to-end flow (SMS with HLR → correct routing, standalone lookup → billing, admin provider CRUD, cache hit on repeat lookup)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (proto generated, migrations applied) — BLOCKS all user stories
- **US1+US2 (Phase 3)**: Depends on Phase 2 — core MVP
- **US3 (Phase 4)**: Depends on Phase 2 (uses HLR service from Phase 3, but can use it once Phase 2 infra is ready; practically depends on Phase 3 for HLR service)
- **US4 (Phase 5)**: Depends on Phase 2 for infra; integrates into routing pipeline from Phase 3
- **Polish (Phase 6)**: Depends on Phases 3-5

### User Story Dependencies

- **US1+US2 (P1)**: Start after Phase 2 — no dependencies on other stories. This IS the MVP.
- **US3 (P2)**: Depends on US1+US2 (uses HLR service for actual lookups)
- **US4 (P2)**: Can start after Phase 2, but T032 (integration) depends on US1+US2 routing pipeline being in place. T029-T031 (domain + repo + service) can be built in parallel with US1+US2.

### Within Each User Story

- Domain entities before services
- Services before gRPC handlers
- gRPC handlers before HTTP handlers
- Core logic before integrations (billing, rate-limiting, metrics)

### Parallel Opportunities

**Phase 1**: T005, T006, T007 (migrations) can run in parallel
**Phase 2**: T008, T009, T010, T011, T012 (all different files) can run in parallel
**Phase 3**: T017+T018+T019 can run in parallel (Kafka consumer, publisher, metrics — different files)
**Phase 4**: T024 can run in parallel with T022-T023 (HTTP handlers vs gRPC handlers)
**Phase 5**: T029, T030, T034 can run in parallel (domain, repo, admin handlers — different files)
**Phase 6**: T037, T039 can run in parallel

---

## Parallel Example: Phase 2 (Foundational)

```bash
# All foundational tasks target different files — run in parallel:
Task: "Create HLR domain entities in internal/services/routing/domain/hlr.go"
Task: "Create HLR provider adapter in internal/services/routing/infrastructure/hlr_provider_adapter.go"
Task: "Implement HLR cache in internal/services/routing/infrastructure/hlr_cache.go"
Task: "Implement HLR provider repo in internal/services/routing/infrastructure/hlr_provider_repo.go"
Task: "Implement lookup log repo in internal/services/routing/infrastructure/lookup_log_repo.go"
```

## Parallel Example: User Story 4

```bash
# Domain + repo + admin handlers target different files:
Task: "Create SmartRoute domain entity in internal/services/routing/domain/smart_route.go"
Task: "Implement smart route weights repo in internal/services/routing/infrastructure/smart_route_repo.go"
Task: "Create admin HLR handlers in internal/gateway/admin/handlers/hlr.go"
```

---

## Implementation Strategy

### MVP First (User Story 1+2 Only)

1. Complete Phase 1: Setup (proto, migrations)
2. Complete Phase 2: Foundational (domain, cache, repos, adapter)
3. Complete Phase 3: US1+US2 (HLR routing + caching)
4. **STOP and VALIDATE**: Send SMS to ported number — correct routing. Repeat — cache hit. HLR down — prefix fallback works.
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Infrastructure ready
2. US1+US2 → HLR routing works → Deploy (MVP!)
3. US3 → Standalone lookup API available → Deploy
4. US4 → Smart routing with weights → Deploy
5. Polish → Portal, partitions, operational readiness → Deploy

### Parallel Team Strategy

With multiple developers after Phase 2:
- Developer A: US1+US2 (core routing — Phase 3)
- Developer B: US4 domain + repo (T029, T030, T031 — no dependency on Phase 3)
- Developer C: US3 HTTP handlers skeleton (T024 — preparatory)
- After Phase 3 completes: B integrates smart routing (T032), C wires lookup to HLR service (T022-T027)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US1 и US2 объединены в одну фазу — кеширование неотделимо от HLR-lookup
- Proto changes (T001-T003) должны быть в одном коммите перед регенерацией (T004)
- Migrations (T005-T007) нумеруются последовательно от последней существующей миграции
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently

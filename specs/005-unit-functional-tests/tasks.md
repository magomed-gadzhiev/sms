# Tasks: Unit & Functional Tests

**Input**: Design documents from `/specs/005-unit-functional-tests/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: This feature IS test implementation — all tasks are test-related.

**Organization**: Tasks grouped by user story. P1 stories first, then P2.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1-US9 maps to spec.md user stories
- Exact file paths included

---

## Phase 1: Setup

**Purpose**: Verify testify/mock dependency and project readiness

- [X] T001 Verify testify/mock is in go.mod, run `go mod tidy` if needed
- [X] T002 Create mocks directory structure for all services: `internal/services/{auth,messaging,routing,billing,tarification,analytics,client,provider,webhook}/mocks/`

**Checkpoint**: Directory structure ready, dependencies resolved

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Test infrastructure that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Extend `internal/testutil/testdb.go` with BeginTestTx(t, db) function for transactional test isolation with automatic rollback via t.Cleanup()
- [X] T004 Create service fixtures in `internal/testutil/service_fixtures.go`: NewTestUser, NewTestAPIKey, NewTestAccount, NewTestTransaction, NewTestTariffPlan, NewTestTariffPeriod, NewTestTariffTier, NewTestLookupResult, NewTestHLRProvider, NewTestOperator, NewTestSmartRouteWeight, NewTestMetric, NewTestSenderRegistration, NewTestUsageCounter
- [X] T005 [P] Create auth service mocks in `internal/services/auth/mocks/mock_password_hasher.go` and `mock_api_key_generator.go` implementing PasswordHasher and APIKeyGenerator interfaces via testify/mock
- [X] T006 [P] Create messaging service mocks in `internal/services/messaging/mocks/`: mock_message_repository.go, mock_dlr_repository.go, mock_event_publisher.go implementing MessageRepository, DLRRepository, EventPublisher interfaces
- [X] T007 [P] Create routing service mocks in `internal/services/routing/mocks/`: mock_route_repository.go, mock_hlr_cache.go, mock_hlr_provider_adapter.go, mock_hlr_provider_repository.go, mock_lookup_log_repository.go, mock_adapter_factory.go, mock_provider_repository.go, mock_operator_repository.go, mock_operator_prefix_repository.go, mock_country_repository.go, mock_smart_route_weight_repository.go, mock_event_publisher.go, mock_provider_selector.go
- [X] T008 [P] Create billing service mocks in `internal/services/billing/mocks/`: mock_account_repository.go, mock_transaction_repository.go, mock_transfer_repository.go, mock_pricing_rule_repository.go, mock_event_publisher.go
- [X] T009 [P] Create tarification service mocks in `internal/services/tarification/mocks/`: mock_tariff_plan_repository.go, mock_tariff_period_repository.go, mock_tariff_tier_repository.go, mock_usage_counter_repository.go, mock_sender_registration_repository.go, mock_tarification_log_repository.go, mock_prepaid_fee_repository.go, mock_pricing_period_repository.go, mock_event_publisher.go, mock_billing_strategy.go
- [X] T010 [P] Create analytics service mocks in `internal/services/analytics/mocks/`: mock_metric_repository.go, mock_report_repository.go, mock_event_consumer.go
- [X] T011 [P] Create client/provider/webhook mocks: `internal/services/client/mocks/mock_client_repository.go`, `mock_config_repository.go`; `internal/services/provider/mocks/mock_provider_repository.go`; `internal/services/webhook/mocks/mock_cache_invalidator.go`
- [X] T012 Verify all mocks compile: run `go build ./internal/services/*/mocks/...`

**Checkpoint**: All 43 mocks compile, fixtures ready, testdb transaction isolation works

---

## Phase 3: User Story 1 — Юнит-тесты Application-слоя (Priority: P1) MVP

**Goal**: Юнит-тесты для application-сервисов всех 7 сервисов с Happy Path и Error Cases

**Independent Test**: `go test ./internal/services/*/application/...` — все проходят без внешних зависимостей

### Implementation

- [X] T013 [P] [US1] Create auth application tests in `internal/services/auth/application/auth_service_test.go`: TestAuthService/AuthenticateByCredentials (valid creds → user+tokens, invalid password → ErrInvalidCredentials, user not found → ErrNotFound), TestAuthService/AuthenticateByAPIKey (valid key → user, expired key → error, revoked key → error), TestAuthService/CreateAPIKey, TestAuthService/RevokeAPIKey
- [X] T014 [P] [US1] Create auth token service tests in `internal/services/auth/application/token_service_test.go`: TestTokenService/GenerateToken (valid claims → JWT), TestTokenService/ValidateToken (valid → user, expired → error, malformed → error)
- [X] T015 [P] [US1] Create auth TOTP service tests in `internal/services/auth/application/totp_service_test.go`: TestTOTPService/SetupTOTP, TestTOTPService/VerifyTOTP (valid code → success, invalid code → error), TestTOTPService/DisableTOTP
- [X] T016 [P] [US1] Create auth password reset tests in `internal/services/auth/application/password_reset_service_test.go`: TestPasswordResetService/RequestReset, TestPasswordResetService/ResetPassword (valid token → success, expired token → error)
- [X] T017 [P] [US1] Create messaging application tests in `internal/services/messaging/application/message_service_test.go`: TestMessageService/SendMessage (valid → PENDING + event published, empty text → validation error, empty destination → error, repository timeout → error without state change), TestMessageService/GetMessageStatus, TestMessageService/CancelMessage (pending → canceled, already delivered → error), TestMessageService/GetMessages
- [X] T018 [P] [US1] Create messaging DLR service tests in `internal/services/messaging/application/dlr_service_test.go`: TestDLRService/ProcessDLR (DELIVRD → status update, UNDELIVERABLE → status update, duplicate DLR → idempotent)
- [X] T019 [P] [US1] Create messaging scheduler tests in `internal/services/messaging/application/scheduler_test.go`: TestScheduler/ProcessScheduledMessages, TestScheduler/ProcessRetries (retry count exceeded → failed)
- [X] T020 [P] [US1] Create routing HLR service tests in `internal/services/routing/application/hlr_service_test.go`: TestHLRService/LookupNumber (cache hit → return cached, cache miss → call provider + cache result, invalid E.164 → validation error, short code → skip lookup), TestHLRService/LookupNumber/failover (provider 1 fails → provider 2 used, all providers fail → error, all providers TTL expired → graceful error), TestHLRService/ValidateMSISDN
- [X] T021 [P] [US1] Create routing service tests in `internal/services/routing/application/routing_service_test.go`: TestRoutingService/RouteMessage (matching route found → provider selected, no route → error), TestRoutingService/GetActiveRoutes
- [X] T022 [P] [US1] Create smart routing tests in `internal/services/routing/application/smart_routing_service_test.go`: TestSmartRoutingService/GetWeightedRoutes, TestSmartRoutingService/UpdateWeights
- [X] T023 [P] [US1] Create operator resolver tests in `internal/services/routing/application/operator_resolver_test.go`: TestOperatorResolver/ResolveByPrefix (known prefix → operator, unknown prefix → nil)
- [X] T024 [P] [US1] Create billing service tests in `internal/services/billing/application/billing_service_test.go`: TestBillingService/ChargeAccount (sufficient balance → charge + transaction, insufficient → InsufficientFunds), TestBillingService/GetBalance, TestBillingService/RefundTransaction (valid → refund + balance restored, already refunded → error), TestBillingService/TransferBalance
- [X] T025 [P] [US1] Create pricing service tests in `internal/services/billing/application/pricing_service_test.go`: TestPricingService/GetPrice (matching rule → price, no rule → default, client-specific overrides global)
- [X] T026 [P] [US1] Create tarification service tests in `internal/services/tarification/application/tarification_service_test.go`: TestTarificationService/CalculatePrice (fixed strategy, threshold strategy with tier crossing, prepaid strategy), TestTarificationService/CalculatePrice/idempotency (duplicate messageID → same result)
- [X] T027 [P] [US1] Create strategy tests in `internal/services/tarification/application/strategy_fixed_test.go`: TestFixedStrategy/Calculate (single tier → fixed price)
- [X] T028 [P] [US1] Create threshold strategy tests in `internal/services/tarification/application/strategy_threshold_test.go`: TestThresholdStrategy/Calculate (below threshold → tier 1 price, above threshold → tier 2 price, crossing threshold → correct transition)
- [X] T029 [P] [US1] Create prepaid strategy tests in `internal/services/tarification/application/strategy_prepaid_test.go`: TestPrepaidStrategy/Calculate
- [X] T030 [P] [US1] Create saga tests in `internal/services/tarification/application/saga_test.go`: TestSagaOrchestrator/Execute (success → all steps completed, step fails → compensation triggered)
- [X] T030a [P] [US1] Create tariff plan service tests in `internal/services/tarification/application/tariff_plan_service_test.go`: TestTariffPlanService/CreatePlan, TestTariffPlanService/GetPlan, TestTariffPlanService/UpdatePlan, TestTariffPlanService/DeletePlan (NOTE: file already exists in working tree — review and align with Describe/Context/It structure)
- [X] T030b [P] [US1] Create sender service tests in `internal/services/tarification/application/sender_service_test.go`: TestSenderService/Register, TestSenderService/Verify (NOTE: file already exists — review and align)
- [X] T028a [P] [US1] Create threshold recalculation tests in `internal/services/tarification/application/strategy_threshold_recalc_test.go`: TestThresholdRecalc/Recalculate (NOTE: file already exists — review and align)
- [X] T031 [P] [US1] Create analytics service tests in `internal/services/analytics/application/analytics_service_test.go`: TestAnalyticsService/RecordMetric, TestAnalyticsService/GetStatistics
- [X] T032 [P] [US1] Create aggregation service tests in `internal/services/analytics/application/aggregation_service_test.go`: TestAggregationService/Aggregate
- [X] T033 [P] [US1] Create report service tests in `internal/services/analytics/application/report_service_test.go`: TestReportService/GenerateReport, TestReportService/GetReport
- [X] T034 [P] [US1] Create client service tests in `internal/services/client/application/client_service_test.go`: TestClientService/CreateClient, TestClientService/GetClient, TestClientService/UpdateClient
- [X] T034a [P] [US1] Create sub-account service tests in `internal/services/client/application/sub_account_service_test.go`: TestSubAccountService/CreateSubAccount, TestSubAccountService/ListSubAccounts, TestSubAccountService/DeleteSubAccount (active sub-account → deleted, non-existent → NotFound)
- [X] T035 [US1] Run `go test -cover ./internal/services/*/application/...` and verify >= 70% coverage for each service

**Checkpoint**: All application-layer unit tests pass, coverage >= 70%

---

## Phase 4: User Story 2 — Юнит-тесты Domain-слоя (Priority: P1)

**Goal**: Тесты для доменных сущностей: валидация, переходы состояний, бизнес-свойства

**Independent Test**: `go test ./internal/services/*/domain/...` — чистые функции, нет зависимостей

### Implementation

- [X] T036 [P] [US2] Create message domain tests in `internal/services/messaging/domain/message_test.go`: TestMessage/MarkAsQueued (PENDING→QUEUED), TestMessage/MarkAsSent (QUEUED→SENT), TestMessage/MarkAsDelivered (SENT→DELIVERED), TestMessage/MarkAsFailed (PENDING→FAILED, DELIVERED→FAILED returns error), TestMessage/MarkAsCanceled (PENDING→CANCELED, SENT→error)
- [X] T037 [P] [US2] Create message validation tests in `internal/services/messaging/domain/validation_test.go`: TestMessageValidator/Validate (valid message → nil, empty text → error, empty destination → error, invalid E.164 → error)
- [X] T038 [P] [US2] Create user domain tests in `internal/services/auth/domain/user_test.go`: TestUser/HasPermission (matching permission → true, missing → false), TestUser/HasAnyPermission, TestUser/IsActive
- [X] T039 [P] [US2] Create API key domain tests in `internal/services/auth/domain/api_key_test.go`: TestAPIKey/IsExpired, TestAPIKey/HasScope, TestAPIKey/IsActive
- [X] T040 [P] [US2] Create role domain tests in `internal/services/auth/domain/role_test.go`: TestRole/Permissions
- [X] T041 [P] [US2] Create HLR domain tests in `internal/services/routing/domain/hlr_test.go`: TestLookupResult/IsDeliverable (active→true, absent→false), TestLookupResult/IsInvalid (invalid→true, active→false), TestHLRProvider/IsHealthy, TestHLRProvider/StatusTransitions
- [X] T042 [P] [US2] Create route domain tests in `internal/services/routing/domain/route_test.go`: TestRoute/MatchesDestination (prefix match, exact match, regex match, no match)
- [X] T043 [P] [US2] Create account domain tests in `internal/services/billing/domain/account_test.go`: TestAccount/HasSufficientBalance, TestAccount/Debit, TestAccount/Credit
- [X] T044 [P] [US2] Create transaction domain tests in `internal/services/billing/domain/transaction_test.go`: TestTransaction/IsRefundable
- [X] T045 [P] [US2] Create tariff plan domain tests in `internal/services/tarification/domain/tariff_plan_test.go`: TestTariffPlan/Validate (valid plan → nil, overlapping tiers → error), TestTariffPlan/IsActive, TestTariffPlan/StrategyType
- [X] T046 [P] [US2] Create usage counter domain tests in `internal/services/tarification/domain/usage_counter_test.go`: TestUsageCounter/Increment, TestUsageCounter/GetCurrentTier
- [X] T045a [P] [US2] Create sender registration domain tests in `internal/services/tarification/domain/sender_registration_test.go`: TestSenderRegistration/Validate, TestSenderRegistration/StatusTransitions (NOTE: file already exists — review and align)
- [X] T039a [P] [US2] Create permission domain tests in `internal/services/auth/domain/permission_test.go`: TestPermission/Matches, TestPermission/IsWildcard (NOTE: file already exists — review and align)
- [X] T042a [P] [US1] Create selection strategy tests in `internal/services/routing/application/selection_strategy_test.go`: TestSelectionStrategy/SelectByWeight, TestSelectionStrategy/SelectByPriority (NOTE: file already exists — review and align)
- [X] T046a [P] [US2] Create analytics domain tests (if domain layer exists) in `internal/services/analytics/domain/`: metric validation, report status transitions. Skip if analytics has no domain layer (pure application logic only) — document decision in checkpoint

**Checkpoint**: All domain unit tests pass

---

## Phase 5: User Story 6 — Функциональные тесты: SMS-цепочка (Priority: P1)

**Goal**: Полная цепочка SMS от создания до DLR с тестовой БД

**Independent Test**: `go test ./test/functional/ -run TestMessagingChain` с тестовой PostgreSQL

### Implementation

- [X] T047 [US6] Create functional test helpers in `test/functional/helpers_test.go`: add `//go:build functional` build tag to ALL functional test files, setupTestDB, beginTx with rollback, createTestRepositories (messaging repos with real pgx), createMockEventPublisher
- [X] T048 [US6] Create SMS chain functional tests in `test/functional/messaging_test.go`: TestMessagingChain/SendMessage (create message → verify DB row with PENDING status → verify event published), TestMessagingChain/ProcessDLR (insert SENT message → process DELIVRD DLR → verify status=DELIVERED + dlr_receipts row), TestMessagingChain/CancelMessage (insert PENDING message → cancel → verify CANCELED → second cancel returns error)

**Checkpoint**: SMS functional tests pass with test DB

---

## Phase 6: User Story 7 — Функциональные тесты: Биллинг и тарификация (Priority: P1)

**Goal**: Цепочка тарификация → списание → рефанд с тестовой БД

**Independent Test**: `go test ./test/functional/ -run TestBillingChain` с тестовой PostgreSQL

### Implementation

- [X] T049 [US7] Create billing chain functional tests in `test/functional/billing_test.go`: TestBillingChain/ChargeAccount (insert account balance=100.00 + tariff fixed=0.50 → charge → verify balance=99.50 + transaction record), TestBillingChain/InsufficientFunds (balance=0.10, price=0.50 → ChargeAccount → InsufficientFunds error + balance unchanged), TestBillingChain/ThresholdPricing (create threshold plan with tiers → CalculatePrice at volume 1500 → verify tier 2 price), TestBillingChain/RefundTransaction (charge → refund → verify balance restored + refund transaction), TestBillingChain/ConcurrentCharge (launch 10 goroutines charging same account → verify final balance is atomic, no double-charge)

**Checkpoint**: Billing functional tests pass with test DB

---

## Phase 7: User Story 3 — Юнит-тесты Gateway-хендлеров (Priority: P2)

**Goal**: Тесты HTTP-хендлеров для admin, client, portal gateway

**Independent Test**: `go test ./internal/gateway/*/handlers/...` с httptest и mock gRPC clients

### Implementation

- [X] T050 [P] [US3] Create client gateway SMS handler tests in `internal/gateway/client/handlers/sms_test.go`: TestSMSHandler/Send (valid JSON → 200 + messageID, missing auth → 401, invalid body → 400 with error structure, empty destination → 400), TestSMSHandler/GetStatus (valid ID → 200, not found → 404)
- [X] T051 [P] [US3] Create client gateway account handler tests in `internal/gateway/client/handlers/account_test.go`: TestAccountHandler/GetBalance (valid → 200 + balance, unauthorized → 401)
- [X] T052 [P] [US3] Create client gateway lookup handler tests in `internal/gateway/client/handlers/lookup_test.go`: TestLookupHandler/LookupNumber (valid MSISDN → 200 + result, invalid MSISDN → 400)
- [X] T053 [P] [US3] Create portal gateway auth handler tests in `internal/gateway/portal/handlers/auth_test.go`: TestAuthHandler/Login (valid creds → 200 + session, invalid → 401, TOTP required → 200 + requires_2fa), TestAuthHandler/Logout, TestAuthHandler/RefreshSession
- [X] T054 [P] [US3] Create portal gateway dashboard tests in `internal/gateway/portal/handlers/dashboard_test.go`: TestDashboardHandler/GetDashboard
- [X] T055 [P] [US3] Create portal gateway API keys handler tests in `internal/gateway/portal/handlers/api_keys_test.go`: TestAPIKeysHandler/Create, TestAPIKeysHandler/List, TestAPIKeysHandler/Revoke
- [X] T056 [P] [US3] Create admin gateway providers handler tests in `internal/gateway/admin/handlers/providers_test.go`: TestProvidersHandler/Create (valid → 201), TestProvidersHandler/Delete (valid UUID → 200, invalid UUID → 400), TestProvidersHandler/List
- [X] T057 [P] [US3] Create admin gateway routing handler tests in `internal/gateway/admin/handlers/routing_test.go`: TestRoutingHandler/CreateRoute, TestRoutingHandler/UpdateRoute, TestRoutingHandler/DeleteRoute
- [X] T058 [P] [US3] Create admin gateway billing handler tests in `internal/gateway/admin/handlers/billing_test.go`: TestBillingHandler/GetBalance, TestBillingHandler/GetTransactions
- [X] T059 [P] [US3] Create admin gateway tarification handler tests in `internal/gateway/admin/handlers/tarification_test.go`: TestTarificationHandler/CreatePlan, TestTarificationHandler/ListPlans
- [X] T060 [P] [US3] Create admin gateway HLR handler tests in `internal/gateway/admin/handlers/hlr_test.go`: TestHLRHandler/ListProviders, TestHLRHandler/GetLookupLog

**Checkpoint**: All gateway handler tests pass

---

## Phase 8: User Story 4 — Юнит-тесты Middleware (Priority: P2)

**Goal**: Тесты middleware для всех трёх gateway

**Independent Test**: `go test ./internal/gateway/*/middleware/...`

### Implementation

- [X] T061 [P] [US4] Create client gateway auth middleware tests in `internal/gateway/client/middleware/auth_test.go`: TestAuthMiddleware/ValidAPIKey (valid → next handler + context), TestAuthMiddleware/MissingHeader (→ 401), TestAuthMiddleware/InvalidKey (→ 401), TestAuthMiddleware/ExpiredKey (→ 401)
- [X] T062 [P] [US4] Create portal gateway session auth tests in `internal/gateway/portal/middleware/session_auth_test.go`: TestSessionAuth/ValidSession (→ next handler), TestSessionAuth/ExpiredSession (→ 401), TestSessionAuth/MissingCookie (→ 401)
- [X] T063 [P] [US4] Create portal gateway CSRF tests in `internal/gateway/portal/middleware/csrf_test.go`: TestCSRF/ValidToken (POST + token → next), TestCSRF/MissingToken (POST → 403), TestCSRF/GET_NoCsrfRequired (GET → next)
- [X] T064 [P] [US4] Create portal gateway rate limit tests in `internal/gateway/portal/middleware/rate_limit_test.go`: TestRateLimit/UnderLimit (→ next), TestRateLimit/OverLimit (→ 429)
- [X] T065 [P] [US4] Create admin gateway auth middleware tests in `internal/gateway/admin/middleware/auth_test.go`: TestAdminAuth/ValidJWT (→ next + user context), TestAdminAuth/ExpiredJWT (→ 401), TestAdminAuth/InvalidJWT (→ 401)
- [X] T066 [P] [US4] Create shared recovery middleware tests in `internal/gateway/portal/middleware/recovery_test.go`: TestRecovery/PanicHandled (handler panics → 500 returned, no propagation)
- [X] T067 [P] [US4] Create shared CORS middleware tests in `internal/gateway/portal/middleware/cors_test.go`: TestCORS/AllowedOrigin, TestCORS/DisallowedOrigin

**Checkpoint**: All middleware tests pass

---

## Phase 9: User Story 5 — Юнит-тесты gRPC-серверов (Priority: P2)

**Goal**: Тесты gRPC-серверов с bufconn и маппингом ошибок → gRPC codes

**Independent Test**: `go test ./internal/services/*/grpc/...`

### Implementation

- [X] T068 [P] [US5] Create messaging gRPC server tests in `internal/services/messaging/grpc/server_test.go`: TestMessagingServer/SendMessage (valid → OK + messageID, validation error → InvalidArgument), TestMessagingServer/GetMessage (found → OK, not found → NotFound), TestMessagingServer/CancelMessage (success → OK, not found → NotFound)
- [X] T069 [P] [US5] Create auth gRPC server tests in `internal/services/auth/grpc/server_test.go`: TestAuthServer/Authenticate (valid → OK + tokens, invalid password → Unauthenticated, user not found → NotFound), TestAuthServer/ValidateToken (valid → OK + user, invalid → Unauthenticated), TestAuthServer/CreateAPIKey (→ OK + key)
- [X] T070 [P] [US5] Create billing gRPC server tests in `internal/services/billing/grpc/server_test.go`: TestBillingServer/GetBalance (→ OK + balance), TestBillingServer/ChargeAccount (sufficient → OK, insufficient → FailedPrecondition), TestBillingServer/RefundTransaction (→ OK)
- [X] T071 [P] [US5] Create routing gRPC server tests in `internal/services/routing/grpc/server_test.go`: TestRoutingServer/RouteMessage (→ OK + provider), TestRoutingServer/LookupNumber (valid → OK + result, invalid MSISDN → InvalidArgument)
- [X] T072 [P] [US5] Create tarification gRPC server tests in `internal/services/tarification/grpc/server_test.go`: TestTarificationServer/CalculatePrice (→ OK + price), TestTarificationServer/CreateTariffPlan (valid → OK, invalid → InvalidArgument)
- [X] T073 [P] [US5] Create analytics gRPC server tests in `internal/services/analytics/grpc/server_test.go`: TestAnalyticsServer/GetMetrics, TestAnalyticsServer/GetStatistics, TestAnalyticsServer/GenerateReport
- [X] T074 [P] [US5] Create client gRPC server tests in `internal/services/client/grpc/server_test.go`: TestClientServer/GetClient (found → OK, not found → NotFound), TestClientServer/CreateClient

**Checkpoint**: All gRPC server tests pass with correct error code mapping

---

## Phase 10: User Story 8 — Функциональные тесты: Auth (Priority: P2)

**Goal**: Полный цикл аутентификации с тестовой БД и Redis

**Independent Test**: `go test ./test/functional/ -run TestAuthChain` с тестовой PostgreSQL + Redis

### Implementation

- [X] T075 [US8] Create auth chain functional tests in `test/functional/auth_test.go`: TestAuthChain/LoginWithCredentials (insert user → authenticate → verify tokens returned + refresh token in DB), TestAuthChain/RefreshToken (authenticate → refresh → new access token + old refresh invalidated), TestAuthChain/TOTPFlow (insert user + setup TOTP → authenticate without code → requires_2fa → verify with code → success), TestAuthChain/APIKeyAuth (create API key with scopes → authenticate by key → verify permissions match scopes), TestAuthChain/IPWhitelist (create key with allowed_ips → request from wrong IP → Unauthorized)

**Checkpoint**: Auth functional tests pass

---

## Phase 11: User Story 9 — Функциональные тесты: HLR и маршрутизация (Priority: P2)

**Goal**: HLR lookup + кеш + failover + smart routing с тестовой БД

**Independent Test**: `go test ./test/functional/ -run TestHLRRoutingChain` с тестовой PostgreSQL

### Implementation

- [X] T076 [US9] Create HLR routing chain functional tests in `test/functional/hlr_routing_test.go`: TestHLRRoutingChain/LookupWithCache (insert HLR provider + mock adapter → lookup → verify result + cache entry + lookup_log row), TestHLRRoutingChain/CacheHit (pre-populate cache → lookup without forceRefresh → verify cached=true, adapter not called), TestHLRRoutingChain/ForceRefresh (pre-populate cache → lookup with forceRefresh=true → verify adapter called), TestHLRRoutingChain/ProviderFailover (provider 1 fails → provider 2 used → verify fallback + provider 1 status updated), TestHLRRoutingChain/RouteSelection (insert routes with weights → RouteMessage → verify weighted selection)

**Checkpoint**: HLR + routing functional tests pass

---

## Phase 12: Polish — Рефакторинг существующих тестов

**Purpose**: Приведение 28 существующих тест-файлов к единообразной Describe/Context/It структуре

- [X] T077 [P] Refactor `internal/router/router_test.go` to Describe/Context/It structure via t.Run (group by methods: RouteMessage, GetActiveRoutes)
- [X] T078 [P] Refactor `internal/router/retry_test.go` to Describe/Context/It structure via t.Run
- [X] T079 [P] Refactor `internal/api/http/handlers_test.go` to Describe/Context/It structure via t.Run (group by endpoints)
- [X] T080 [P] Refactor `internal/api/grpc/server_test.go` to Describe/Context/It structure via t.Run
- [X] T081 [P] Refactor `internal/api/middleware/auth_test.go` to Describe/Context/It structure via t.Run
- [X] T082 [P] Refactor `internal/api/middleware/cors_test.go` to Describe/Context/It structure via t.Run
- [X] T083 [P] Refactor `internal/api/middleware/logging_test.go` to Describe/Context/It structure via t.Run
- [X] T084 [P] Refactor `internal/api/middleware/ratelimit_test.go` to Describe/Context/It structure via t.Run
- [X] T085 [P] Refactor `internal/api/middleware/recovery_test.go` to Describe/Context/It structure via t.Run
- [X] T086 [P] Refactor `internal/shared/errors_test.go` to Describe/Context/It structure via t.Run
- [X] T087 [P] Refactor `internal/config/config_test.go` to Describe/Context/It structure via t.Run
- [X] T088 [P] Refactor `internal/monitoring/metrics_test.go` to Describe/Context/It structure via t.Run
- [X] T089 [P] Refactor `internal/queue/message_test.go` to Describe/Context/It structure via t.Run
- [X] T090 [P] Refactor `internal/smpp/protocol/decoder_test.go` to Describe/Context/It structure via t.Run
- [X] T091 [P] Refactor `internal/smpp/protocol/encoder_test.go` to Describe/Context/It structure via t.Run
- [X] T092 [P] Refactor `internal/smpp/protocol/validator_test.go` to Describe/Context/It structure via t.Run
- [X] T093 [P] Refactor `internal/smsc/pool_test.go` to Describe/Context/It structure via t.Run
- [X] T094 [P] Refactor `test/integration/grpc_services_test.go` to Describe/Context/It structure via t.Run
- [X] T095 [P] Refactor `test/integration/kafka_test.go` to Describe/Context/It structure via t.Run
- [X] T096 [P] Refactor `test/integration/storage_test.go` to Describe/Context/It structure via t.Run
- [X] T097 [P] Refactor `test/integration/service_communication_test.go` to Describe/Context/It structure via t.Run
- [X] T098 [P] Refactor `test/integration/directus_test.go` to Describe/Context/It structure via t.Run
- [X] T099 [P] Refactor `test/load/api_load_test.go` to Describe/Context/It structure via t.Run
- [X] T100 [P] Refactor `test/load/high_performance_test.go` to Describe/Context/It structure via t.Run
- [X] T101 Run full test suite: `go test ./...` (unit only) and `go test -tags=functional ./test/functional/...` (functional with test DB) — verify all pass
- [X] T102 Run coverage report: `go test -cover ./internal/services/*/application/...` and verify >= 70% per service

**Checkpoint**: All 100+ tests pass, coverage targets met, uniform structure

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user stories
- **Phase 3-6 (P1 stories)**: All depend on Phase 2 completion
  - US1 and US2 can run in parallel (different files)
  - US6 and US7 can run in parallel (different test files)
- **Phase 7-11 (P2 stories)**: Depend on Phase 2 completion
  - US3, US4, US5 can all run in parallel
  - US8 and US9 can run in parallel
- **Phase 12 (Polish)**: Can start after Phase 2, independent of other stories

### User Story Dependencies

- **US1 (Application tests)**: Phase 2 → T013-T035 + T034a, T030a/b, T028a, T042a (all parallelizable within phase)
- **US2 (Domain tests)**: Phase 2 → T036-T046 + T045a, T039a, T046a (all parallelizable, no mock dependency)
- **US3 (Gateway tests)**: Phase 2 → T050-T060 (all parallelizable)
- **US4 (Middleware tests)**: Phase 2 → T061-T067 (all parallelizable)
- **US5 (gRPC tests)**: Phase 2 → T068-T074 (all parallelizable)
- **US6 (Functional SMS)**: Phase 2 + T047 → T048
- **US7 (Functional Billing)**: Phase 2 + T047 → T049
- **US8 (Functional Auth)**: Phase 2 + T047 → T075
- **US9 (Functional HLR)**: Phase 2 + T047 → T076

### Parallel Opportunities

```text
After Phase 2 completes, launch in parallel:
├── US1: T013-T035 (23 tasks, all [P])
├── US2: T036-T046 (11 tasks, all [P])
├── US6: T047-T048 (sequential within)
├── US7: T049 (1 task, depends on T047)
├── US3: T050-T060 (11 tasks, all [P])
├── US4: T061-T067 (7 tasks, all [P])
├── US5: T068-T074 (7 tasks, all [P])
├── US8: T075 (1 task, depends on T047)
├── US9: T076 (1 task, depends on T047)
└── Polish: T077-T100 (24 tasks, all [P])
```

---

## Implementation Strategy

### MVP First (P1 Stories Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (mocks + fixtures + testdb)
3. Complete Phase 3: US1 — Application tests (critical business logic)
4. Complete Phase 4: US2 — Domain tests (validation, state transitions)
5. **STOP and VALIDATE**: `go test ./internal/services/*/application/... ./internal/services/*/domain/...`
6. Complete Phase 5-6: US6+US7 — Functional tests for SMS + Billing

### Incremental Delivery

1. Setup + Foundational → Mock infrastructure ready
2. US1 + US2 → Unit test coverage for core logic (MVP)
3. US6 + US7 → Functional tests for critical business chains
4. US3 + US4 + US5 → Gateway + middleware + gRPC test coverage
5. US8 + US9 → Auth + HLR functional chains
6. Polish → Refactor existing tests for uniformity

---

## Notes

- [P] tasks = different files, no shared state
- All mocks use testify/mock (AssertExpectations pattern)
- Functional tests require TEST_DB_* and TEST_REDIS_* env vars
- Existing function-field mocks in testutil/mocks.go are NOT modified
- Each test file uses Describe/Context/It via t.Run nesting
- Coverage target: >= 70% for application layer per service

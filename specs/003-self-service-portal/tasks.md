# Tasks: Multi-tenant Self-Service Portal + Sub-accounts

**Input**: Design documents from `/specs/003-self-service-portal/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/portal-api.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization, directory structure, dependencies

- [x] T001 Create portal-gateway directory structure: `cmd/portal-gateway/`, `internal/gateway/portal/{handlers,middleware,router}/`
- [x] T002 [P] Initialize React frontend project with Vite + TypeScript in `portal-frontend/` (package.json, vite.config.ts, tsconfig.json)
- [x] T003 [P] Add Go dependencies: `pquerna/otp` (TOTP) to go.mod
- [x] T004 [P] Create shared audit package structure: `internal/shared/audit/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T005 Create migration `migrations/000014_add_totp_tables.up.sql` and `.down.sql`: add `totp_secret_encrypted`, `totp_enabled`, `totp_verified_at` columns to `users`, create `totp_recovery_codes` table
- [x] T006 [P] Create migration `migrations/000015_add_password_reset_tokens.up.sql` and `.down.sql`: create `password_reset_tokens` table with token_hash, expires_at, used
- [x] T007 [P] Create migration `migrations/000016_add_sub_accounts.up.sql` and `.down.sql`: add `parent_client_id`, `is_reseller`, `max_sub_accounts` to `clients`, add constraints and indexes
- [x] T008 [P] Create migration `migrations/000017_create_audit_log.up.sql` and `.down.sql`: create `audit_log` table with monthly partitioning, indexes on tenant_id+created_at, action, user_id
- [x] T009 [P] Create migration `migrations/000018_add_sessions_table.up.sql` and `.down.sql`: create `sessions` table for session tracking
- [x] T010 [P] Create migration `migrations/000019_add_api_key_ip_whitelist.up.sql` and `.down.sql`: add `allowed_ips` text[] column to `api_keys`
- [x] T011 [P] Create migration `migrations/000020_create_balance_transfers.up.sql` and `.down.sql`: create `balance_transfers` table with indexes
- [x] T012 Extend auth proto `api/proto/auth/auth.proto`: add TOTP methods (SetupTOTP, VerifyTOTP, DisableTOTP), PasswordReset methods (RequestPasswordReset, ResetPassword), LoginWithSession, ValidateSession, Logout, update APIKey messages for allowed_ips
- [x] T013 [P] Extend client proto `api/proto/client/client.proto`: add sub-account methods (CreateSubAccount, ListSubAccounts, GetSubAccount, DeleteSubAccount, UpdateSubAccountLimits), add reseller fields to Client message
- [x] T014 [P] Extend billing proto `api/proto/billing/billing.proto`: add TransferBalance method, TransferRequest/Response messages
- [x] T015 [P] Create audit proto `api/proto/audit/audit.proto`: define AuditService with QueryAuditLog method, AuditLogEntry message, filter/pagination parameters
- [x] T016 Regenerate all proto Go code: run protoc for auth, client, billing, audit proto files
- [x] T017 Implement audit event model and Kafka publisher in `internal/shared/audit/event.go` and `internal/shared/audit/publisher.go`
- [x] T018 Implement portal-gateway entry point `cmd/portal-gateway/main.go`: init logger, load config, create gRPC clients, handlers, middleware, router, HTTP server, graceful shutdown (follow admin-gateway pattern)
- [x] T019 [P] Implement portal gRPC clients in `internal/gateway/portal/clients.go`: AuthClient, ClientClient, BillingClient, MessagingClient, AnalyticsClient, WebhookClient, AuditClient
- [x] T020 Implement session auth middleware in `internal/gateway/portal/middleware/session_auth.go`: Redis session lookup, extract user_id/client_id/role into context, reject if no valid session
- [x] T021 [P] Implement CSRF middleware in `internal/gateway/portal/middleware/csrf.go`: double-submit cookie pattern, verify X-CSRF-Token header on POST/PUT/DELETE
- [x] T022 [P] Implement rate limit middleware in `internal/gateway/portal/middleware/rate_limit.go`: Redis-based rate limiting for login endpoint (5 attempts/15min per email, 20/15min per IP)
- [x] T023 [P] Implement logging middleware in `internal/gateway/portal/middleware/logging.go`: zerolog, method/path/status/duration (follow client-gateway pattern)
- [x] T024 [P] Implement recovery middleware in `internal/gateway/portal/middleware/recovery.go`: panic recovery with JSON error response (follow client-gateway pattern)
- [x] T025 [P] Implement CORS middleware in `internal/gateway/portal/middleware/cors.go`: allow frontend origin, credentials support (follow client-gateway pattern)
- [x] T026 Implement common handler utilities in `internal/gateway/portal/handlers/common.go`: respondJSON, respondError, respondGRPCError, parseIntParam, pagination helpers (follow client-gateway pattern)
- [x] T027 Implement portal router in `internal/gateway/portal/router/router.go`: setup all route groups under /portal/v1/, apply middleware stack (recovery → logging → cors → session_auth), health/metrics endpoints without auth

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Авторизация и управление профилем (Priority: P1) MVP

**Goal**: Клиент входит по email/паролю, может сбросить пароль, включить 2FA (TOTP), обновить профиль. Данные изолированы по tenant'у.

**Independent Test**: Выполнить вход, сброс пароля и включение 2FA через портал.

### Implementation for User Story 1

- [x] T028 [P] [US1] Implement TOTP domain model in `internal/services/auth/domain/totp.go`: TOTPConfig struct, TOTPRecoveryCode struct, validation methods
- [x] T029 [P] [US1] Implement password reset domain model in `internal/services/auth/domain/password_reset.go`: PasswordResetToken struct, IsExpired/IsValid methods
- [x] T030 [US1] Implement TOTP repository in `internal/services/auth/infrastructure/repository/totp_repository.go`: SaveTOTPSecret, GetTOTPConfig, SaveRecoveryCodes, UseRecoveryCode, DeleteTOTPConfig
- [x] T031 [P] [US1] Implement password reset repository in `internal/services/auth/infrastructure/repository/password_reset_repository.go`: Create, GetByTokenHash, MarkUsed, DeleteExpired
- [x] T032 [US1] Implement TOTP service in `internal/services/auth/application/totp_service.go`: GenerateSecret (with AES-256-GCM encryption), ValidateCode (pquerna/otp), SetupTOTP (generate + recovery codes), VerifyAndEnable, Disable, ValidateRecoveryCode
- [x] T033 [P] [US1] Implement password reset service in `internal/services/auth/application/password_reset_service.go`: RequestReset (generate token, hash, store, return plaintext), ResetPassword (validate token, update password, mark used)
- [x] T034 [US1] Extend auth gRPC server in `internal/services/auth/grpc/server.go`: add SetupTOTP, VerifyTOTP, DisableTOTP, RequestPasswordReset, ResetPassword, LoginWithSession, ValidateSession, Logout methods
- [x] T035 [US1] Implement session management in auth service: CreateSession (Redis hash + PostgreSQL record), ValidateSession (Redis lookup), DestroySession (Redis delete + PostgreSQL update), enforce max 5 sessions per user
- [x] T036 [US1] Implement portal auth handlers in `internal/gateway/portal/handlers/auth.go`: POST /auth/login (check credentials, check 2FA, create session, set cookies), POST /auth/login/2fa (verify TOTP, create session), POST /auth/logout (destroy session), POST /auth/password/reset-request (send email, always 202), POST /auth/password/reset (validate token, update password)
- [x] T037 [US1] Implement portal profile handlers in `internal/gateway/portal/handlers/profile.go`: GET /profile (return user + client info), PUT /profile (update contact info), POST /profile/2fa/setup (generate TOTP + QR), POST /profile/2fa/verify (enable 2FA), DELETE /profile/2fa (disable with password check)
- [x] T038 [US1] Wire US1 routes in router: register /auth/* and /profile/* route groups in `internal/gateway/portal/router/router.go`
- [x] T039 [US1] Add audit logging to auth operations: publish audit events for login, logout, password_reset, totp_enabled, totp_disabled, profile.updated via shared audit publisher
- [x] T040 [P] [US1] Create frontend auth pages: LoginPage, PasswordResetRequestPage, PasswordResetPage, TwoFactorPage in `portal-frontend/src/pages/auth/`
- [x] T041 [P] [US1] Create frontend profile page: ProfilePage with 2FA setup/disable in `portal-frontend/src/pages/profile/`
- [x] T042 [US1] Create frontend API client and auth context: API client with session cookie support, AuthContext with login/logout/session state in `portal-frontend/src/api/` and `portal-frontend/src/contexts/`
- [x] T043 [US1] Create frontend App shell: layout with navigation, route guards (redirect to login if not authenticated) in `portal-frontend/src/App.tsx` and `portal-frontend/src/components/Layout.tsx`

**Checkpoint**: User Story 1 fully functional — login, logout, password reset, 2FA, profile management work independently

---

## Phase 4: User Story 2 - Просмотр баланса и истории сообщений (Priority: P1)

**Goal**: Клиент видит баланс и историю SMS с фильтрацией по дате, статусу, получателю.

**Independent Test**: Авторизоваться и проверить баланс, отфильтровать сообщения по дате и статусу.

### Implementation for User Story 2

- [x] T044 [US2] Implement portal dashboard handler in `internal/gateway/portal/handlers/dashboard.go`: GET /dashboard — aggregate balance (billing gRPC), messages_today/delivered (analytics gRPC), active API keys count, active webhooks count
- [x] T045 [US2] Implement portal messages handler in `internal/gateway/portal/handlers/messages.go`: GET /messages — proxy to messaging gRPC with filters (status, date_from, date_to, destination prefix), pagination, client_id isolation
- [x] T046 [US2] Wire US2 routes in router: register /dashboard and /messages route groups in `internal/gateway/portal/router/router.go`
- [x] T047 [P] [US2] Create frontend DashboardPage in `portal-frontend/src/pages/dashboard/DashboardPage.tsx`: balance card, today stats, quick links
- [x] T048 [P] [US2] Create frontend MessagesPage in `portal-frontend/src/pages/messages/MessagesPage.tsx`: messages table with pagination, date/status/destination filters

**Checkpoint**: User Story 2 functional — dashboard and message history with filtering

---

## Phase 5: User Story 3 - Управление API-ключами (Priority: P1)

**Goal**: Клиент создаёт, просматривает и отзывает API-ключи с поддержкой IP-whitelist.

**Independent Test**: Создать ключ, увидеть секрет один раз, отозвать ключ.

### Implementation for User Story 3

- [x] T049 [US3] Extend API key domain model in `internal/services/auth/domain/api_key.go`: add AllowedIPs []string field, IsIPAllowed(ip string) method
- [x] T050 [US3] Extend API key repository in `internal/services/auth/infrastructure/repository/api_key_repository.go`: handle allowed_ips column in Create, GetByID, ListByUserID, GetByKeyHash
- [x] T051 [US3] Extend auth gRPC server: update CreateAPIKey to accept allowed_ips, update ListAPIKeys response to include allowed_ips, update ValidateToken to check IP against whitelist
- [x] T052 [US3] Implement portal API keys handler in `internal/gateway/portal/handlers/api_keys.go`: GET /api-keys (list via auth gRPC), POST /api-keys (create with name, scopes, allowed_ips, expires_at — return key once), DELETE /api-keys/{id} (revoke)
- [x] T053 [US3] Wire US3 routes in router: register /api-keys route group in `internal/gateway/portal/router/router.go`
- [x] T054 [US3] Add audit logging: publish audit events for api_key.created, api_key.revoked
- [x] T055 [P] [US3] Create frontend APIKeysPage in `portal-frontend/src/pages/api-keys/APIKeysPage.tsx`: keys list table, create key dialog (show secret once with copy button), revoke confirmation, IP whitelist input

**Checkpoint**: User Story 3 functional — full API key lifecycle with IP restrictions

---

## Phase 6: User Story 4 - Аналитика по отправкам (Priority: P2)

**Goal**: Агрегированная аналитика: количество по статусам, конверсия, расходы за период с графиками.

**Independent Test**: Выбрать период и проверить графики и метрики.

### Implementation for User Story 4

- [x] T056 [US4] Implement portal analytics handler in `internal/gateway/portal/handlers/analytics.go`: GET /analytics — proxy to analytics gRPC, aggregate summary (sent/delivered/failed/expired, delivery_rate, total_cost), build timeline by day/week, group by country if requested
- [x] T057 [US4] Wire US4 routes in router: register /analytics route group in `internal/gateway/portal/router/router.go`
- [x] T058 [US4] Create frontend AnalyticsPage in `portal-frontend/src/pages/analytics/AnalyticsPage.tsx`: period selector (7d/30d/90d/custom), summary cards, delivery rate line chart (recharts), cost bar chart, country breakdown table

**Checkpoint**: User Story 4 functional — analytics dashboard with charts and filters

---

## Phase 7: User Story 5 - Настройка Webhook'ов (Priority: P2)

**Goal**: Клиент настраивает webhook URL для DLR, тестирует доставку, видит историю.

**Independent Test**: Добавить webhook, отправить тест, проверить результат.

### Implementation for User Story 5

- [x] T059 [US5] Implement portal webhooks handler in `internal/gateway/portal/handlers/webhooks.go`: GET /webhooks (list via webhook gRPC), POST /webhooks (create, return secret once), PUT /webhooks/{id} (update URL/events), DELETE /webhooks/{id}, POST /webhooks/{id}/test (trigger test event, return status)
- [x] T060 [US5] Wire US5 routes in router: register /webhooks route group in `internal/gateway/portal/router/router.go`
- [x] T061 [US5] Add audit logging: publish events for webhook.created, webhook.updated, webhook.deleted, webhook.test_sent
- [x] T062 [US5] Create frontend WebhooksPage in `portal-frontend/src/pages/webhooks/WebhooksPage.tsx`: webhooks list, create/edit dialog (URL, event types selector), test button with result display, delete confirmation, status badge (active/deactivated/error)

**Checkpoint**: User Story 5 functional — webhook CRUD with test delivery

---

## Phase 8: User Story 6 - Создание и управление Sub-accounts (Priority: P2)

**Goal**: Реселлер создаёт sub-accounts с балансами, лимитами и собственным логином. Полный доступ к данным sub-accounts.

**Independent Test**: Создать sub-account, пополнить баланс, проверить блокировку при нулевом балансе.

### Implementation for User Story 6

- [x] T063 [US6] Extend client domain model in `internal/services/client/domain/client.go`: add ParentClientID, IsReseller, MaxSubAccounts fields, add IsSubAccount(), CanCreateSubAccount() methods
- [x] T064 [US6] Implement sub-account repository in `internal/services/client/infrastructure/repository/sub_account_repository.go`: CreateSubAccount (insert with parent_client_id), ListByParentID, GetSubAccount, DeleteSubAccount, UpdateLimits, CountByParentID
- [x] T065 [US6] Implement sub-account application logic in `internal/services/client/application/client_service.go`: extend with CreateSubAccount (validate reseller, check limit, create client+user+account atomically), DeleteSubAccount (return balance, cascade), UpdateSubAccountLimits
- [x] T066 [US6] Extend client gRPC server in `internal/services/client/grpc/server.go`: add CreateSubAccount, ListSubAccounts, GetSubAccount, DeleteSubAccount, UpdateSubAccountLimits methods
- [x] T067 [US6] Implement balance transfer in `internal/services/billing/domain/transfer.go`: BalanceTransfer struct, new TransactionType constants (transfer_out, transfer_in)
- [x] T068 [US6] Implement transfer in billing application in `internal/services/billing/application/billing_service.go`: TransferBalance method — atomic PG transaction (check balance, debit sender, credit receiver, create 2 transactions + balance_transfer record)
- [x] T069 [US6] Extend billing gRPC server in `internal/services/billing/grpc/server.go`: add TransferBalance method
- [x] T070 [US6] Implement portal sub-accounts handler in `internal/gateway/portal/handlers/sub_accounts.go`: GET /sub-accounts (list with balances/stats), POST /sub-accounts (create with initial balance), GET /sub-accounts/{id} (details), PUT /sub-accounts/{id}/limits, POST /sub-accounts/{id}/transfer, DELETE /sub-accounts/{id} (return balance), GET /sub-accounts/{id}/messages, GET /sub-accounts/{id}/analytics, GET /sub-accounts/{id}/api-keys, GET /sub-accounts/{id}/webhooks
- [x] T071 [US6] Wire US6 routes in router: register /sub-accounts route group in `internal/gateway/portal/router/router.go`, add reseller-check middleware for sub-account routes
- [x] T072 [US6] Add audit logging: publish events for sub_account.created, sub_account.deleted, sub_account.limit_updated, balance.transfer_out, balance.transfer_in
- [x] T073 [P] [US6] Create frontend SubAccountsListPage in `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx`: sub-accounts table with balances/stats, create dialog (name, email, initial balance, limits), limit indicator
- [x] T074 [P] [US6] Create frontend SubAccountDetailPage in `portal-frontend/src/pages/sub-accounts/SubAccountDetailPage.tsx`: tabs for messages/analytics/api-keys/webhooks of sub-account, balance transfer form, limits editor, delete with confirmation

**Checkpoint**: User Story 6 functional — full sub-account lifecycle with balance transfers and reseller dashboard

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Audit log UI, worker consumer, Docker integration, security hardening

- [x] T075 Implement audit Kafka consumer in `internal/shared/audit/consumer.go`: consume from `audit.events` topic, batch insert into `audit_log` table, idempotency by event_id
- [x] T076 Integrate audit consumer into worker: register audit consumer in `cmd/worker/main.go`
- [x] T077 Implement audit gRPC service in `internal/services/audit/`: domain model, repository (query audit_log with filters/pagination), gRPC server (QueryAuditLog)
- [x] T078 Implement portal audit log handler in `internal/gateway/portal/handlers/audit.go`: GET /audit-log — proxy to audit gRPC with filters (action, date_from, date_to, user_id), pagination
- [x] T079 Wire audit routes in router: register /audit-log route group in `internal/gateway/portal/router/router.go`
- [x] T080 [P] Create frontend AuditLogPage in `portal-frontend/src/pages/audit/AuditLogPage.tsx`: audit log table with filters (action type, date range, user), pagination
- [x] T081 Update `deployments/docker-compose.yml`: add portal-gateway service (port 8082), portal-frontend nginx service (port 8083), HAProxy config for /portal/* routing
- [x] T082 [P] Create Dockerfile for portal-gateway in `cmd/portal-gateway/Dockerfile`
- [x] T083 [P] Create Dockerfile for portal-frontend in `portal-frontend/Dockerfile` (multi-stage: node build + nginx serve)
- [x] T084 Add Prometheus metrics to portal-gateway: login_attempts_total, active_sessions_gauge, api_requests_total by endpoint, audit_events_published_total
- [x] T085 [P] Create partition management SQL for audit_log: auto-create monthly partitions for next 12 months in `migrations/000021_create_audit_partitions_2026.up.sql`
- [x] T086 Security review: verify tenant isolation in all portal handlers (client_id enforcement), verify CSRF on all mutations, verify session invalidation on password change, verify TOTP secret encryption
- [x] T087 Run quickstart.md validation: verify all curl commands from quickstart.md work end-to-end
- [ ] T088 Verify sub-account send blocking: confirm that messaging-service checks balance via billing-service using sub-account's own client_id (not parent's), and that daily/monthly limits in client-service config are enforced for sub-account client_id during message send flow

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-8)**: All depend on Foundational phase completion
  - US1 (Auth/Profile) can proceed in parallel with others after Foundation
  - US2 (Balance/Messages) can proceed in parallel — uses existing gRPC services
  - US3 (API Keys) can proceed in parallel — extends auth service
  - US4 (Analytics) can proceed in parallel — uses existing analytics gRPC
  - US5 (Webhooks) can proceed in parallel — uses existing webhook gRPC
  - US6 (Sub-accounts) depends loosely on US1 (auth flow for sub-account users) but can be developed independently
- **Polish (Phase 9)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: No dependencies on other stories. Foundation only.
- **US2 (P1)**: No dependencies on other stories. Foundation only.
- **US3 (P1)**: No dependencies on other stories. Foundation only.
- **US4 (P2)**: No dependencies on other stories. Foundation only.
- **US5 (P2)**: No dependencies on other stories. Foundation only.
- **US6 (P2)**: Can be developed independently but integration-tested after US1 (sub-account login flow).

### Within Each User Story

- Domain models before repositories
- Repositories before application services
- Application services before gRPC server
- gRPC server before portal handlers
- Portal handlers before frontend pages
- Audit logging after core handler implementation

### Parallel Opportunities

- All migrations (T005-T011) can run in parallel
- All proto files (T012-T015) can be edited in parallel (regenerate once at T016)
- All middleware (T020-T025) can be developed in parallel
- US1, US2, US3 (all P1) can be developed in parallel after Foundation
- US4, US5 can be developed in parallel
- Frontend pages within different stories can be developed in parallel

---

## Parallel Example: Foundation Phase

```bash
# Launch all migrations in parallel:
Task: T005 "Create migration 000014_add_totp_tables"
Task: T006 "Create migration 000015_add_password_reset_tokens"
Task: T007 "Create migration 000016_add_sub_accounts"
Task: T008 "Create migration 000017_create_audit_log"
Task: T009 "Create migration 000018_add_sessions_table"
Task: T010 "Create migration 000019_add_api_key_ip_whitelist"
Task: T011 "Create migration 000020_create_balance_transfers"

# Launch all middleware in parallel:
Task: T021 "CSRF middleware"
Task: T022 "Rate limit middleware"
Task: T023 "Logging middleware"
Task: T024 "Recovery middleware"
Task: T025 "CORS middleware"
```

## Parallel Example: P1 User Stories

```bash
# After Foundation, launch all P1 stories in parallel:
# Developer A: US1 (Auth/Profile) — T028-T043
# Developer B: US2 (Balance/Messages) — T044-T048
# Developer C: US3 (API Keys) — T049-T055
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T027)
3. Complete Phase 3: US1 Auth/Profile (T028-T043)
4. **STOP and VALIDATE**: Login, logout, password reset, 2FA, profile management
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 (Auth/Profile) → Test → Deploy (MVP!)
3. Add US2 (Balance/Messages) + US3 (API Keys) → Test → Deploy (Core Portal)
4. Add US4 (Analytics) + US5 (Webhooks) → Test → Deploy (Full Portal)
5. Add US6 (Sub-accounts) → Test → Deploy (Reseller Model)
6. Polish (Audit UI, Docker, Security) → Test → Deploy (Production Ready)

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: US1 (Auth/Profile)
   - Developer B: US2 (Balance/Messages) + US3 (API Keys)
   - Developer C: US4 (Analytics) + US5 (Webhooks)
3. After P1 stories complete:
   - Developer A: US6 (Sub-accounts)
   - Developer B: Polish (Audit, Docker)
   - Developer C: Security review + testing

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- All portal handlers MUST enforce client_id isolation (tenant safety)
- All mutations MUST publish audit events via shared audit publisher
- Frontend pages follow existing patterns (if any) or use shadcn/ui + recharts per research.md

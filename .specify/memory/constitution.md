<!--
Sync Impact Report
- Version change: 1.1.2 → 1.2.0
- Modified principles:
  II. Event-Driven Architecture — added event versioning roadmap note
  (schema_version field recommended, current JSON without versions acknowledged)
- Added sections: none
- Removed sections: none
- Clarifications:
  Technical Constraints: Currency — added RUB-only constraint
  Technical Constraints: Auth — clarified CSRF status (portal-gateway
  responsibility, explicit middleware required before production)
  Development Workflow: Test build tags — expanded to include functional,
  load, and integration tags reflecting actual codebase usage
  Development Workflow: Test directories — documented test/ and tests/
  coexistence with purpose distinction
- Templates requiring updates:
  ✅ plan-template.md — no changes needed (Constitution Check is dynamic)
  ✅ spec-template.md — no changes needed
  ✅ tasks-template.md — no changes needed
- Follow-up TODOs:
  ⚠ Event schema versioning: add schema_version field to Kafka message
    structs (tracked as future improvement, not blocking)
  ⚠ CSRF middleware: verify portal-gateway implements CSRF protection
    before production deployment
  ⚠ Data retention policies: verify 90d/1y SQL policies on partitioned
    tables
  ⚠ README.md is outdated (refers to "API Gateway + SMPP Server + Worker"
    but actual architecture has 14+ microservices) — flagged for manual
    update
-->

# SMS Gateway Platform Constitution

## Core Principles

### I. Domain-Driven Design

Each microservice MUST own a single bounded context with clear domain boundaries.
Services MUST follow the DDD layered structure: `domain/`, `application/`,
`infrastructure/`, `grpc/`. Domain logic MUST NOT depend on infrastructure
(no database imports in domain layer). Cross-service communication MUST use
gRPC with Protocol Buffers — no direct database sharing between services.

### II. Event-Driven Architecture

State changes that affect other services MUST be published as events via
Apache Kafka. Message lifecycle transitions (created, queued, sent, delivered,
failed, expired) MUST produce Kafka events. Consumers MUST be idempotent —
processing the same event twice MUST NOT corrupt state. Event schemas MUST
be versioned and backward-compatible; at minimum, each event struct SHOULD
include a `schema_version` field. Until a schema registry is adopted, backward
compatibility MUST be maintained via additive-only JSON field changes (no
field removals or type changes without a new topic).

### III. Contract-First APIs

All external APIs (HTTP REST, gRPC, SMPP) MUST be defined in contract files
(proto definitions, OpenAPI) before implementation. Breaking changes to public
APIs MUST be versioned. Internal gRPC contracts between services MUST use
the `api/proto/` directory with generated code in `api/proto/<service>v1/`.
Proto changes MUST be regenerated before merge.

### IV. Observability

Every service MUST expose Prometheus metrics on a dedicated metrics port
(default: 2112). Structured JSON logging via zerolog MUST be used — no
`fmt.Println` or unstructured output. Every request MUST carry a `request_id`
(UUID, propagated via `X-Request-ID` header) for tracing. Health check
endpoints (`/health`, `/health/live`, `/health/ready`) MUST be implemented
for all services. Critical business metrics (messages sent, delivered, failed,
segments, DLR expiry, provider throughput) MUST have dedicated Prometheus
counters/histograms.

### V. Data Safety

Messages MUST NOT be lost once accepted by the system (acknowledged to client).
Database tables with high write volume MUST use monthly partitioning.
Data retention policies MUST be enforced automatically (90 days for messages,
1 year for audit logs). Sensitive data (API keys, provider passwords, TOTP
secrets) MUST be stored encrypted or hashed — never in plaintext logs.
Balance operations MUST be atomic — no partial charges or double billing.
All monetary values MUST use RUB (Russian Ruble) as the single currency —
multi-currency support is out of scope.

### VI. Simplicity

Prefer the simplest solution that meets requirements. Existing services
SHOULD be extended rather than creating new ones. New services MUST be
justified in the plan's Complexity Tracking section with rationale for why
extending an existing service is insufficient (e.g., distinct bounded
context, separate data lifecycle, incompatible scaling profile). Background
goroutines MUST follow the established Start/Stop pattern (see scheduler.go).
Configuration MUST use environment variables with sensible defaults.
Avoid premature abstraction — three similar lines are better than one
premature helper.

## Technical Constraints

- **Language**: Go 1.24+ for all backend services
- **Frontend**: React 19 + TypeScript + Vite for web portals; served via
  nginx in separate container
- **Database**: PostgreSQL 15+ with pgx driver, monthly partitioning for
  high-volume tables (messages, events, lookup_log)
- **Cache**: Redis 7+ for rate limiting, API key caching, session storage,
  and domain data caching (e.g., HLR lookup results with TTL)
- **Messaging**: Apache Kafka via Sarama for all async communication;
  producers MUST use `Idempotent=true` with `RequiredAcks=WaitForAll`
- **Protocols**: SMPP v3.4 (custom implementation), HTTP REST (gorilla/mux),
  gRPC (google.golang.org/grpc)
- **Auth**: JWT + API keys for machine-to-machine (client-gateway);
  session cookies + CSRF tokens for web portals (portal-gateway).
  CSRF middleware MUST be verified in portal-gateway before production
  deployment
- **Currency**: RUB only — all billing, tarification, and balance
  operations MUST use Russian Ruble
- **Deployment**: Docker Compose with HAProxy load balancing
- **Monitoring**: Prometheus + Grafana; all services expose /metrics on
  port 2112. Grafana dashboards MUST be version-controlled as JSON files
  in `deployments/configs/grafana/dashboards/` and provisioned via YAML
  (datasources + dashboard providers) — no manually created dashboards

## Development Workflow

- Feature branches MUST follow the pattern `NNN-short-name`
- Specifications MUST precede implementation (`/speckit.specify` →
  `/speckit.plan` → `/speckit.tasks` → `/speckit.implement`)
- Proto files MUST be regenerated after any `.proto` change
- Test build tags:
  - `//go:build integration` — integration tests (cross-service, DB, Kafka)
  - `//go:build functional` — functional tests (single-service with real
    dependencies)
  - `//go:build load` — load/performance tests
- Test directories: `test/` for functional, load, and performance tests;
  `tests/` for integration tests
- Database migrations MUST be sequential (`000NNN_description.up.sql`)
  and reversible (matching `.down.sql`)
- All services MUST have graceful shutdown handling OS signals (SIGINT,
  SIGTERM) with 30-second timeout

## Governance

This constitution supersedes ad-hoc practices and informal conventions.
All code changes MUST be consistent with these principles. Violations
MUST be justified in the plan's Complexity Tracking section.
Amendments require: (1) documented rationale, (2) version bump,
(3) propagation to dependent templates.

**Version**: 1.2.0 | **Ratified**: 2026-03-20 | **Last Amended**: 2026-03-31

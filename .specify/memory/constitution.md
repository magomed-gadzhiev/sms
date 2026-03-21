<!--
Sync Impact Report
- Version change: 1.0.0 → 1.1.0
- Modified principles:
  VI. Simplicity — refined "New services MUST NOT be created" to allow
  query-only bounded contexts with Complexity Tracking justification
- Added sections:
  Technical Constraints: Frontend stack (React 19, TypeScript, Vite)
  Technical Constraints: Session auth (cookies, CSRF) for web portals
- Removed sections: none
- Templates requiring updates:
  ✅ plan-template.md — no changes needed (Constitution Check is dynamic)
  ✅ spec-template.md — no changes needed
  ✅ tasks-template.md — no changes needed
- Follow-up TODOs: none
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
be versioned and backward-compatible.

### III. Contract-First APIs

All external APIs (HTTP REST, gRPC, SMPP) MUST be defined in contract files
(proto definitions, OpenAPI) before implementation. Breaking changes to public
APIs MUST be versioned. Internal gRPC contracts between services MUST use
the `api/proto/` directory with generated code in `api/proto/<service>v1/`.
Proto changes MUST be regenerated before merge.

### IV. Observability

Every service MUST expose Prometheus metrics on a dedicated metrics port.
Structured JSON logging via zerolog MUST be used — no `fmt.Println` or
unstructured output. Every request MUST carry a `request_id` for tracing.
Health check endpoints MUST be implemented for all services. Critical
business metrics (messages sent, delivered, failed, segments, DLR expiry)
MUST have dedicated Prometheus counters/histograms.

### V. Data Safety

Messages MUST NOT be lost once accepted by the system (acknowledged to client).
Database tables with high write volume MUST use monthly partitioning.
Data retention policies MUST be enforced automatically (90 days for messages,
1 year for audit logs). Sensitive data (API keys, provider passwords, TOTP
secrets) MUST be stored encrypted or hashed — never in plaintext logs.
Balance operations MUST be atomic — no partial charges or double billing.

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
  high-volume tables
- **Cache**: Redis 7+ for rate limiting, API key caching, session storage
- **Messaging**: Apache Kafka via Sarama for all async communication
- **Protocols**: SMPP v3.4 (custom implementation), HTTP REST (gorilla/mux),
  gRPC (google.golang.org/grpc)
- **Auth**: JWT + API keys for machine-to-machine (client-gateway);
  session cookies + CSRF tokens for web portals (portal-gateway)
- **Deployment**: Docker Compose with HAProxy load balancing
- **Monitoring**: Prometheus + Grafana; all services expose /metrics

## Development Workflow

- Feature branches MUST follow the pattern `NNN-short-name`
- Specifications MUST precede implementation (`/speckit.specify` → `/speckit.plan`
  → `/speckit.tasks` → `/speckit.implement`)
- Proto files MUST be regenerated after any `.proto` change
- Integration tests MUST use `//go:build integration` build tag
- Database migrations MUST be sequential (`000NNN_description.up.sql`)
  and reversible (matching `.down.sql`)
- All services MUST have graceful shutdown handling OS signals

## Governance

This constitution supersedes ad-hoc practices and informal conventions.
All code changes MUST be consistent with these principles. Violations
MUST be justified in the plan's Complexity Tracking section.
Amendments require: (1) documented rationale, (2) version bump,
(3) propagation to dependent templates.

**Version**: 1.1.0 | **Ratified**: 2026-03-20 | **Last Amended**: 2026-03-21

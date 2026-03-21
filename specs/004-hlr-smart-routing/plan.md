# Implementation Plan: Number Lookup (HLR/MNP) + Smart Routing

**Branch**: `004-hlr-smart-routing` | **Date**: 2026-03-21 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-hlr-smart-routing/spec.md`

## Summary

Интеграция HLR-lookup в pipeline маршрутизации SMS для определения реального оператора получателя (MNP), выбора оптимального провайдера на основе взвешенных факторов (стоимость, качество, доступность), кеширование результатов в Redis (TTL 24h), и предоставление standalone API для валидации номеров как отдельного тарифицируемого продукта.

## Technical Context

**Language/Version**: Go 1.24.0
**Primary Dependencies**: gorilla/mux (HTTP), google.golang.org/grpc v1.78.0 (gRPC), IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5 (PostgreSQL), redis/go-redis/v9 (Redis), rs/zerolog (logging), prometheus/client_golang (metrics)
**Storage**: PostgreSQL 15+ (pgx driver, monthly partitioning для lookup_log), Redis 7+ (HLR cache)
**Testing**: stretchr/testify, integration tests с build tag `//go:build integration`
**Target Platform**: Linux server (Docker Compose + HAProxy)
**Project Type**: Microservice (расширение существующего routing-service + gateway endpoints)
**Performance Goals**: HLR lookup с кешем добавляет ≤200мс p95 к обработке SMS; bulk lookup 1000 номеров < 30с
**Constraints**: Синхронный lookup блокирует отправку; fallback на prefix-routing при недоступности HLR
**Scale/Scope**: Рынки с высоким MNP (Европа, СНГ); cache hit rate ≥70% после 24ч

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Domain-Driven Design | PASS | HLR-lookup расширяет bounded context routing-service (определение оператора — часть маршрутизации). Новый сервис не создаётся. |
| II. Event-Driven Architecture | PASS | Lookup-результаты публикуются как Kafka-события. Cache invalidation по событию delivery failure. |
| III. Contract-First APIs | PASS | Новые RPC в routing.proto, новые HTTP endpoints в client-gateway. Контракты определяются до реализации. |
| IV. Observability | PASS | Prometheus-метрики для HLR (latency, hit rate, provider health). Zerolog для всех запросов. request_id propagation. |
| V. Data Safety | PASS | Lookup_log partitioned по месяцам. TTL 90 дней (PII). Monetary operations атомарны (per-number billing через tarification). |
| VI. Simplicity | PASS | Расширение routing-service, не новый сервис. HLR-provider management через admin-gateway. |

**Gate result: PASS** — все принципы соблюдены, Complexity Tracking не требуется.

## Project Structure

### Documentation (this feature)

```text
specs/004-hlr-smart-routing/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
# Расширение существующих сервисов (новые файлы/пакеты внутри)

internal/services/routing/
├── domain/
│   ├── hlr.go                    # HLR entities: LookupResult, HLRProvider, NumberStatus
│   ├── smart_route.go            # SmartRoute entity: weighted scoring, provider selection
│   └── route.go                  # (existing) — расширение RouteMessage для HLR-enriched routing
├── application/
│   ├── hlr_service.go            # HLR lookup orchestration: cache → provider → fallback
│   ├── smart_routing_service.go  # Weighted provider selection logic
│   └── routing_service.go        # (existing) — интеграция HLR в pipeline
├── infrastructure/
│   ├── hlr_provider_adapter.go   # Adapter interface + implementations для HLR провайдеров
│   ├── hlr_cache.go              # Redis-based HLR cache (TTL 24h, invalidation)
│   ├── hlr_provider_repo.go      # PostgreSQL repository for HLR provider configs
│   ├── smart_route_repo.go       # PostgreSQL repository for smart route weights
│   └── lookup_log_repo.go        # PostgreSQL repository for lookup audit log
└── grpc/
    └── server.go                 # (existing) — новые RPC: Lookup, BulkLookup, GetHLRProviders

internal/gateway/client/
├── handlers/
│   └── lookup.go                 # HTTP handlers: POST /lookup, POST /lookup/bulk
└── router/
    └── router.go                 # (existing) — регистрация lookup endpoints

internal/gateway/admin/
├── handlers/
│   └── hlr.go                    # Admin: CRUD HLR providers, smart route weights
└── router/
    └── router.go                 # (existing) — регистрация HLR admin endpoints

internal/gateway/portal/
├── handlers/
│   └── lookup.go                 # Portal: lookup history, stats
└── router/
    └── router.go                 # (existing) — регистрация portal lookup endpoints

api/proto/routing/
└── routing.proto                 # (existing) — расширение: Lookup, BulkLookup, HLR provider mgmt, SmartRoute config

migrations/
├── 000NNN_create_hlr_providers.up.sql
├── 000NNN_create_hlr_providers.down.sql
├── 000NNN_create_lookup_log.up.sql
├── 000NNN_create_lookup_log.down.sql
├── 000NNN_create_smart_routes.up.sql
├── 000NNN_create_smart_routes.down.sql
├── 000NNN_add_routing_weights.up.sql
└── 000NNN_add_routing_weights.down.sql
```

**Structure Decision**: Расширение routing-service (HLR lookup — часть домена маршрутизации). Новые HTTP endpoints в client-gateway, admin-gateway, portal-gateway. Нет нового сервиса — соответствует Constitution VI (Simplicity).

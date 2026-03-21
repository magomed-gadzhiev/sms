# Implementation Plan: Multi-tenant Self-Service Portal + Sub-accounts

**Branch**: `003-self-service-portal` | **Date**: 2026-03-21 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/003-self-service-portal/spec.md`

## Summary

Клиентский портал самообслуживания для SMS-шлюза с поддержкой sub-accounts (реселлерская модель). Реализуется как новый portal-gateway (HTTP, сессионная аутентификация) + расширение существующих сервисов (auth, client, billing) + фронтенд SPA. Audit logging реализуется через Kafka-события (shared-пакет) с записью в PostgreSQL.

## Technical Context

**Language/Version**: Go 1.24.0 (backend), TypeScript (frontend SPA)
**Primary Dependencies**: gorilla/mux (HTTP), google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0 (Kafka), jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, golang-jwt/jwt/v5, pquerna/otp (TOTP 2FA), React 19 + Vite (frontend)
**Storage**: PostgreSQL 15+ (pgx, monthly partitioning для audit_log), Redis 7+ (сессии, rate-limiting, кеш)
**Testing**: stretchr/testify, integration tests с `//go:build integration`
**Target Platform**: Linux server (Docker Compose + HAProxy)
**Project Type**: web-service (микросервисная архитектура)
**Performance Goals**: 500 одновременных пользователей портала, аналитика < 3 сек для 30-дневного периода
**Constraints**: Полная изоляция данных между tenant'ами, атомарность балансовых операций
**Scale/Scope**: ~10 существующих сервисов, 3 gateway'я, ~15 новых HTTP-эндпоинтов портала

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Domain-Driven Design | PASS | Portal-gateway — entry point, не новый сервис. Расширения auth/client/billing остаются в своих bounded contexts. Sub-accounts — расширение domain client-service. |
| II. Event-Driven Architecture | PASS | Audit events публикуются в Kafka. Балансовые операции sub-accounts — Kafka events. |
| III. Contract-First APIs | PASS | Proto-файлы расширяются ДО реализации. Portal HTTP API определяется в contracts/. |
| IV. Observability | PASS | Portal-gateway: Prometheus метрики, zerolog, request_id. Audit log — dedicated метрики. |
| V. Data Safety | PASS | Балансовые переводы атомарны. Audit log — monthly partitioning. API-ключи хешируются. TOTP-секреты шифруются. |
| VI. Simplicity | PASS | Portal-gateway следует паттерну admin-gateway. Audit — shared пакет + Kafka consumer в существующем worker. Без новых микросервисов. |

**Technical Constraints Check:**
- Go 1.24+ для backend: PASS
- PostgreSQL 15+ с pgx: PASS
- Redis 7+: PASS (сессии)
- Kafka via Sarama: PASS (audit events)
- Docker Compose + HAProxy: PASS
- Prometheus + Grafana: PASS

## Project Structure

### Documentation (this feature)

```text
specs/003-self-service-portal/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── portal-api.md   # Portal HTTP API contracts
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
# Portal Gateway (новый gateway, паттерн admin-gateway)
cmd/portal-gateway/
└── main.go

internal/gateway/portal/
├── clients.go                # gRPC клиенты к сервисам
├── handlers/
│   ├── common.go             # Общие утилиты (respondJSON, respondError)
│   ├── auth.go               # Login, logout, password reset, 2FA
│   ├── dashboard.go          # Баланс, сводка
│   ├── messages.go           # История сообщений, фильтрация
│   ├── api_keys.go           # CRUD API-ключей
│   ├── webhooks.go           # CRUD webhook-эндпоинтов
│   ├── analytics.go          # Аналитика, графики
│   ├── sub_accounts.go       # Управление sub-accounts
│   ├── profile.go            # Профиль, настройки, 2FA
│   └── audit.go              # Просмотр аудит-лога
├── middleware/
│   ├── session_auth.go       # Сессионная аутентификация (Redis)
│   ├── logging.go            # Zerolog middleware
│   ├── recovery.go           # Panic recovery
│   ├── cors.go               # CORS для SPA
│   ├── csrf.go               # CSRF-защита
│   └── rate_limit.go         # Rate limiting (login attempts)
└── router/
    └── router.go             # Маршрутизация /portal/v1/

# Расширения существующих сервисов
internal/services/auth/domain/
├── totp.go                   # TOTP 2FA сущность
└── password_reset.go         # Password reset token

internal/services/auth/application/
├── totp_service.go           # 2FA логика
└── password_reset_service.go # Password reset логика

internal/services/auth/infrastructure/repository/
├── totp_repository.go        # TOTP хранилище
└── password_reset_repository.go

internal/services/client/domain/
└── sub_account.go            # Sub-account сущность + reseller fields

internal/services/client/infrastructure/repository/
└── sub_account_repository.go

internal/services/billing/domain/
└── transfer.go               # Balance transfer entity

# Audit (shared пакет)
internal/shared/audit/
├── event.go                  # Audit event model
├── publisher.go              # Kafka publisher
└── consumer.go               # Kafka consumer (для worker)

# Frontend SPA
portal-frontend/
├── package.json
├── vite.config.ts
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── api/                  # API клиент
│   ├── pages/                # Страницы портала
│   ├── components/           # UI компоненты
│   └── hooks/                # React hooks
└── public/

# Proto расширения
api/proto/auth/auth.proto     # + 2FA, password reset, brute-force methods
api/proto/client/client.proto # + sub-account management
api/proto/billing/billing.proto # + balance transfer
api/proto/audit/audit.proto   # Новый — audit log queries

# Миграции
migrations/
├── 000014_add_totp_tables.up.sql
├── 000014_add_totp_tables.down.sql
├── 000015_add_password_reset_tokens.up.sql
├── 000015_add_password_reset_tokens.down.sql
├── 000016_add_sub_accounts.up.sql
├── 000016_add_sub_accounts.down.sql
├── 000017_create_audit_log.up.sql
├── 000017_create_audit_log.down.sql
├── 000018_add_sessions_table.up.sql
└── 000018_add_sessions_table.down.sql

# Docker
deployments/docker-compose.yml  # + portal-gateway, portal-frontend
```

**Structure Decision**: Portal-gateway как отдельный gateway (паттерн admin-gateway) — разная модель аутентификации (сессии vs API-ключи). Все бизнес-логика остаётся в существующих сервисах. Audit как shared-пакет + Kafka consumer в worker. Frontend — SPA, отдельный контейнер со статикой через nginx.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Новый gateway (portal-gateway) | Сессионная аутентификация (cookies, CSRF) принципиально отличается от API-key auth в client-gateway | Расширение client-gateway смешивает два auth-модели, усложняет middleware стек |
| Frontend SPA (React) | Портал с аналитикой, графиками, real-time обновлениями требует интерактивный UI | Go templates слишком ограничены для сложных дашбордов с графиками и фильтрацией |
| Новый proto (audit.proto) | Audit log — новый bounded context для запросов аудит-данных | Встраивание в существующие proto нарушает принцип единой ответственности |
| Новый сервис (audit) | Audit — query-only bounded context, отделён от write-path (Kafka consumer). Читает из собственной partitioned таблицы с отдельной retention policy (1 год vs 90 дней для сообщений) | Добавление audit queries в billing-service связывает billing domain с audit схемой; analytics-service не имеет доступа к audit_log |

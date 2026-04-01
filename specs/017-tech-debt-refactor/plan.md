# Implementation Plan: Technical Debt Resolution

**Branch**: `017-tech-debt-refactor` | **Date**: 2026-04-01 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/017-tech-debt-refactor/spec.md`

## Summary

Устранение 7 категорий технического долга в Go-бэкенде и React-фронтенде: консолидация дублированной авторизации gRPC в middleware, устранение N+1 запросов в LeastLoadedSelector, исправление утечки памяти в RoundRobinSelector, замена магических DLR-строк на константы, удаление заглушки SMPPConnectionAdapter, исправление тихих ошибок (Warn→Error) и добавление useMemo/useCallback в компоненты таблиц.

## Technical Context

**Language/Version**: Go 1.24.0 (бэкенд), TypeScript 5.x + React 19 (фронтенд)  
**Primary Dependencies**: gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, React 19 + Vite  
**Storage**: PostgreSQL 15+ (pgx), Redis 7+  
**Testing**: stretchr/testify (unit + integration), build tags: integration/functional/load  
**Target Platform**: Linux (Docker Compose), порт 2112 для Prometheus metrics  
**Project Type**: Microservices (14+ сервисов), web-service + SPA frontend  
**Performance Goals**: Сокращение SQL запросов при маршрутизации с N до 1 (SC-002); стабильное потребление памяти round-robin при 24h нагрузке (SC-003)  
**Constraints**: Изменения не затрагивают публичные gRPC-контракты; бэкенд должен компилироваться; существующие тесты должны проходить  
**Scale/Scope**: Рефакторинг в пределах существующих сервисов: messaging-service, routing-service, provider-service, portal-frontend

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Принцип | Статус | Комментарий |
|---------|--------|-------------|
| I. DDD — domain не зависит от infrastructure | ✅ PASS | Рефакторинг не меняет границы домена; middleware добавляется в слой grpc/ |
| II. Event-Driven — Kafka события идемпотентны | ✅ PASS | Исправление Warn→Error в DLR publisher не меняет поведение событий |
| III. Contract-First — proto не меняются | ✅ PASS | Только внутренние изменения; gRPC контракты (`api/proto/`) не затрагиваются |
| IV. Observability — zerolog, no fmt.Println | ✅ PASS | Исправляем уровни логирования; добавляем Error там где был Warn |
| V. Data Safety — нет потерь сообщений | ✅ PASS | Batch-загрузка health данных и очистка round-robin не влияют на доставку |
| VI. Simplicity — расширяем, не создаём | ✅ PASS | Все изменения в существующих файлах; новый файл только для DLR-констант |

**Нет нарушений конституции — все изменения в рамках существующих сервисов.**

## Project Structure

### Documentation (this feature)

```text
specs/017-tech-debt-refactor/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # N/A — нет новых API контрактов
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

Рефакторинг затрагивает существующие файлы. Новые файлы — только там где необходима консолидация.

```text
# Backend (Go microservices)
internal/
├── services/
│   ├── messaging/
│   │   ├── grpc/
│   │   │   ├── server.go              # MODIFY: убрать дублированный uuid.Parse
│   │   │   └── middleware.go          # NEW: UnaryInterceptor для валидации client_id
│   │   └── application/
│   │       ├── dlr_service.go         # MODIFY: Warn→Error для Kafka publish (line 128)
│   │       └── scheduler.go          # REVIEW: Warn на line 113 (обоснован — см. research.md)
│   ├── routing/
│   │   ├── application/
│   │   │   ├── selection_strategy.go  # MODIFY: batch GetHealth + RoundRobin cleanup
│   │   │   └── routing_service.go     # MODIFY: batch GetByIDs вместо N GetByID
│   │   ├── domain/
│   │   │   └── repository.go          # MODIFY: добавить GetHealthBatch, GetByIDs
│   │   └── infrastructure/postgres/
│   │       └── provider_repo.go       # MODIFY: реализовать GetHealthBatch, GetByIDs
│   └── provider/
│       └── infrastructure/smpp/
│           └── pool_adapter.go        # MODIFY: удалить ConnectionAdapter.SendMessage заглушку
├── smpp/
│   └── protocol/
│       └── constants.go               # MODIFY: добавить DLR status constants
└── shared/
    └── dlr/
        └── status.go                  # NEW: именованные DLR-статус константы

# Frontend (React + TypeScript)
portal-frontend/src/
├── pages/
│   ├── campaigns/
│   │   ├── CampaignsPage.tsx          # MODIFY: useMemo для columns (line 90)
│   │   └── CampaignDetailPage.tsx     # MODIFY: useMemo/useCallback
│   ├── messages/
│   │   └── MessagesPage.tsx           # MODIFY: useMemo для columns
│   ├── contacts/
│   │   └── ContactListDetailPage.tsx  # MODIFY: useMemo/useCallback
│   └── [other pages with DataTable]   # MODIFY: аналогично
```

**Structure Decision**: Монорепо с microservices структурой. Рефакторинг — только изменение существующих файлов + 2 новых файла (`middleware.go` для gRPC interceptor, `shared/dlr/status.go` для констант).

## Complexity Tracking

*Нет нарушений конституции — секция не требуется.*

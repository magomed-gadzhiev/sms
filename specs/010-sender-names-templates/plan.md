# Implementation Plan: Sender Names & Message Templates Registration

**Branch**: `010-sender-names-templates` | **Date**: 2026-03-31 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/010-sender-names-templates/spec.md`

## Summary

Реализация механизма регистрации имён отправителей (Sender ID) с workflow одобрения администратором, привязка шаблонов сообщений к одобренным именам и поддержка переменных в шаблонах. Функциональность расширяет существующий `template-service` (добавляется домен `SenderName`) и использует готовую инфраструктуру approval workflow.

## Technical Context

**Language/Version**: Go 1.24.0 (backend), TypeScript 5.x + React 19 (frontend)  
**Primary Dependencies**: gorilla/mux, google.golang.org/grpc v1.78.0, IBM/sarama v1.43.0, jackc/pgx/v5, redis/go-redis/v9, rs/zerolog, prometheus/client_golang, stretchr/testify  
**Storage**: PostgreSQL 15+ (pgx driver); новые таблицы `sender_names`, `sender_name_status_history`; ALTER TABLE `templates`  
**Testing**: testify/assert + testify/mock; `//go:build integration` для тестов с БД  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: web-service (microservices)  
**Performance Goals**: <200ms p95 для CRUD операций с именами отправителей  
**Constraints**: <200ms p95, RUB only (не применимо к данной фиче), offline-capable — N/A  
**Scale/Scope**: Неограниченное количество имён/шаблонов на клиента; ожидаемый объём — единицы тысяч записей

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Принцип | Статус | Примечание |
|---------|--------|------------|
| I. DDD — сервис с единым bounded context | ✅ PASS | Расширяем `template-service`, SenderName логически в том же домене |
| I. DDD — domain не зависит от infrastructure | ✅ PASS | domain layer без импортов БД |
| I. DDD — cross-service через gRPC | ✅ PASS | Новый proto; portal/admin gateway вызывают gRPC |
| II. Event-Driven — Kafka события при смене статуса | ✅ PASS | `sender_name.status_changed` публикуется |
| II. Idempotent consumers | ✅ PASS | Продюсер idempotent (WaitForAll), consumers проверяют текущий статус |
| III. Contract-First — proto перед реализацией | ✅ PASS | `api/proto/sender-name/sender_name.proto` создаётся первым |
| IV. Observability — metrics, logging, health | ✅ PASS | template-service уже экспортирует /metrics на :2112 |
| V. Data Safety — no data loss | ✅ PASS | Имена не удаляются, история сохраняется навсегда |
| V. Monthly partitioning | ✅ PASS | `sender_name_status_history` партиционируется (высокий write volume при масштабе) |
| VI. Simplicity — расширение, не новый сервис | ✅ PASS | template-service расширяется, не создаётся sender-name-service |
| VI. New service justification | N/A | Нового сервиса нет |

**Результат gate-проверки**: ✅ Все принципы соблюдены. Переходим к Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/010-sender-names-templates/
├── plan.md              # Этот файл
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── sender_name.proto   # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
# Backend — расширение template-service
internal/services/template/
├── domain/
│   ├── models.go                   # MODIFY: добавить SenderName, SenderNameStatusHistory
│   ├── sender_name.go              # NEW: SenderName domain model + validation
│   └── sender_name_test.go         # NEW: unit tests
├── application/
│   ├── ports.go                    # MODIFY: добавить SenderNameRepository interface
│   ├── template_service.go         # MODIFY: добавить sender_name_id в CreateTemplate
│   ├── sender_name_service.go      # NEW: SenderNameService (CRUD + approval)
│   └── sender_name_service_test.go # NEW: unit tests
├── infrastructure/
│   └── repository/
│       ├── template_repo.go        # MODIFY: поддержка sender_name_id filter/join
│       └── sender_name_repo.go     # NEW: PostgreSQL implementation
└── grpc/
    └── sender_name_handler.go      # NEW: gRPC handler for SenderNameService

# gRPC contracts
api/proto/sender-name/
└── sender_name.proto               # NEW: SenderNameService proto
api/proto/sender-name-v1/           # NEW: generated code (protoc)
api/proto/template/template.proto   # MODIFY: добавить sender_name_id в TemplateInfo

# Migrations
migrations/
├── 000064_sender_names.up.sql      # NEW
└── 000064_sender_names.down.sql    # NEW

# Portal Gateway — operator UI
internal/gateway/portal/
├── clients.go                      # MODIFY: добавить SenderNameServiceClient
├── handlers/
│   └── sender_names.go             # NEW: CRUD + submit/resubmit handlers
└── router/router.go                # MODIFY: /api/v1/sender-names/* routes

# Admin Gateway — admin UI  
internal/gateway/admin/
├── clients.go                      # MODIFY: добавить SenderNameServiceClient
├── handlers/
│   └── sender_names.go             # NEW: list + approve/reject + deactivate
└── router/router.go                # MODIFY: /api/v1/admin/sender-names/* routes

# Frontend — Portal
portal-frontend/src/
├── pages/
│   └── sender-names/
│       └── SenderNamesPage.tsx     # NEW: список + регистрация + редактирование
├── pages/admin/
│   └── SenderNamesAdminPage.tsx   # NEW: (если есть admin panel в frontend)
└── api/
    └── client.ts                   # MODIFY: senderNamesApi

# Service entrypoint
cmd/services/template-service/
└── main.go                         # MODIFY: регистрация SenderNameService в gRPC сервере
```

**Structure Decision**: Монорепозиторий с одним Go-модулем. Расширяем `template-service` (новые файлы в `domain/`, `application/`, `infrastructure/`, `grpc/`). Порталы-шлюзы (portal-gateway, admin-gateway) получают новые handler-файлы и роуты. Frontend получает новую страницу.

## Complexity Tracking

> Нет нарушений Constitution — раздел не заполняется.

---

## Phase 0: Research Findings Summary

*Полные детали в [research.md](research.md)*

### Ключевые решения

1. **Расширение template-service** (не новый сервис) — SenderName в одном bounded context с Template; approval workflow идентичен.

2. **Валидация имени отправителя (GSM 03.40)**:
   - Alphanumeric: `^[A-Za-z0-9 ]{1,11}$`, не может быть только из пробелов
   - Numeric: `^\d{1,15}$`
   - Кириллица — не допускается (нет в GSM Sender ID standard)

3. **Уникальность**: UNIQUE(client_id, name) — один клиент не может иметь два одинаковых имени; разные клиенты — могут.

4. **Kafka событие**: `sender_name.status_changed` при каждой смене статуса.

5. **Templates**: добавляем nullable `sender_name_id FK`; при создании шаблона с sender_name_id — проверяем что имя существует, принадлежит клиенту, в статусе `approved`.

6. **Переменные шаблона**: существующий формат `{{var}}` сохраняется (спецификация упоминает `{имя}`, но система использует `{{var}}`).

---

## Phase 1: Design & Contracts

### Data Model

*Полные детали в [data-model.md](data-model.md)*

#### Новые таблицы

```sql
-- sender_names: имена отправителей
sender_names (id, client_id, name, status, rejection_reason, reviewer_id, reviewed_at, created_at, updated_at)
  UNIQUE(client_id, name)
  status: pending | approved | rejected | deactivated

-- sender_name_status_history: история смен статуса
sender_name_status_history (id, sender_name_id, old_status, new_status, actor_id, actor_type, comment, created_at)
  PARTITIONED BY RANGE (created_at)  -- monthly
```

#### Изменения существующих таблиц

```sql
ALTER TABLE templates ADD COLUMN sender_name_id UUID REFERENCES sender_names(id) ON DELETE SET NULL;
```

#### State machine SenderName

```
       ┌─────────────┐
       │   pending   │ ◄──────────────────┐
       └──────┬──────┘                    │
       admin  │  admin                    │ client resubmit
     approve  │  reject                   │ (edit + submit)
              ▼                           │
    ┌──────────────────┐         ┌────────┴──────┐
    │    approved      │         │   rejected    │
    └────────┬─────────┘         └───────────────┘
     admin   │
  deactivate │
             ▼
    ┌──────────────────┐
    │   deactivated    │
    └──────────────────┘
```

### gRPC Contract

*Полный proto в [contracts/sender_name.proto](contracts/sender_name.proto)*

#### SenderNameService (добавляется в template-service)

```protobuf
service SenderNameService {
  // Operator (Portal)
  rpc CreateSenderName(CreateSenderNameRequest) returns (CreateSenderNameResponse);
  rpc UpdateSenderName(UpdateSenderNameRequest) returns (UpdateSenderNameResponse);
  rpc GetSenderName(GetSenderNameRequest) returns (GetSenderNameResponse);
  rpc ListSenderNames(ListSenderNamesRequest) returns (ListSenderNamesResponse);
  rpc ResubmitSenderName(ResubmitSenderNameRequest) returns (ResubmitSenderNameResponse);
  rpc GetSenderNameHistory(GetSenderNameHistoryRequest) returns (GetSenderNameHistoryResponse);

  // Admin
  rpc ApproveSenderName(ApproveSenderNameRequest) returns (ApproveSenderNameResponse);
  rpc RejectSenderName(RejectSenderNameRequest) returns (RejectSenderNameResponse);
  rpc DeactivateSenderName(DeactivateSenderNameRequest) returns (DeactivateSenderNameResponse);
  rpc ListAllSenderNames(ListAllSenderNamesRequest) returns (ListAllSenderNamesResponse);
}
```

#### Изменения в template.proto

```protobuf
// TemplateInfo — добавить поле
message TemplateInfo {
  // ... existing fields ...
  string sender_name_id = 13;   // NEW: nullable UUID
  string sender_name = 14;      // NEW: denormalized name for display
}

// CreateTemplateRequest — добавить поле
message CreateTemplateRequest {
  // ... existing fields ...
  optional string sender_name_id = 4;  // NEW
}
```

#### HTTP API (Portal Gateway)

```
POST   /api/v1/sender-names                          # Создать заявку
GET    /api/v1/sender-names                          # Список (своих)
GET    /api/v1/sender-names/{id}                     # Детали
PUT    /api/v1/sender-names/{id}                     # Обновить (только rejected)
POST   /api/v1/sender-names/{id}/resubmit            # Повторно отправить на рассмотрение
GET    /api/v1/sender-names/{id}/history             # История статусов
```

#### HTTP API (Admin Gateway)

```
GET    /api/v1/admin/sender-names                    # Все заявки (с фильтром по статусу)
GET    /api/v1/admin/sender-names/{id}               # Детали
POST   /api/v1/admin/sender-names/{id}/approve       # Одобрить
POST   /api/v1/admin/sender-names/{id}/reject        # Отклонить
POST   /api/v1/admin/sender-names/{id}/deactivate    # Деактивировать
```

### Frontend Pages

#### Оператор: SenderNamesPage

- Список имён отправителей с badge-статусами
- Кнопка "Зарегистрировать имя" → Modal с формой (поле name + валидация на клиенте)
- Для `rejected` имён: кнопка "Редактировать и повторить" → Modal + POST /resubmit
- Для `rejected` — отображение причины отказа
- История статусов в виде timeline (отдельная вкладка/секция)
- Интеграция с `templatesApi`: при создании шаблона — выбор одобренного имени отправителя (dropdown)

#### Администратор: SenderNamesAdminPage

- Таблица всех заявок с фильтром по статусу
- Actions: Одобрить / Отклонить (с полем причины) / Деактивировать

### Agent Context Update

После генерации плана обновить agent context:

```bash
.specify/scripts/bash/update-agent-context.sh claude
```

---

## Post-Design Constitution Check

| Принцип | Статус | Детали |
|---------|--------|--------|
| I. Bounded context | ✅ PASS | SenderName в template-service — тот же bounded context «управление контентом» |
| II. Kafka events | ✅ PASS | `sender_name.status_changed` определён в contracts |
| III. Contract-first | ✅ PASS | proto + HTTP API задокументированы до impl |
| IV. Observability | ✅ PASS | Наследует metrics/logging template-service |
| V. No data loss | ✅ PASS | sender_name_status_history хранит всю историю; ON DELETE SET NULL для FK |
| V. Partitioning | ✅ PASS | sender_name_status_history партиционируется по месяцам |
| VI. Simplicity | ✅ PASS | Нет лишних абстракций; 2 новые таблицы, 1 новый proto-сервис |

**Итог**: ✅ Дизайн соответствует всем принципам конституции.

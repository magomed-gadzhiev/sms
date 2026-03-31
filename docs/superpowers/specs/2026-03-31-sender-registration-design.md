# Sender Registration Design

**Date:** 2026-03-31
**Status:** Approved
**Feature:** Регистрация имён отправителей и шаблонов у операторов связи (РФ)

---

## Overview

Клиент подаёт заявку на регистрацию альфа-имени отправителя (например, `MYBANK`) у конкретного оператора связи через портал самообслуживания. К заявке прикладываются шаблоны сообщений. Администратор рассматривает заявку, взаимодействует с оператором вручную и фиксирует результат. Система блокирует отправку SMS, если имя отправителя не имеет статуса `approved` у целевого оператора.

---

## Architecture

Новый независимый DDD-сервис `sender-service` с собственными таблицами. Routing-сервис обращается к нему через gRPC при каждой маршрутизации сообщения.

```
Portal Frontend
     │  REST
     ▼
portal-gateway ──────────────────────────────────────────────────────────┐
     │                                                                    │
     │ gRPC (create/list/manage sender registrations & templates)        │
     ▼                                                                    │
sender-service  (новый DDD-сервис)                                       │
     │                                                                    │
     ├── domain/                                                          │
     │     ├── SenderRegistration    (имя + оператор + статус + история) │
     │     └── RegistrationTemplate (шаблон привязан к регистрации)      │
     │                                                                    │
     ├── storage/ → PostgreSQL (новые таблицы)                           │
     │                                                                    │
     └── gRPC server ─────────────────────────────────────────────────── │
                  ▲                                                       │
                  │ CheckSenderAllowed(client_id, sender, operator_id)    │
              routing-service                                             │
              (hot-path валидация при маршрутизации)                     │
                                                                         │
admin-gateway ───────────────────────────────────────────────────────────┘
     │  REST  (review, approve/reject, add comments)
     ▼
sender-service (те же gRPC-методы + admin-только методы)
```

### New Components

| Path | Purpose |
|------|---------|
| `cmd/services/sender-service/` | Новый бинарник (по образцу других сервисов) |
| `internal/services/sender/domain/` | Доменные модели и бизнес-логика |
| `internal/services/sender/application/` | Use cases (create, submit, approve, etc.) |
| `internal/services/sender/infrastructure/` | gRPC сервер, storage адаптер |
| `internal/services/sender/storage/` | PostgreSQL репозитории |
| `internal/gateway/admin/handlers/sender_registrations.go` | Admin REST handlers |
| `internal/gateway/portal/handlers/sender_registrations.go` | Portal REST handlers |
| `migrations/000064_sender_registrations.up.sql` | Новые таблицы |

### Unchanged Components

- `template-service` — не затрагивается (шаблоны регистраций — отдельные сущности от шаблонов сообщений)
- `tarification-service` — не затрагивается (существующий `SenderRegistration` там остаётся как есть)
- `routing-service` — добавляется один новый gRPC-вызов в hot-path

---

## Domain Model

### `SenderRegistration`

```sql
CREATE TABLE sender_registrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id       UUID NOT NULL REFERENCES clients(id),
    operator_id     UUID NOT NULL REFERENCES operators(id),
    sender_name     VARCHAR(11) NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at    TIMESTAMPTZ,
    resolved_at     TIMESTAMPTZ,
    CONSTRAINT sender_registrations_unique UNIQUE (client_id, operator_id, sender_name),
    CONSTRAINT sender_registrations_status_check
        CHECK (status IN ('draft','submitted','approved','rejected','revision_requested'))
);
```

**Поля:**
- `sender_name` — альфа-имя отправителя: только латиница и цифры, 3–11 символов
- `status` — текущий статус в lifecycle
- `submitted_at` — момент подачи клиентом
- `resolved_at` — момент вынесения решения администратором

### `RegistrationTemplate`

```sql
CREATE TABLE registration_templates (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id  UUID NOT NULL REFERENCES sender_registrations(id) ON DELETE CASCADE,
    name             VARCHAR(255) NOT NULL,
    body             TEXT NOT NULL,
    status           VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT registration_templates_status_check
        CHECK (status IN ('active','inactive'))
);
```

**Поля:**
- `name` — человекочитаемое название шаблона
- `body` — текст шаблона с `{{переменными}}` (тот же синтаксис что в template-сервисе)
- `status` — `active` / `inactive` (мягкое удаление)

### `RegistrationComment`

```sql
CREATE TABLE registration_comments (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id  UUID NOT NULL REFERENCES sender_registrations(id) ON DELETE CASCADE,
    author_id        UUID NOT NULL REFERENCES users(id),
    body             TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Назначение:** история переписки — заметки администратора, видимые клиенту (для прозрачности процесса).

---

## Status Lifecycle

```
draft ──[client: submit]──► submitted
                                │
              ┌─────────────────┼──────────────────────┐
              ▼                 ▼                       ▼
          approved          rejected         revision_requested
                                                       │
                                            [client: resubmit]
                                                       ▼
                                                  submitted
```

**Правила переходов:**

| Из \ В | draft | submitted | approved | rejected | revision_requested |
|--------|-------|-----------|----------|----------|--------------------|
| draft | — | клиент (submit) | — | — | — |
| submitted | — | — | admin | admin | admin |
| revision_requested | — | клиент (resubmit) | — | — | — |
| approved | — | — | — | — | — |
| rejected | — | — | — | — | — |

- `approved` и `rejected` — финальные статусы, изменений не допускают.
- Редактировать шаблоны разрешено только в `draft` и `revision_requested`.
- Submit без шаблонов — ошибка (минимум 1 шаблон обязателен).

---

## API

### Portal REST API (клиент)

```
POST   /portal/v1/sender-registrations
GET    /portal/v1/sender-registrations
GET    /portal/v1/sender-registrations/{id}
PUT    /portal/v1/sender-registrations/{id}           -- только draft
POST   /portal/v1/sender-registrations/{id}/submit
POST   /portal/v1/sender-registrations/{id}/resubmit  -- из revision_requested
DELETE /portal/v1/sender-registrations/{id}           -- только draft

POST   /portal/v1/sender-registrations/{id}/templates
PUT    /portal/v1/sender-registrations/{id}/templates/{tid}
DELETE /portal/v1/sender-registrations/{id}/templates/{tid}
GET    /portal/v1/sender-registrations/{id}/comments
```

Клиент видит только свои заявки (фильтрация по `client_id` из JWT).

### Admin REST API

```
GET    /admin/v1/sender-registrations                         -- все (фильтры: status, operator_id, client_id)
GET    /admin/v1/sender-registrations/{id}
POST   /admin/v1/sender-registrations/{id}/approve
POST   /admin/v1/sender-registrations/{id}/reject
POST   /admin/v1/sender-registrations/{id}/request-revision
POST   /admin/v1/sender-registrations/{id}/comments
```

### gRPC (sender-service → routing-service)

```protobuf
service SenderService {
  rpc CheckSenderAllowed(CheckSenderRequest) returns (CheckSenderResponse);
}

message CheckSenderRequest {
  string client_id   = 1;
  string sender_name = 2;
  string operator_id = 3;
}

message CheckSenderResponse {
  bool   allowed           = 1;
  string rejection_reason  = 2;
}
```

---

## Routing Integration

При маршрутизации каждого сообщения routing-сервис:

1. Определяет оператора получателя по номеру телефона (через `operator_prefixes` — уже существует).
2. Вызывает `CheckSenderAllowed(client_id, sender_name, operator_id)`.
3. Если `allowed = false` — сообщение переходит в статус `failed`, `status_description` = `rejection_reason`.

**Кэш:** результат `CheckSenderAllowed` кэшируется в Redis с ключом `sender:check:{client_id}:{sender_name}:{operator_id}`, TTL = 5 минут. При approve/reject/revision_requested — инвалидация кэша.

**Fail-closed:** если sender-service недоступен (gRPC timeout/error) → `allowed = false`. Лучше отказать, чем пропустить незарегистрированного отправителя.

**Bypass:** если `sender_name` не является альфа-именем (то есть — числовой номер телефона) — проверка не применяется.

---

## Error Handling

| Ситуация | HTTP код | Детали |
|----------|----------|--------|
| Редактирование не-draft регистрации | 400 | `status must be draft` |
| Submit без шаблонов | 422 | `at least one template required` |
| Дубль (client+operator+sender) | 409 | `registration already exists` |
| Доступ к чужой регистрации | 403 | — |
| Недопустимый `sender_name` | 422 | `sender_name: 3-11 latin chars or digits` |
| sender-service недоступен (routing) | — | сообщение → failed, reason: `sender_service_unavailable` |
| Redis недоступен | — | fallback: прямой gRPC без кэша |

---

## Testing Strategy

### Unit (domain)

- Переходы статусов: все допустимые и недопустимые пути
- Валидация `sender_name`: граничные случаи (2 символа → ошибка, 11 → ок, кириллица → ошибка)
- Submit без шаблонов → ошибка

### Functional (HTTP)

- Portal lifecycle: draft → submit → (admin approve) → approved
- Portal: попытка редактировать submitted регистрацию → 400
- Portal: submit без шаблонов → 422
- Admin: approve / reject / request-revision / comment
- Admin: фильтрация по статусу и оператору

### Integration (routing)

- Сообщение с approved sender → `allowed = true`
- Сообщение с submitted sender → `allowed = false`
- Сообщение с rejected sender → `allowed = false`
- Сообщение с неизвестным sender → `allowed = false`
- Сообщение с числовым sender → проверка не применяется, `allowed = true`
- sender-service недоступен → сообщение в failed

---

## Sender Name Validation Rules

Требования операторов РФ к альфа-именам:
- Длина: от 3 до 11 символов включительно
- Допустимые символы: латинские буквы (A-Z, a-z), цифры (0-9), пробел, дефис, точка
- Не может начинаться или заканчиваться пробелом
- Не может состоять только из цифр (иначе воспринимается как номер телефона)

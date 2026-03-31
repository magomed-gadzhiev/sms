# Quickstart: Sender Names & Templates — Developer Guide

## Контекст

Фича добавляет в `template-service` поддержку имён отправителей SMS (Sender ID) с workflow одобрения администратором. Шаблоны сообщений привязываются к одобренному имени отправителя.

## Архитектура изменений

```
[Portal Frontend]
      │  HTTP
      ▼
[portal-gateway]  ──gRPC──►  [template-service]
                                  │
[admin-gateway]   ──gRPC──►       │  (SenderNameService + TemplateService)
                                  │
                             [PostgreSQL]
                             sender_names
                             sender_name_status_history
                             templates (+ sender_name_id FK)
                                  │
                             [Kafka]
                             sender_name.status_changed
```

## Запуск локально

```bash
# 1. Применить миграции
scripts/server.sh migrate

# 2. Пересобрать template-service
go build ./cmd/services/template-service/...

# 3. Пересобрать portal-gateway и admin-gateway
go build ./cmd/portal-gateway/...
go build ./cmd/admin-gateway/...

# 4. Regenerate proto (после изменений .proto)
cd api/proto/sender-name && protoc --go_out=. --go-grpc_out=. sender_name.proto
```

## Файлы для изучения (существующий код)

| Файл | Зачем смотреть |
|------|----------------|
| `internal/services/template/domain/models.go` | Паттерн domain model |
| `internal/services/template/application/template_service.go` | Паттерн application service |
| `internal/services/template/infrastructure/repository/` | Паттерн PostgreSQL repository |
| `internal/gateway/portal/handlers/templates.go` | Паттерн portal handler |
| `internal/gateway/admin/handlers/templates.go` | Паттерн admin handler |
| `portal-frontend/src/pages/templates/TemplatesPage.tsx` | Паттерн frontend page |
| `migrations/000054_add_template_workflow.up.sql` | Паттерн миграции |

## Валидация имени отправителя

```go
// Два типа Sender ID по стандарту GSM 03.40:
// Alphanumeric: латиница + цифры + пробелы, 1-11 символов (не только пробелы)
var alphanumericRegex = regexp.MustCompile(`^[A-Za-z0-9 ]{1,11}$`)

// Numeric: только цифры, 1-15 символов
var numericRegex = regexp.MustCompile(`^\d{1,15}$`)

func ValidateSenderName(name string) error {
    if numericRegex.MatchString(name) {
        return nil // numeric sender ID — OK
    }
    if alphanumericRegex.MatchString(name) && strings.TrimSpace(name) != "" {
        return nil // alphanumeric sender ID — OK
    }
    return ErrInvalidSenderName
}
```

## Lifecycle операций

### Оператор: регистрация имени
1. `POST /api/v1/sender-names` → создаётся запись со статусом `pending`
2. Kafka публикует `sender_name.status_changed` (system, null → pending)
3. Оператор видит статус «На рассмотрении» в личном кабинете

### Администратор: одобрение
1. `POST /api/v1/admin/sender-names/{id}/approve` → статус `pending → approved`
2. Kafka публикует `sender_name.status_changed` (admin, pending → approved)
3. Оператор может создавать шаблоны под этим именем

### Оператор: повторная подача (после отклонения)
1. `PUT /api/v1/sender-names/{id}` → обновить имя (только в статусе `rejected`)
2. `POST /api/v1/sender-names/{id}/resubmit` → статус `rejected → pending`
3. Kafka публикует событие

### Создание шаблона с именем отправителя
```json
POST /api/v1/templates
{
  "name": "Промо-шаблон",
  "body": "Здравствуйте, {{name}}! Ваш промокод: {{code}}",
  "sender_name_id": "uuid-of-approved-sender-name"
}
```

**Валидация**: `sender_name_id` должен принадлежать тому же клиенту и иметь статус `approved`.

## Kafka событие

```go
// Топик: sender_name.status_changed
type SenderNameStatusChangedEvent struct {
    SenderNameID string    `json:"sender_name_id"`
    ClientID     string    `json:"client_id"`
    Name         string    `json:"name"`
    OldStatus    string    `json:"old_status"`    // "" при создании
    NewStatus    string    `json:"new_status"`
    ActorID      string    `json:"actor_id"`
    ActorType    string    `json:"actor_type"`    // "client" | "admin" | "system"
    OccurredAt   time.Time `json:"occurred_at"`
}
```

## Тесты

```bash
# Unit tests (без БД)
go test ./internal/services/template/domain/...
go test ./internal/services/template/application/...

# Functional tests (с реальной БД)
go test -tags=functional ./test/...

# Integration tests
go test -tags=integration ./tests/...
```

## Миграция (номер 000064)

```bash
# Применить
scripts/server.sh migrate

# Откатить
migrate -path migrations -database "postgres://..." down 1
```

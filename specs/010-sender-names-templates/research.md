# Research: Sender Names & Message Templates

## Decision Log

### 1. Service Extension vs. New Microservice

**Decision**: Расширить существующий `template-service` (`internal/services/template/`) вместо создания отдельного `sender-name-service`.

**Rationale**:
- Constitution предписывает предпочитать расширение существующего сервиса созданию нового, если доменная связность высока.
- Sender names тесно связаны с templates: шаблоны привязываются к sender names, и оба проходят через approval workflow.
- `template-service` уже содержит инфраструктуру approval workflow (статусы `pending` / `approved` / `rejected`, admin review, `AuditEntry`).
- Переиспользование существующих портов, репозиториев и gRPC-сервера снижает операционные накладные расходы.

**Alternatives Considered**:
- Отдельный `sender-name-service`: отклонено — дублирование approval-инфраструктуры, лишний сетевой хоп при каждом создании шаблона, необходимость синхронизации состояний между двумя сервисами.

---

### 2. Валидация Sender Name (GSM-стандарт)

**Decision**:
- **Alphanumeric Sender ID**: только латинские буквы, цифры и пробелы; длина 1–11 символов; regex `^[A-Za-z0-9 ]{1,11}$`; значение не может состоять только из пробелов.
- **Numeric Sender ID**: только цифры; длина 1–15 символов; regex `^[0-9]{1,15}$`.
- Тип определяется автоматически: если строка соответствует только цифрам — тип `numeric`, иначе — `alphanumeric`.

**Rationale**:
- Alphanumeric Sender ID ограничен 11 символами по стандарту GSM 03.40 (TP-OA field).
- Numeric Sender ID — до 15 цифр (E.164 без `+`).
- Операторы в большинстве стран отклоняют имена с символами вне Latin/digit диапазона.

**Alternatives Considered**:
- Поддержка кириллицы в Sender ID: отклонено — не входит в GSM-стандарт для поля Sender ID; большинство операторов не поддерживают.
- Разрешить спецсимволы (`-`, `_`, `.`): отклонено — не поддерживаются рядом операторов, лучше ограничиться гарантированно безопасным набором.

---

### 3. Статусная Модель SenderName

**Decision**: Состояния: `pending` → `approved` | `rejected`; `approved` → `deactivated`; `rejected` → `pending` (повторная подача после редактирования).

```
pending → approved
pending → rejected
approved → deactivated
rejected → pending  (resubmit)
```

**Rationale**:
- Зеркалирует существующий workflow шаблонов (`pending` / `approved` / `rejected`) — снижает когнитивную нагрузку, переиспользует логику review в admin panel.
- `deactivated` вместо `deleted`: сохраняет исторические данные, не нарушает ссылочную целостность из templates.

**Alternatives Considered**:
- `draft` как начальный статус: отклонено — у sender names нет смысла в состоянии черновика, каждое имя сразу отправляется на модерацию.
- Физическое удаление: отклонено — нарушает FK из `templates.sender_name_id` и аудит-трейл.

---

### 4. Хранение Истории Статусов

**Decision**: Отдельная таблица `sender_name_status_history` (не EAV-колонки в основной таблице).

Схема:
```sql
CREATE TABLE sender_name_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_name_id UUID NOT NULL REFERENCES sender_names(id),
    old_status  TEXT,
    new_status  TEXT NOT NULL,
    actor_id    UUID,
    actor_type  TEXT NOT NULL,  -- 'admin' | 'client'
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Rationale**:
- Чище схема: не засоряет основную таблицу `reviewed_at`, `rejected_reason`, `reviewer_id` и т.д.
- Поддерживает многособытийную историю (несколько reject → resubmit циклов).
- Согласуется с существующим паттерном `AuditEntry` в template domain.

**Alternatives Considered**:
- Soft-колонки в основной таблице (`rejected_at`, `approved_at`, `rejection_reason`): отклонено — при повторных циклах теряется предыдущая история; усложняет запросы к текущему состоянию.

---

### 5. Привязка Template → SenderName

**Decision**: Добавить nullable FK `sender_name_id UUID REFERENCES sender_names(id)` в таблицу `templates`. Шаблон может существовать без sender name (обратная совместимость). Создавать новые шаблоны можно только под `approved` sender name (если `sender_name_id` указан).

**Rationale**:
- Nullable FK обеспечивает backward compatibility: существующие шаблоны без sender name продолжают работать.
- Проверка `approved` при создании шаблона предотвращает запуск кампаний с неодобренным именем.
- Archiving шаблонов — через статус `archived` (не `archived_at` timestamp), для согласованности с существующим паттерном статусной модели в template domain.

**Alternatives Considered**:
- Обязательный sender_name_id для всех шаблонов: отклонено — ломает существующих клиентов, которые не используют sender names.
- `archived_at` timestamp: отклонено — template domain уже использует статусы для soft delete, добавление отдельного timestamp создаёт двойственность.

---

### 6. Обработка Дублирующихся Имён

**Decision**: UNIQUE constraint на `(client_id, sender_name)` — один клиент не может зарегистрировать одно и то же имя дважды. Разные клиенты могут использовать одинаковые имена; коллизии между клиентами разрешает администратор на этапе аппрувала.

**Rationale**:
- Ограничение на уровне БД даёт быстрый и надёжный отказ.
- Политика межклиентских конфликтов — бизнес-решение (first-come-first-served или priority), делегированное администратору.

**Alternatives Considered**:
- Глобальный UNIQUE по `sender_name`: отклонено — слишком жёстко; один клиент мог уже использовать имя, другому его тоже нужно зарегистрировать с другим оператором.
- Проверка дублей только на уровне приложения: отклонено — race condition при конкурентных запросах.

---

### 7. Структура Frontend

**Decision**:
- Клиентский портал: новая страница `SenderNamesPage` в `portal-frontend/src/pages/sender-names/`.
- Admin panel: новая страница `SenderNamesAdminPage` в соответствующем admin-разделе.
- Использовать существующую компонентную библиотеку: `Badge`, `DataTable`, `Modal`, `Input`, `Button`, `ConfirmDialog`.

**Rationale**:
- Единый UX-паттерн с существующими страницами Templates — ниже порог обучения для пользователей и разработчиков.
- Переиспользование компонентов снижает объём новой CSS/TSX.

**Alternatives Considered**:
- Вкладка "Sender Names" внутри существующей страницы Templates: отклонено — sender names — самостоятельная сущность с собственным lifecycle и admin-workflow; отдельная страница даёт более чистую навигацию.

---

### 8. gRPC Контракт

**Decision**: Новый proto-файл `api/proto/sender-name/sender_name.proto` с package `sendername.v1`. В существующий `template.proto` добавить поле `sender_name_id` (optional) в `CreateTemplateRequest` и `TemplateInfo`.

**Rationale**:
- Отдельный proto-файл для sender name соблюдает принцип single responsibility и позволяет версионировать API независимо.
- Добавление `sender_name_id` в template proto — минимальное изменение, backward-compatible (поле optional/nullable).

**Alternatives Considered**:
- Добавить все sender name RPC прямо в `template.proto`: отклонено — нарушает single responsibility, увеличивает поверхность изменений в уже стабильном контракте.

---

### 9. Kafka Events

**Decision**: Публиковать события `sender_name.status_changed` при каждом переходе статуса (pending→approved, pending→rejected, approved→deactivated, rejected→pending).

Структура события:
```json
{
  "event_type": "sender_name.status_changed",
  "sender_name_id": "<uuid>",
  "client_id": "<uuid>",
  "sender_name": "<string>",
  "old_status": "<string>",
  "new_status": "<string>",
  "actor_id": "<uuid>",
  "timestamp": "<iso8601>"
}
```

**Rationale**:
- Constitution: изменения состояния, влияющие на другие сервисы, должны публиковаться как Kafka events.
- Downstream сервисы (campaign-service, routing) могут реагировать на деактивацию sender name без polling.

**Alternatives Considered**:
- Только синхронный gRPC: отклонено — нарушает принцип loose coupling; деактивация имени должна быть видна другим сервисам без tight dependency.

---

### 10. Деактивация Sender Name

**Decision**: Администратор может деактивировать одобренное sender name. Деактивированное имя нельзя использовать для новых кампаний. Существующие кампании, в которых это имя уже зафиксировано, продолжают работать (они хранят строку имени, а не FK).

**Rationale**:
- Деактивация влияет только на будущие использования; ретроактивное изменение отправленных или запущенных кампаний недопустимо с точки зрения аудита и операторских требований.
- Хранение строки имени в записях кампании/сообщения (а не FK) — стандартная практика для immutable historical records.

**Alternatives Considered**:
- Каскадная приостановка активных кампаний при деактивации: отклонено — выходит за рамки данной фичи; требует отдельного saga/workflow; может вызвать нежелательные side effects.

---

## Open Questions (Resolved)

| Вопрос | Решение |
|---|---|
| Допускать ли кириллицу в Sender ID? | Нет — не поддерживается GSM-стандартом для TP-OA |
| Формат переменных в sender name templates: `{var}` или `{{var}}`? | `{{var}}` — единообразно с существующим `template-service` (regex `\{\{(\w+)\}\}`) |
| Хранить историю статусов в основной таблице или отдельно? | Отдельная таблица `sender_name_status_history` |
| Soft delete или смена статуса для архивирования? | Смена статуса (`archived`) — согласованно с паттерном template domain |
| Обязателен ли sender_name_id для шаблонов? | Нет, nullable FK — backward compatibility |
| Нужен ли отдельный микросервис? | Нет — расширить существующий `template-service` |

---

## Implementation Notes

### Порядок реализации

1. **Domain** — добавить `SenderName` struct, константы статусов, validation (regex), методы переходов состояний в `internal/services/template/domain/`.
2. **DB migrations** — таблицы `sender_names`, `sender_name_status_history`, ALTER TABLE `templates ADD COLUMN sender_name_id`.
3. **Repository** — `SenderNameRepository` interface + pgx-реализация в `infrastructure/repository/`.
4. **Application layer** — методы `RegisterSenderName`, `ApproveSenderName`, `RejectSenderName`, `DeactivateSenderName` в `template_service.go`; обновить `CreateTemplate` для проверки approved-статуса sender name.
5. **Kafka publisher** — публикация `sender_name.status_changed` через существующий Kafka producer.
6. **gRPC** — новый proto + имплементация в `grpc/server.go`.
7. **Frontend** — `SenderNamesPage` (client) + `SenderNamesAdminPage` (admin).

### Индексы БД

```sql
-- Основной lookup: клиент ищет свои имена
CREATE INDEX idx_sender_names_client_id ON sender_names(client_id);
-- Быстрая фильтрация по статусу (admin panel)
CREATE INDEX idx_sender_names_status ON sender_names(status);
-- История по конкретному sender name
CREATE INDEX idx_sender_name_history_sender_name_id ON sender_name_status_history(sender_name_id);
```

### Validation Logic

```go
var (
    alphanumericSenderRegex = regexp.MustCompile(`^[A-Za-z0-9 ]{1,11}$`)
    numericSenderRegex      = regexp.MustCompile(`^[0-9]{1,15}$`)
)

func ValidateSenderName(name string) (senderType string, err error) {
    if numericSenderRegex.MatchString(name) {
        return "numeric", nil
    }
    if alphanumericSenderRegex.MatchString(name) && strings.TrimSpace(name) != "" {
        return "alphanumeric", nil
    }
    return "", ErrInvalidSenderName
}
```

### Совместимость с существующими Templates

- Поле `sender_name_id` в `templates` — nullable, существующие записи имеют `NULL`.
- gRPC `CreateTemplateRequest` — поле `sender_name_id` optional (proto3 `optional string`).
- Существующие клиенты не меняют поведение: отсутствие `sender_name_id` в запросе означает "без sender name".

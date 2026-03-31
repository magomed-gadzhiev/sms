# Research: Multichannel Cascade Delivery

**Feature**: `012-multichannel-cascade`  
**Branch**: `012-multichannel-cascade`  
**Date**: 2026-03-31

---

## 1. Архитектура оркестрации каскада

### Решение
Kafka-based event-driven оркестрация. Каждый шаг каскада публикует событие; оркестратор
(`cascade-service`) подписывается на результаты и принимает решение о следующем шаге.

### Обоснование
Spec явно указывает асинхронную Kafka-оркестрацию (Clarifications 2026-03-31). При целевом
масштабе 1 000 каскадов/сек синхронный wait блокировал бы goroutine-пул.

### Альтернативы
- **Синхронный wait**: проще, но не масштабируется и нарушает Constitution II.
- **Saga-паттерн с отдельным workflow-движком (Temporal)**: избыточно для текущего масштаба.

### Kafka-топики (новые)

| Топик | Назначение | Key | Producers | Consumers |
|-------|-----------|-----|-----------|-----------|
| `cascade.start` | Запрос на доставку | delivery_id | client-gateway, portal-gateway | cascade-service |
| `cascade.attempt.send` | Команда на отправку через канал | attempt_id | cascade-service orchestrator | channel adapters |
| `cascade.attempt.result` | Результат попытки (успех/таймаут/ошибка) | attempt_id | channel adapters, webhook handler | cascade-service orchestrator |
| `cascade.billing` | Команда тарификации после завершения | delivery_id | cascade-service | billing consumer |

---

## 2. Channel Interface (in-process)

### Решение
Go-интерфейс `Channel` в домене `cascade`. Каждый канал компилируется вместе с ядром.
Webhook-каналы (flash call) получают подтверждение через входящий HTTP-эндпоинт; этот
эндпоинт публикует `cascade.attempt.result` в Kafka.

```go
// domain/channel.go
type Channel interface {
    Type() ChannelType
    // Send инициирует отправку и возвращает управление; результат приходит через Kafka.
    Send(ctx context.Context, attempt *DeliveryAttempt, cfg *ChannelConfig) error
}
```

### Обоснование
Spec (FR-001, Clarifications): "Go-интерфейс in-process, новый канал компилируется вместе с ядром".
Не плагинная система (dlopen/RPC) — проще и безопаснее.

### Flash Call webhook flow
1. `cascade-service` channel adapter вызывает HTTP провайдера → провайдер звонит получателю.
2. Провайдер при ответе вызывает webhook: `POST /webhooks/flash-call/{attempt_id}` на
   `portal-gateway` или `admin-gateway`.
3. Webhook handler публикует `cascade.attempt.result` {attempt_id, status: "delivered"}.
4. Оркестратор завершает каскад.

---

## 3. Reachability Check

### Решение
Двухуровневая проверка перед каждым шагом:
1. **HLR-данные**: вызов `routing-service` gRPC → `LookupResult.OperatorMCCMNC` определяет оператора.
2. **Таблица совместимости**: `operator_channel_support` → проверяет поддержку канала оператором.

Кеш в Redis (TTL 1 час) по ключу `reachability:{msisdn}:{channel_type}`.

### Обоснование
Spec (FR-005, Clarifications): "HLR (фича 004) + таблица администратора". Кеш предотвращает
повторные HLR-запросы при каждом шаге каскада для одного номера.

---

## 4. Тарификация и биллинг

### Решение
Расширить существующую `tarification-service` для поддержки `channel_type` в запросе
тарификации. Cascade-service вызывает `tarification-service.TarifyMessage` после каждой
тарифицируемой попытки. Биллинг за неудачные попытки: только если попытка "тарифицируемая"
(определяется бизнес-правилом по типу канала и статусу).

**Правило "поздний дубликат"**: attempt помечается `late_duplicate`; тарифицируется только
первый успешный attempt в рамках delivery.

### Интеграция
- gRPC → `tarification-service` (как в `pipeline/sender/stage.go`)
- gRPC → `billing-service` для рефанда при неудаче
- Тарификация через `cascade.billing` Kafka-топик (async, отделяет критический путь доставки)

---

## 5. Новый сервис vs расширение существующего

### Решение
**Новый `cascade-service`** — оправдано по Constitution VI.

### Обоснование нового сервиса

| Критерий | Значение |
|----------|---------|
| Bounded context | Каскадная оркестрация — отдельный домен (стратегии, каналы, reachability, state machine) |
| Данные | `deliveries` + `delivery_attempts` требуют monthly partitioning; lifecycle независим от `messages` |
| Масштабирование | При 1 000 каскадов/сек оркестратор масштабируется отдельно от messaging-service |
| Go-интерфейс Channel | Channel adapters должны жить в одном процессе с оркестратором |

---

## 6. Timeout Management

### Решение
Scheduler goroutine в `cascade-service` (паттерн `Start/Stop` как в `messaging-service/application/scheduler.go`).
Сканирует pending attempts каждые 5 секунд; публикует `cascade.attempt.result` {status: "timeout"}
для попыток, где `created_at + timeout_s < now`.

### Почему не Kafka delayed messages
Sarama не поддерживает delayed messages нативно. Scheduler + DB polling проще и надёжнее.

---

## 7. Frontend

### Решение
Новые страницы в `portal-frontend/src/pages/`:
- `channels/` — управление каналами (только admin)
- `delivery-strategies/` — управление стратегиями (только admin)
- `cascade-history/` — история каскадных доставок (клиент)

Маршруты добавляются в `App.tsx`. Новые API-клиенты в `portal-frontend/src/api/`.

---

## 8. Производительность при 1 000 каскадов/сек

### Оценка
- **Оркестратор**: BatchConsumer (batch=100, timeout=50ms) → 1 000 сообщений/сек = 10 батчей/сек.
- **DB writes**: `delivery_attempts` — monthly partition, bulk insert через pgx batch.
- **Redis cache**: reachability check кешируется, снижает HLR-нагрузку.
- **Kafka partitions**: 12 партиций для `cascade.attempt.result` обеспечивают параллелизм.

Все оценки укладываются в SC-002 (95% за 30 секунд).

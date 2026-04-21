# Design: Cascade Idempotency — DB UNIQUE + CAS

**Date**: 2026-04-21  
**Status**: Draft  
**Scope**: `internal/services/cascade/`  
**Problem**: Spec drift D1/D-C — дублирование шагов каскада при параллельных consumers и at-least-once Kafka

---

## Problem Statement

При параллельных экземплярах сервиса или перечитывании Kafka-offset после рестарта возможны три класса дублирования:

1. **At-least-once redeliver**: consumer перечитывает offset → повторный `StartCascade` / `ProcessAttemptResult`.
2. **Restart с in-flight attempts**: attempt создана, `adapter.Send` ещё не вызван → рестарт → второй consumer обрабатывает тот же event.
3. **Параллельные consumers**: два экземпляра обрабатывают одно событие одновременно → race condition на уровне БД.

Текущая защита: ручная проверка `delivery.Status != pending` в `StartCascade` — не атомарна при параллельном доступе. В `executeNextStep` защиты нет совсем.

---

## Solution: DB Idempotency — UNIQUE + CAS

### Принцип

Использовать атомарные гарантии БД вместо read-then-check:
- `INSERT ... ON CONFLICT DO NOTHING` + проверка `rows_affected` для попыток.
- `UPDATE ... WHERE status = expected` (CAS) для переходов статуса delivery.

Только первый writer побеждает; остальные получают сигнал "уже обработано" и выходят без ошибки.

---

## Changes

### 1. Migration

```sql
ALTER TABLE delivery_attempts
  ADD CONSTRAINT uq_delivery_step UNIQUE (delivery_id, step_order);
```

**Инвариант**: каждый `step_order` в стратегии уникален; каскад создаёт ровно одну попытку на шаг (включая `skipped`). Повторный INSERT конфликтует и возвращает 0 rows_affected.

**Pre-flight перед миграцией**:
```sql
SELECT delivery_id, step_order, COUNT(*)
FROM delivery_attempts
GROUP BY delivery_id, step_order
HAVING COUNT(*) > 1;
```
Если результат пуст — `ALTER TABLE` безопасен.

---

### 2. Domain errors (`domain/errors.go`)

Два новых sentinel:

```go
var ErrAttemptAlreadyExists = errors.New("attempt already exists for this step")
var ErrDeliveryConflict     = errors.New("delivery status conflict: concurrent update")
```

---

### 3. Repository layer

#### `AttemptRepo.Create` (`infrastructure/postgres/attempt_repo.go`)

Добавить `ON CONFLICT DO NOTHING` к INSERT; вернуть `ErrAttemptAlreadyExists` при `rows_affected == 0`:

```go
query := `INSERT INTO delivery_attempts (...) VALUES (...)
          ON CONFLICT (delivery_id, step_order) DO NOTHING`
tag, err := r.pool.Exec(ctx, query, ...)
if err != nil {
    return fmt.Errorf("create attempt: %w", err)
}
if tag.RowsAffected() == 0 {
    return domain.ErrAttemptAlreadyExists
}
return nil
```

#### Новый метод `DeliveryRepo.UpdateStatusCAS` (`infrastructure/postgres/delivery_repo.go`)

```go
func (r *DeliveryRepo) UpdateStatusCAS(
    ctx context.Context,
    id uuid.UUID,
    expectedStatus domain.DeliveryStatus,
    newStatus domain.DeliveryStatus,
    deliveredVia string,
) error {
    query := `UPDATE deliveries SET status=$1, delivered_via=$2, updated_at=$3
              WHERE id=$4 AND status=$5`
    tag, err := r.pool.Exec(ctx, query, newStatus, deliveredVia, time.Now(), id, expectedStatus)
    if err != nil {
        return fmt.Errorf("update delivery status cas: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return domain.ErrDeliveryConflict
    }
    return nil
}
```

Существующий `UpdateStatus` не удаляется — используется в `billing_integration.go` и `delivery_service.go` где CAS не нужен.

Интерфейс `domain.DeliveryRepository` дополняется методом `UpdateStatusCAS`.

---

### 4. Application layer (`application/cascade_service.go`)

#### `StartCascade` — атомарный переход `pending → in_progress`

Заменить ручную проверку статуса (строки 121–131) на CAS:

```go
if err := s.deliveries.UpdateStatusCAS(ctx, deliveryID,
    domain.DeliveryPending, domain.DeliveryInProgress, ""); err != nil {
    if errors.Is(err, domain.ErrDeliveryConflict) {
        return nil // другой consumer уже взял эту доставку
    }
    return fmt.Errorf("update delivery status to in_progress: %w", err)
}
s.metrics.ActiveDeliveries.Inc()
```

#### `executeNextStep` — guard после `attempts.Create`

```go
if err := s.attempts.Create(ctx, attempt); err != nil {
    if errors.Is(err, domain.ErrAttemptAlreadyExists) {
        return nil // второй consumer — шаг уже запущен
    }
    return fmt.Errorf("create attempt: %w", err)
}
// adapter.Send вызывается только если attempt реально создана
```

#### `ProcessAttemptResult` — CAS при финализации delivery

Переход `in_progress → delivered`:
```go
if err := s.deliveries.UpdateStatusCAS(ctx, deliveryID,
    domain.DeliveryInProgress, domain.DeliveryDelivered, attempt.ChannelType); err != nil {
    if errors.Is(err, domain.ErrDeliveryConflict) {
        return nil
    }
    return fmt.Errorf("update delivery delivered: %w", err)
}
```

Переход `in_progress → failed` (в `executeNextStep`, все шаги исчерпаны):
```go
if err := s.deliveries.UpdateStatusCAS(ctx, delivery.ID,
    domain.DeliveryInProgress, domain.DeliveryFailed, ""); err != nil {
    if errors.Is(err, domain.ErrDeliveryConflict) {
        return nil
    }
    return fmt.Errorf("mark delivery failed: %w", err)
}
```

---

## What This Does NOT Fix

- **Spec drift D1 (async flow)**: переходы по-прежнему синхронные. Эта задача — только idempotency поверх существующего синхронного flow.
- **Параллельные стратегии (D2)**: не реализованы, не затрагиваются.
- **`PublishAttemptSend` мёртвый метод**: не удаляется в этом scope.

---

## Testing

- Unit: `AttemptRepo.Create` дважды с одинаковым `(delivery_id, step_order)` → второй вызов возвращает `ErrAttemptAlreadyExists`.
- Unit: `DeliveryRepo.UpdateStatusCAS` с неверным `expectedStatus` → `ErrDeliveryConflict`.
- Integration: два параллельных вызова `StartCascade` с одним `delivery_id` → ровно один attempt создан, ровно один `adapter.Send` вызван.

---

## File Checklist

| Файл | Изменение |
|------|-----------|
| `migrations/20260421_cascade_idempotency.sql` | новый — UNIQUE constraint |
| `internal/services/cascade/domain/errors.go` | +2 sentinel |
| `internal/services/cascade/domain/repository.go` | +`UpdateStatusCAS` в интерфейс |
| `internal/services/cascade/infrastructure/postgres/attempt_repo.go` | `Create` → ON CONFLICT |
| `internal/services/cascade/infrastructure/postgres/delivery_repo.go` | +`UpdateStatusCAS` |
| `internal/services/cascade/application/cascade_service.go` | 3 guard-блока |

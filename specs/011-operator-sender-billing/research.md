# Research: Operator Sender Name Billing

**Feature**: 011-operator-sender-billing  
**Date**: 2026-03-31

## Decisions

### 1. Хранение тарифа оператора

**Decision**: Добавить колонку `monthly_tariff_amount NUMERIC(20,6)` в таблицу `operators`. Поле nullable — NULL для операторов со стратегией "free_only".

**Rationale**: Тариф хранится как единственное текущее значение без истории (confirmed в Clarifications спека). Сумма фиксируется в каждой записи `sender_name_billing_records` на момент начисления. Существующие булевы флаги `supports_paid_sender` / `supports_free_sender` уже кодируют стратегию — отдельный enum не нужен (`supports_paid_sender = true` → стратегия "free-and-paid"; `false` → "free-only").

**Alternatives considered**:
- Отдельная таблица `operator_tariffs` с историей версий → отклонено (спек явно исключает историю тарифов)
- Новый enum-столбец `registration_strategy` → отклонено (избыточно, `supports_paid_sender` несёт тот же смысл)

---

### 2. Расположение billing records

**Decision**: Новая таблица `sender_name_billing_records` в домене `tarification-service`. Поле `billing_month DATE` (первое число месяца), UNIQUE(sender_registration_id, billing_month) для идемпотентности.

**Rationale**: `tarification-service` уже владеет `sender_registrations`, содержит всю биллинговую логику и паттерн идемпотентных записей (tarification_log использует idempotency_key). Расширение этого сервиса соответствует принципу VI конституции (Simplicity).

**Alternatives considered**:
- Billing records в `billing-service` → отклонено (billing-service работает с балансами клиентов, а не с регулярными начислениями за имена)
- Billing records в `template-service` → отклонено (template-service — домен approval workflow, не billing)

---

### 3. Cron-планировщик

**Decision**: Горутина-планировщик внутри `tarification-service` по паттерну `messaging-service/scheduler.go`: `Start()` / `Stop()` / `run()` с `time.Ticker`. Запуск в 00:01 первого числа каждого месяца. Идемпотентность через UNIQUE constraint на `(sender_registration_id, billing_month)` — INSERT ON CONFLICT DO NOTHING.

**Rationale**: Конституция (VI) требует использовать паттерн Start/Stop. Идемпотентность на уровне БД проще и надёжнее, чем application-level проверки. Cron-горутина в Go (`time.Sleep` до следующего первого числа месяца) без внешних зависимостей.

**Alternatives considered**:
- Внешний cron (Kubernetes CronJob, системный cron) → отклонено (добавляет инфраструктурную зависимость, тогда как всё уже живёт в Go-процессе)
- Robfig/cron библиотека → отклонено (overkill, достаточно `time.NewTicker` / sleep-until-next-month)

---

### 4. Показ стоимости inline (FR-008)

**Decision**: Portal-gateway добавляет endpoint `GET /api/operators/:id/sender-tariff` (без аутентификации клиента не возвращает данные — только авторизованный клиент). Frontend при выборе оператора + типа "paid" делает запрос и отображает стоимость за текущий месяц. Стоимость = `monthly_tariff_amount` оператора (полная сумма, без пропорциональных расчётов — confirmed в Assumptions).

**Rationale**: Минимальный новый endpoint. Данные уже доступны в routing-service; portal-gateway проксирует через gRPC GetOperator.

**Alternatives considered**:
- Включить тариф в список операторов → возможно, но специализированный endpoint чище для frontend-потребления
- Показывать тариф только при подтверждении → отклонено (FR-008 требует inline до нажатия кнопки)

---

### 5. История начислений

**Decision**: Endpoint `GET /api/sender-names/:id/billing` в portal-gateway, возвращает список `SenderNameBillingRecord` для данной регистрации, отсортированных по убыванию billing_month.

**Rationale**: Стандартный REST-паттерн, аналогичный существующим list-endpoints в платформе.

---

### 6. Обработка edge cases

**Decision** (на основе Assumptions в спеке):
- **Регистрация в последний день месяца**: полная сумма за месяц, без пропорций (out of scope)
- **Изменение тарифа в середине месяца**: вступает в силу со следующего месяца (биллинг использует тариф на момент начисления в начале месяца)
- **Смена стратегии с "free-and-paid" на "free-only"**: уже существующие платные регистрации остаются платными; billing scheduler проверяет `sender_registrations.type = 'paid' AND status = 'active'` — если статус не изменён, начисление продолжается
- **Недостаточно средств**: вне scope данной фичи (Assumption в спеке)

---

### 7. Существующий billing flow

**Decision**: Понять взаимосвязь `sender_names` (template-service) ↔ `sender_registrations` (tarification-service):
- `sender_names`: lifecycle approval (pending → approved/rejected/deactivated) 
- `sender_registrations`: operator-level registration для routing/tarification с типом (paid/free)
- Новый billing record создаётся через gRPC tarification-service при создании платной регистрации (или через portal-gateway, вызывающий tarification gRPC)

**Rationale**: Billing record создаётся в момент регистрации (FR-004), а не в момент approval в template-service. Это упрощает flow.

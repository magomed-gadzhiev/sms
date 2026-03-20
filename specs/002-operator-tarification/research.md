# Research: Operator-Based SMS Tarification System

**Date**: 2026-03-20
**Status**: Complete — all unknowns resolved

## R-001: Strategy Pattern для стратегий тарификации

**Decision**: Использовать interface-based Strategy Pattern, аналогичный существующему `ProviderSelector` в routing-service.

**Rationale**: В проекте уже есть паттерн Strategy (`selection_strategy.go`), который определяет интерфейс `ProviderSelector` с двумя реализациями (`RoundRobinSelector`, `LeastLoadedSelector`). Tarification-сервис применит тот же подход: интерфейс `BillingStrategy` с 4 реализациями.

**Alternatives considered**:
- Switch/case по типу стратегии в одном методе — отклонено: нарушает OCP, сложно тестировать отдельные стратегии
- Map[strategy_type]func — отклонено: теряем типобезопасность, сложнее с зависимостями (репозитории)

## R-002: Saga-паттерн для взаимодействия tarification → billing

**Decision**: Orchestration-based Saga внутри tarification-service. Tarification-service выступает оркестратором, вызывает billing-service через gRPC синхронно, компенсирует при сбоях.

**Rationale**: В текущей архитектуре admin-gateway уже вызывает gRPC-сервисы синхронно. Tarification-service делает то же: gRPC-вызов `ChargeMessage` в billing-service, при ошибке — компенсирующий вызов. Kafka используется для публикации результатов (async), но сама тарификация — синхронный поток.

**Alternatives considered**:
- Choreography-based Saga через Kafka — отклонено: eventual consistency усложняет отказ от отправки при недостатке средств, нужен синхронный ответ
- Distributed transaction (2PC) — отклонено: избыточная сложность, не поддерживается инфраструктурой

## R-003: Конкурентный доступ к UsageCounter

**Decision**: `SELECT ... FOR UPDATE` при чтении счётчика + `UPDATE SET segment_count = segment_count + N` для атомарного инкремента. PostgreSQL row-level lock гарантирует корректность.

**Rationale**: Стандартный подход для PostgreSQL. Существующий billing-service использует аналогичную атомарную операцию для баланса (`UpdateBalance`). Row-level lock минимизирует contention.

**Alternatives considered**:
- Redis-based counter с периодической синхронизацией в PostgreSQL — отклонено: risк потери данных, усложняет Saga
- Optimistic locking (version column) — отклонено: при высокой конкуренции много retry-ов

## R-004: Расширение routing-service (Country, Operator)

**Decision**: Добавить Country, Operator, OperatorPrefix как доменные сущности в routing-service. Расширить существующий routing.proto новыми RPC. Resolver определяет оператора по prefix matching с приоритетом длины.

**Rationale**: Операторы и страны — часть маршрутизации (определение оператора по номеру). Routing-service уже имеет domain/route.go, domain/rule.go. Country/Operator логически расширяют этот контекст. Определение оператора нужно routing-service для обогащения сообщения.

**Alternatives considered**:
- Отдельный operator-service — отклонено: нарушает принцип VI (Simplicity), Constitution запрещает создавать сервис, когда можно расширить существующий
- Хранить операторов в tarification-service — отклонено: routing-service нужны данные для обогащения, двойное хранение нарушает DDD

## R-005: Хранение и архивация TarificationLog

**Decision**: Таблица tarification_log с monthly partitioning (как messages). Данные хранятся 12 месяцев, затем удаляются cron-задачей (аналогично data purging для messages).

**Rationale**: В проекте уже есть механизм data purging и monthly partitioning для таблицы messages (миграция 000001). TarificationLog — высоковолюмная таблица, аналогичная messages по паттерну записи. Применяем тот же подход.

**Alternatives considered**:
- Хранение бессрочно без партиционирования — отклонено: деградация производительности при росте данных
- Архивация в отдельное хранилище (S3/cold storage) — отклонено: избыточно для текущего масштаба, может быть добавлено позже

## R-006: Взаимодействие admin-gateway с tarification-service

**Decision**: Admin-gateway подключается к tarification-service через gRPC-клиент (аналогично существующему `billingv1.BillingServiceClient`). HTTP-хендлеры преобразуют REST-запросы в gRPC-вызовы.

**Rationale**: Все существующие admin-handlers работают по паттерну HTTP → gRPC adapter. `billing.go` handler получает `BillingServiceClient` через конструктор и делегирует логику. Tarification handlers следуют тому же подходу.

**Alternatives considered**:
- Прямое подключение admin-gateway к БД tarification-service — отклонено: нарушает принцип I (DDD), прямой доступ к чужой БД запрещён Constitution

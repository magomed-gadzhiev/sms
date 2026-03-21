# Research: Number Lookup (HLR/MNP) + Smart Routing

**Feature**: 004-hlr-smart-routing | **Date**: 2026-03-21

## R-001: HLR Provider Integration Pattern

**Decision**: Абстрактный adapter interface для HLR-провайдеров с конкретными реализациями per-provider. Провайдеры конфигурируются в БД с приоритетом и failover.

**Rationale**: В индустрии HLR-провайдеры используют разные протоколы (SS7/MAP, DIAMETER, HTTP REST API). Adapter pattern позволяет подключать новых провайдеров без изменения бизнес-логики. Приоритет + failover обеспечивает отказоустойчивость.

**Alternatives considered**:
- Единый HTTP-клиент с config-based mapping — слишком жёсткий, не покрывает SS7-провайдеров
- Отдельный hlr-service — нарушает Constitution VI (Simplicity), HLR lookup — часть routing domain

## R-002: HLR Cache Strategy

**Decision**: Redis с ключом `hlr:{msisdn}`, значение — JSON-сериализованный LookupResult, TTL 24h. Invalidation по событию `message.delivery_failed` с reason=wrong_operator.

**Rationale**: Redis уже используется в проекте для rate-limiting и sessions. Средняя MNP-частота < 1% в месяц, поэтому TTL 24h обеспечивает актуальность. Cache-aside pattern (check cache → miss → query provider → store) — стандартный для lookup-сценариев. Прогнозируемый hit rate ≥70% при стабильной нагрузке (повторные отправки на те же номера).

**Alternatives considered**:
- PostgreSQL materialized view — слишком медленно для синхронного lookup (нужен <50мс)
- In-memory cache (Go map) — не расшаривается между instances routing-service при горизонтальном масштабировании
- Redis + PostgreSQL двухуровневый кеш — избыточная сложность для текущего масштаба

## R-003: Smart Routing Scoring Algorithm

**Decision**: Взвешенный score = `cost_weight * normalized_cost + quality_weight * normalized_delivery_rate + availability_bonus`. Веса настраиваемые per-operator/per-region с defaults (cost_weight=0.6, quality_weight=0.4). Провайдер с наивысшим score выбирается. Недоступные провайдеры (health check failed) исключаются из кандидатов.

**Rationale**: Простая линейная модель достаточна для начального этапа. Normalized scores (0-1 range) обеспечивают сравнимость разных провайдеров. Настраиваемые веса позволяют оператору оптимизировать под конкретный рынок (где-то важнее цена, где-то — качество).

**Alternatives considered**:
- Фиксированный порог (delivery rate diff <5% → cheapest) — не гибко, не масштабируется
- ML-модель — overengineering для текущего этапа, недостаточно данных для обучения
- Multi-armed bandit — интересно для exploration, но добавляет непредсказуемость

## R-004: Синхронный Lookup в Send Pipeline

**Decision**: Синхронный lookup встраивается в routing-service между получением сообщения и выбором провайдера. При cache hit — добавляет <5мс. При cache miss — до 200мс (HLR provider latency). При timeout/failure — fallback на prefix-based routing.

**Rationale**: Подтверждено в clarification (синхронный режим). Cache hit rate ≥70% минимизирует реальное влияние на latency. Fallback гарантирует, что HLR-проблемы не блокируют отправку.

**Alternatives considered**:
- Асинхронный pre-lookup — сложнее (staging queue), не гарантирует актуальность
- Гибрид (sync cache hit, async miss) — добавляет сложность staging, оправдан только при очень высоких объёмах

## R-005: Standalone Lookup API Billing

**Decision**: Per-number тарификация через существующий tarification-service. Новый тип тарификации `lookup` рядом с существующими `fixed`/`threshold`/`prepaid`. Каждый номер в bulk-запросе — отдельная lookup charge. Cached результаты тоже тарифицируются (клиент платит за результат, не за HLR-запрос).

**Rationale**: Подтверждено в clarification (per-number). Использование tarification-service сохраняет единообразие ценообразования. Клиент платит за результат lookup, а не за факт HLR-запроса — это проще для биллинга и прозрачнее для клиента.

**Alternatives considered**:
- Бесплатно при отправке SMS (bundled) — теряется линия дохода от standalone lookup
- Тарификация только uncached запросов — непрозрачно, стоимость зависит от порядка запросов
- Отдельная billing таблица — дублирование, нарушает единообразие

## R-006: Lookup Log и PII Retention

**Decision**: Таблица `lookup_log` с monthly partitioning (как messages и audit_log). Автоматическое удаление партиций старше 90 дней. Поля: msisdn, operator (MCC+MNC), status, country, number_type, provider_id, source (sms_routing / api_lookup), client_id, cached (bool), request_id, created_at.

**Rationale**: Подтверждено в clarification (90 дней). Monthly partitioning — проектный стандарт для high-volume таблиц (Constitution V). Source field разделяет lookup при отправке SMS и standalone API запросы для аналитики.

**Alternatives considered**:
- Без лога (только кеш) — нет аудита, невозможна аналитика
- 365 дней — избыточно для PII, увеличивает storage costs
- Kafka event log только — не подходит для аудит-запросов с фильтрацией

## R-007: HLR Provider Health Monitoring

**Decision**: Активный health check с интервалом 30с (ping-запрос к провайдеру). Пассивный мониторинг: tracking success/failure rate per provider. Статусы: healthy (≥95% success), degraded (80-95%), unhealthy (<80% или timeout). Degraded провайдеры участвуют в selection с penalty к score. Unhealthy — исключаются.

**Rationale**: Аналогично существующему provider health в provider-service (success rate, last success/failure). Добавляем active ping для HLR-провайдеров, т.к. они внешние и могут быть недоступны.

**Alternatives considered**:
- Только passive monitoring — медленная реакция на полный outage провайдера
- Circuit breaker pattern — возможно в будущем, для MVP достаточно health check + status

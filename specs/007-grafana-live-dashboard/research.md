# Research: Live Load Test Dashboard

**Feature**: 007-grafana-live-dashboard | **Date**: 2026-03-26

## R-001: Datasource Architecture

**Decision**: Два datasource — Prometheus + PostgreSQL

**Rationale**: Метрики time-series (msg/s, латентность, queue size) уже собираются в Prometheus. Бизнес-данные (баланс, стоимость, лента сообщений, delivery rate по статусам) доступны только в PostgreSQL. Grafana нативно поддерживает оба datasource в одном дашборде.

**Alternatives considered**:
- Только Prometheus: Невозможно — баланс, стоимость, лента сообщений не экспортируются в Prometheus
- Только PostgreSQL: Потеря time-series данных (гистограммы, rate-функции)
- Добавить Prometheus-экспортёр для баланса: Избыточно — SQL-запрос проще и точнее

**Mapping данных по источникам**:

| Метрика | Источник | Запрос |
|---------|----------|--------|
| Баланс аккаунта | PostgreSQL | `SELECT balance FROM accounts WHERE client_id = ...` |
| Накопленная стоимость | PostgreSQL | `SELECT SUM(total_amount) FROM tarification_log WHERE ...` |
| Сообщения отправлено | Prometheus | `sum(smpp_messages_sent_total)` |
| Сообщения доставлено | PostgreSQL | `SELECT COUNT(*) FROM messages WHERE status='delivered'` |
| Сообщения не доставлено | Prometheus + PG | `sum(smpp_messages_failed_total)` / SQL fallback |
| Сообщения в очереди | Prometheus | `sum(sms_messages_queued_total)` |
| msg/s (throughput) | Prometheus | `sum(rate(smpp_messages_sent_total[1m]))` |
| Delivery rate % | PostgreSQL | `delivered / (delivered + failed) * 100` |
| Трафик по провайдерам | Prometheus | `sum by (provider_name)(smpp_messages_sent_total)` |
| Латентность провайдеров | Prometheus | `histogram_quantile(0.95, smpp_processing_duration_seconds)` |
| Лента сообщений | PostgreSQL | `SELECT ... FROM messages ORDER BY created_at DESC LIMIT N` |
| График баланса | PostgreSQL | Time-series запрос по `transactions` |
| График msg/s | Prometheus | `rate(smpp_messages_sent_total[1m])` |
| График delivery rate | PostgreSQL | Time-series по `aggregated_metrics` |

## R-002: Grafana Provisioning

**Decision**: Добавить полноценный provisioning через YAML-файлы

**Rationale**: Текущая конфигурация монтирует JSON-дашборды, но отсутствуют:
1. `datasources.yml` — определение datasources (Prometheus + PostgreSQL)
2. `dashboards.yml` — конфигурация провайдера дашбордов

Без `dashboards.yml` Grafana не загружает дашборды автоматически из смонтированной папки.

**Структура provisioning**:
```
deployments/configs/grafana/
├── provisioning/
│   ├── datasources/
│   │   └── datasources.yml      # Prometheus + PostgreSQL
│   └── dashboards/
│       └── dashboards.yml       # Файловый провайдер
└── dashboards/
    ├── overview.json            # Существующий
    ├── api-gateway.json         # Существующий
    ├── provider-health.json     # Существующий
    ├── worker.json              # Существующий
    └── load-test-live.json      # НОВЫЙ
```

**Alternatives considered**:
- Grafana API для создания дашборда: Не персистентно, теряется при пересоздании контейнера
- Helm/Terraform: Overengineering для Docker Compose

## R-003: Публичный доступ

**Decision**: Включить анонимный доступ к Grafana с ролью Viewer

**Rationale**: FR-001 требует публичный доступ без аутентификации. Grafana поддерживает это через environment variables:
```yaml
GF_AUTH_ANONYMOUS_ENABLED=true
GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer
```

Безопасность: допустимо согласно Assumptions — данные тестовые, не конфиденциальные.

**Alternatives considered**:
- Snapshot/embed: Ограничен по функциональности, не обновляется в реальном времени
- Публичный дашборд (Grafana Public Dashboard API): Требует enterprise или cloud, не подходит для self-hosted

## R-004: Обновление Prometheus конфигурации

**Decision**: Обновить `prometheus.yml` для текущей архитектуры сервисов

**Rationale**: Текущая конфигурация ссылается на устаревшие сервисы (`api-gateway-1/2`, `worker-1/2`). Актуальные сервисы и их порты метрик:

| Сервис | Контейнер | Порт метрик |
|--------|-----------|-------------|
| auth-service | auth-service | 2112 |
| messaging-service | messaging-service | 2113 |
| routing-service | routing-service | 2114 |
| provider-service | provider-service | 2115 |
| client-service | client-service | 2116 |
| analytics-service | analytics-service | 2117 |
| billing-service | billing-service | 2118 |
| webhook-service | webhook-service | 2119 |
| template-service | template-service | 2120 |
| tarification-service | tarification-service | 2121 |
| smpp-gateway | smpp-gateway | 2112 |
| client-gateway-1/2 | client-gateway-1/2 | 2112 |
| admin-gateway-1/2 | admin-gateway-1/2 | 2112 |

**Alternatives considered**:
- Service discovery: Не поддерживается в Docker Compose без дополнительных компонентов
- Оставить как есть: Метрики не будут собираться с текущих сервисов

## R-005: Маскирование номеров телефонов

**Decision**: Маскирование в SQL-запросе Grafana

**Rationale**: FR-014 требует маскирование номеров. PostgreSQL поддерживает строковые функции:
```sql
CONCAT(LEFT(destination, 3), '***', RIGHT(destination, 4))
-- Результат: +79***1234
```

Выполняется на уровне запроса, не требует изменений в коде Go.

**Alternatives considered**:
- View в PostgreSQL: Лишняя абстракция для одного запроса
- Go-эндпоинт для ленты: Overengineering — Grafana умеет запрашивать SQL напрямую

## R-006: Панели дашборда и типы визуализации

**Decision**: Использовать современные типы панелей Grafana вместо legacy `graph`

**Rationale**: Существующие дашборды используют `graph` (schemaVersion 16). Для нового дашборда используем актуальные типы:
- `stat` — для одиночных значений (баланс, delivery rate, msg/s)
- `timeseries` — для графиков с временной осью (замена `graph`)
- `table` — для ленты сообщений
- `bargauge` — для разбивки по провайдерам
- `gauge` — для процентных показателей

**schemaVersion**: 39 (актуальная для Grafana 10+)

## R-007: Отсутствующие метрики

**Decision**: Добавить `smpp_messages_delivered_total` в Prometheus

**Rationale**: В текущих метриках отсутствует счётчик доставленных сообщений. Есть `smpp_messages_sent_total` и `smpp_messages_failed_total`, но `delivered` отслеживается только в PostgreSQL. Для графиков delivery rate в реальном времени полезно иметь Prometheus-счётчик.

Однако для MVP дашборда можно обойтись PostgreSQL-запросами — `aggregated_metrics` таблица уже содержит агрегации по статусам.

**Alternatives considered**:
- Только PostgreSQL для delivery rate: Рабочий вариант, но менее эффективен для time-series графиков
- Экспортёр из PostgreSQL в Prometheus: Лишний компонент

## R-008: Интервал обновления

**Decision**: Refresh rate = 5s для дашборда

**Rationale**: Спецификация требует обновление каждые 5 секунд (FR-010). Prometheus scrape interval = 15s, но PostgreSQL-запросы обновляются мгновенно. Для Prometheus-панелей данные будут обновляться с задержкой до 15 секунд (один scrape interval), что допустимо по SC-003/SC-004 (один window обновления).

**Alternatives considered**:
- Уменьшить scrape interval до 5s: Увеличит нагрузку на все сервисы, не оправдано для демо-дашборда
- Использовать push-модель: Prometheus работает на pull, изменение архитектуры избыточно

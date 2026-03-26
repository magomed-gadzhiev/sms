# Tasks: Live Load Test Dashboard

**Input**: Design documents from `/specs/007-grafana-live-dashboard/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Не запрошены. Верификация — ручная (quickstart.md).

**Organization**: Задачи сгруппированы по user stories для независимой реализации и тестирования.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Можно выполнять параллельно (разные файлы, нет зависимостей)
- **[Story]**: К какой user story относится задача (US1–US5)
- Включены точные пути файлов

---

## Phase 1: Setup (Grafana Provisioning Infrastructure)

**Purpose**: Создать структуру директорий и provisioning-файлы для автоматической конфигурации Grafana

- [x] T001 [P] Create datasources provisioning file in deployments/configs/grafana/provisioning/datasources/datasources.yml per contracts/grafana-datasources.yml (Prometheus + PostgreSQL)
- [x] T002 [P] Create dashboards provisioning file in deployments/configs/grafana/provisioning/dashboards/dashboards.yml per contracts/grafana-dashboard-provisioning.yml (file provider)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Обновить инфраструктурные конфигурации — без этих изменений панели дашборда не смогут получать данные

**CRITICAL**: Нельзя начинать работу над dashboard JSON до завершения этой фазы

- [x] T003 [P] Replace deployments/configs/prometheus.yml with updated scrape targets per contracts/prometheus-scrape-config.yml (13 job definitions for current microservice architecture)
- [x] T004 [P] Update Grafana service in deployments/docker-compose.yml: add GF_AUTH_ANONYMOUS_ENABLED=true, GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer env vars; add provisioning volume mounts for datasources and dashboards; add postgres to depends_on per quickstart.md
- [x] T005 [P] Add smpp_messages_delivered_total CounterVec (labels: provider_id, provider_name) to internal/monitoring/metrics.go

**Checkpoint**: Инфраструктура готова — Grafana подключена к Prometheus и PostgreSQL, метрики собираются с актуальных сервисов

---

## Phase 3: User Story 1 — Наблюдение за балансом и финансами (Priority: P1)

**Goal**: Заказчик видит текущий баланс, накопленную стоимость и график изменения баланса во времени

**Independent Test**: Отправить 100 сообщений → баланс уменьшился на ожидаемую сумму → панель отобразила изменение

### Implementation for User Story 1

- [x] T006 [US1] Create dashboard scaffold in deployments/configs/grafana/dashboards/load-test-live.json: uid=load-test-live, title=Live Load Test, schemaVersion=39, refresh=5s, template variables ($client_id query from PostgreSQL, $time_range interval 5m/15m/30m/1h/2h default 30m)
- [x] T007 [US1] Add Row 1 "Finance" (y=1) to deployments/configs/grafana/dashboards/load-test-live.json: Current Balance stat panel (PostgreSQL: SELECT balance, currency FROM accounts WHERE client_id), Accumulated Cost stat panel (PostgreSQL: SUM(total_amount) FROM tarification_log), Balance Over Time timeseries panel (PostgreSQL: transactions.balance_after time-series)

**Checkpoint**: Дашборд открывается по http://localhost:3000/d/load-test-live/, отображает баланс и стоимость

---

## Phase 4: User Story 2 — Мониторинг потока и статусов сообщений (Priority: P1)

**Goal**: Заказчик видит счётчики сообщений по статусам, пропускную способность (msg/s) и delivery rate

**Independent Test**: Запустить тест на 1000 сообщений → счётчики соответствуют фактическому количеству → delivery rate корректен

### Implementation for User Story 2

- [x] T008 [US2] Add Row 2 "Messages" (y=10) to deployments/configs/grafana/dashboards/load-test-live.json: Messages Sent stat (Prometheus: sum(smpp_messages_sent_total)), Messages Delivered stat (PostgreSQL: COUNT WHERE status=delivered), Messages Failed stat (Prometheus: sum(smpp_messages_failed_total)), Messages Queued stat (Prometheus: sum(sms_messages_queued_total)) — each panel width=6
- [x] T009 [US2] Add Row 3 "Performance" (y=19) to deployments/configs/grafana/dashboards/load-test-live.json: Throughput msg/s stat (Prometheus: sum(rate(smpp_messages_sent_total[1m]))), Delivery Rate % gauge (PostgreSQL: delivered/(delivered+failed)*100 from messages table), Throughput Over Time timeseries (Prometheus: rate over time) — each panel width=8

**Checkpoint**: Счётчики Sent/Delivered/Failed/Queued обновляются каждые 5 секунд, delivery rate корректно рассчитан

---

## Phase 5: User Story 3 — Анализ работы провайдеров (Priority: P2)

**Goal**: Заказчик видит распределение трафика по провайдерам с латентностью и разбивку по операторам

**Independent Test**: Настроить 2+ провайдеров → запустить тест → трафик отображается для каждого провайдера с корректной латентностью

### Implementation for User Story 3

- [x] T010 [US3] Add Row 4 "Providers" (y=28) to deployments/configs/grafana/dashboards/load-test-live.json: Traffic by Provider bargauge (Prometheus: sum by (provider_name)(smpp_messages_sent_total), width=12), Provider Latency p95 timeseries (Prometheus: histogram_quantile(0.95, sum by (le, provider_name)(rate(smpp_processing_duration_seconds_bucket[5m]))), width=12)
- [x] T011 [US3] Add Row 5 "Operators" (y=37) to deployments/configs/grafana/dashboards/load-test-live.json: Traffic by Operator bargauge (PostgreSQL: messages grouped by destination prefix, width=12), Delivery Rate by Provider timeseries (PostgreSQL: aggregated_metrics grouped by provider_id, width=12)

**Checkpoint**: Панели провайдеров показывают распределение трафика и латентность для каждого провайдера

---

## Phase 6: User Story 4 — Лента последних сообщений (Priority: P2)

**Goal**: Заказчик видит живую ленту последних 20 сообщений с маскированными номерами

**Independent Test**: Отправить сообщения → проверить появление в ленте с корректными атрибутами и маскированными номерами

### Implementation for User Story 4

- [x] T012 [US4] Add Row 6 "Message Feed" (y=46) to deployments/configs/grafana/dashboards/load-test-live.json: Recent Messages table panel (PostgreSQL: last 20 messages with CONCAT(LEFT(destination,3),'***',RIGHT(destination,4)) masking, JOIN providers for name, columns: Time, Destination, Status, Provider, Latency(s), width=24)

**Checkpoint**: Лента показывает последние 20 сообщений с маскированными номерами, обновляется каждые 5 секунд

---

## Phase 7: User Story 5 — Исторические графики (Priority: P3)

**Goal**: Заказчик видит графики msg/s, delivery rate и латентности за сессию тестирования

**Independent Test**: Запустить 10-минутный тест → графики корректно отображают историю за весь период

### Implementation for User Story 5

- [x] T013 [US5] Add Row 7 "Historical Graphs" (y=55) to deployments/configs/grafana/dashboards/load-test-live.json: msg/s History timeseries (Prometheus: sum(rate(smpp_messages_sent_total[1m])), width=12), Delivery Rate & Latency overlay timeseries (Prometheus p95 latency + PostgreSQL delivery rate from aggregated_metrics, width=12)
- [x] T014 [US5] Add Balance Zero annotation to deployments/configs/grafana/dashboards/load-test-live.json: datasource=PostgreSQL, query=SELECT created_at FROM transactions WHERE balance_after <= 0, color=red

**Checkpoint**: Графики показывают временные ряды за выбранный период, аннотация Balance Zero отображается при нулевом балансе

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Финальная верификация и валидация

- [x] T015 Validate load-test-live.json is valid JSON with consistent panel IDs and no duplicate grid positions in deployments/configs/grafana/dashboards/load-test-live.json
- [x] T016 Run quickstart.md validation: docker compose up -d grafana prometheus → verify dashboard loads at http://localhost:3000/d/load-test-live/ → verify all panels show data or "No data" (not errors)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Нет зависимостей — можно начинать сразу
- **Foundational (Phase 2)**: Зависит от Setup (provisioning files должны существовать до обновления docker-compose) — БЛОКИРУЕТ все user stories
- **US1 (Phase 3)**: Зависит от Foundational — создаёт файл load-test-live.json
- **US2 (Phase 4)**: Зависит от US1 (dashboard scaffold must exist)
- **US3 (Phase 5)**: Зависит от US2 (добавляет rows к существующему JSON)
- **US4 (Phase 6)**: Зависит от US3 (последовательное добавление rows)
- **US5 (Phase 7)**: Зависит от US4 (последовательное добавление rows)
- **Polish (Phase 8)**: Зависит от завершения всех user stories

### Constraint: Single Dashboard File

Все панели находятся в одном файле `load-test-live.json`. Задачи T006–T014 модифицируют один и тот же файл и **не могут выполняться параллельно**. Однако это компенсируется тем, что Phase 1 и Phase 2 содержат полностью параллельные задачи (разные файлы).

### Within Each User Story

- Scaffold (metadata, variables) → перед любыми панелями
- Rows добавляются последовательно в порядке y-координат

### Parallel Opportunities

- **Phase 1**: T001 и T002 — параллельно (разные файлы)
- **Phase 2**: T003, T004, T005 — параллельно (разные файлы: prometheus.yml, docker-compose.yml, metrics.go)
- **Phase 3–7**: Последовательно (один файл: load-test-live.json)

---

## Parallel Example: Phase 2 (Foundational)

```bash
# Launch all foundational tasks together (different files):
Task: "Replace prometheus.yml with updated scrape targets"
Task: "Update docker-compose.yml Grafana service configuration"
Task: "Add smpp_messages_delivered_total to metrics.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (2 tasks, parallel)
2. Complete Phase 2: Foundational (3 tasks, parallel)
3. Complete Phase 3: US1 — Balance & Finance
4. Complete Phase 4: US2 — Message Counters & Throughput
5. **STOP and VALIDATE**: Дашборд показывает баланс, стоимость, счётчики, msg/s, delivery rate
6. Демо заказчику — основные показатели реальности данных

### Incremental Delivery

1. Setup + Foundational → Инфраструктура готова
2. US1 → Баланс и финансы → Демо (MVP минимум!)
3. US2 → Счётчики и пропускная способность → Демо (полный MVP)
4. US3 → Провайдеры и маршруты → Демо
5. US4 → Лента сообщений → Демо
6. US5 → Исторические графики → Демо (полная версия)

---

## Notes

- [P] tasks = разные файлы, нет зависимостей
- [Story] label привязывает задачу к user story для трассировки
- Все панели используют schemaVersion=39 (Grafana 10+)
- Типы панелей: stat, timeseries, gauge, bargauge, table (не legacy graph)
- Refresh interval = 5s для всего дашборда
- Маскирование номеров выполняется в SQL: CONCAT(LEFT(destination,3),'***',RIGHT(destination,4))
- PostgreSQL datasource: read-only, макс 5 соединений
- Prometheus scrape_interval остаётся 15s — данные Prometheus обновляются с задержкой до 15 секунд

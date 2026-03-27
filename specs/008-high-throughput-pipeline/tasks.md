# Tasks: High-Throughput Pipeline Architecture

**Input**: Design documents from `/specs/008-high-throughput-pipeline/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Не запрошены в спецификации. Тестовые задачи не включены. Load-тесты включены в фазу Polish.

**Organization**: Задачи сгруппированы по user stories для независимой реализации и тестирования каждой истории.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Можно выполнять параллельно (разные файлы, нет зависимостей)
- **[Story]**: К какой user story относится задача (US1, US2, US3, US4, US5)
- Все пути файлов указаны от корня репозитория

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Создание структуры проекта и базовых типов для pipeline-worker

- [X] T001 Create pipeline directory structure: `cmd/pipeline-worker/`, `internal/pipeline/router/`, `internal/pipeline/sender/`, `internal/pipeline/status/`, `internal/pipeline/backpressure/`, `internal/pipeline/batch/`
- [X] T002 [P] Define Kafka inter-stage message Go structs (RoutedMessage, SentMessage, StatusUpdate) in `internal/pipeline/messages.go` per contracts/kafka-schemas.md
- [X] T003 [P] Add pipeline configuration struct and viper bindings (stage, batch_size, batch_timeout, worker_count, smpp_window_size) in `internal/config/config.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Рефакторинг существующих пакетов и создание инфраструктурных утилит, необходимых ВСЕМ стадиям pipeline

**⚠️ CRITICAL**: Работа над user stories невозможна до завершения этой фазы

- [X] T004 [P] Refactor `internal/queue/producer.go` — add sarama AsyncProducer support alongside existing SyncProducer (Flush.Messages=500, Flush.Frequency=10ms, Compression=Snappy) per R-002
- [X] T005 [P] Refactor `internal/queue/consumer.go` — add batch consumption mode (collect messages by size/timer), configurable BalanceStrategy parameter per R-002
- [X] T006 [P] Implement BatchAccumulator in `internal/pipeline/batch/batcher.go` — accumulate messages by max_size (500) or max_wait (10ms), flush via channel per data-model.md BatchAccumulator spec
- [X] T007 [P] Register pipeline Prometheus metrics (pipeline_messages_processed_total, pipeline_processing_duration_seconds, pipeline_batch_size, pipeline_queue_depth, pipeline_backpressure_active, pipeline_connections_active) in `internal/monitoring/metrics.go` per R-009
- [X] T008 Implement `cmd/pipeline-worker/main.go` — parse `--stage` flag (router|sender|status), load config via viper, initialize DB pool, Kafka clients, metrics HTTP server, dispatch to selected stage per R-001

**Checkpoint**: Инфраструктура готова — можно начинать реализацию user stories

---

## Phase 3: User Story 1 — Массовая отправка SMS через API (Priority: P1) 🎯 MVP

**Goal**: Полный pipeline от приёма сообщения до передачи оператору-симулятору со скоростью 10 000+ msg/sec при 4 экземплярах sender

**Independent Test**: Отправить 100 000 сообщений через API, убедиться что все маршрутизированы и переданы оператору-симулятору. Измерить throughput ≥ 10 000 msg/sec

### Implementation for User Story 1

- [X] T009 [P] [US1] Implement router stage in `internal/pipeline/router/stage.go` — consume from sms.outgoing and sms.failed topics, call existing `internal/router/router.go` RouteMessage(), set primary + fallback provider, produce RoutedMessage to sms.routed per R-007
- [X] T010 [P] [US1] Refactor `internal/smsc/sender.go` — add async SMPP SubmitSM with sliding window (window_size=50), separate writer/reader goroutines, pending responses map by sequence number per R-005
- [X] T011 [P] [US1] Refactor `internal/smsc/pool.go` — add AsyncConnection struct with writer_ch, pending_responses sync.Map, window_semaphore per data-model.md AsyncConnection spec
- [X] T012 [P] [US1] Implement backpressure manager in `internal/pipeline/backpressure/manager.go` — token bucket per provider_id (rate=throughput_per_second, burst=×2), is_throttled flag, pending_count tracking per R-006
- [X] T013 [US1] Implement sender stage in `internal/pipeline/sender/stage.go` — consume RoutedMessage from sms.routed, apply backpressure check, send via async SMPP pool, produce SentMessage to sms.sent (depends on T010, T011, T012)
- [X] T014 [US1] Add failover logic to sender stage in `internal/pipeline/sender/stage.go` — on primary send failure: try fallback_provider_id from metadata, on both fail: publish to sms.failed with retry_count++ (max 3 retries) per R-007
- [X] T015 [P] [US1] Implement status writer stage in `internal/pipeline/status/stage.go` — consume SentMessage from sms.sent, batch upsert message status to DB via `INSERT ... ON CONFLICT (id) DO UPDATE` per data-model.md
- [X] T016 [US1] Wire all three stages into `cmd/pipeline-worker/main.go` — stage selection dispatch, context-based lifecycle, os.Signal graceful shutdown with in-flight message drain

**Checkpoint**: Pipeline полностью функционален. Сообщения проходят путь sms.outgoing → router → sms.routed → sender → sms.sent → status writer → DB. Можно тестировать с SMPP-симулятором.

---

## Phase 4: User Story 2 — Горизонтальное масштабирование (Priority: P2)

**Goal**: Добавление экземпляров pipeline-worker автоматически увеличивает throughput. Выход экземпляра не теряет сообщения.

**Independent Test**: Запустить 1 sender, измерить throughput, добавить ещё 3 — throughput должен вырасти пропорционально (±20%)

### Implementation for User Story 2

- [X] T017 [US2] Configure CooperativeStickyAssignor for all pipeline consumer groups (pipeline-router, pipeline-sender, pipeline-status) in `internal/queue/consumer.go` per R-004
- [X] T018 [US2] Implement graceful shutdown with partition drain — wait for in-flight batches to complete, commit final offsets before leaving consumer group in `cmd/pipeline-worker/main.go`
- [X] T019 [P] [US2] Create `deployments/docker/pipeline-worker.Dockerfile` — multi-stage build for pipeline-worker binary
- [X] T020 [US2] Update `deployments/docker-compose.yml` — add pipeline-router, pipeline-sender (×4 replicas), pipeline-status services replacing old worker, configure Kafka topic auto-creation
- [X] T021 [US2] Implement Kafka topic auto-creation on first startup in `cmd/pipeline-worker/main.go` — create sms.routed (16 partitions), sms.sent (16 partitions), sms.status (8 partitions) via sarama ClusterAdmin per R-003

**Checkpoint**: Pipeline масштабируется горизонтально. `docker-compose up -d --scale pipeline-sender=4` увеличивает throughput пропорционально. Аварийное завершение одного экземпляра → rebalance < 5 секунд, 0 потерь.

---

## Phase 5: User Story 3 — Мониторинг throughput в реальном времени (Priority: P2)

**Goal**: Оператор видит throughput, latency и глубину очередей по каждой стадии pipeline в Grafana

**Independent Test**: Подать нагрузку и проверить что метрики всех стадий отображаются в дашборде с задержкой ≤15 секунд

### Implementation for User Story 3

- [X] T022 [P] [US3] Instrument router stage in `internal/pipeline/router/stage.go` — emit pipeline_messages_processed_total{stage="router"}, pipeline_processing_duration_seconds{stage="router"}, pipeline_batch_size{stage="router"}
- [X] T023 [P] [US3] Instrument sender stage in `internal/pipeline/sender/stage.go` — emit metrics with stage="sender", plus pipeline_backpressure_active{provider_id}, pipeline_connections_active{provider_id}
- [X] T024 [P] [US3] Instrument status writer in `internal/pipeline/status/stage.go` — emit metrics with stage="status"
- [X] T025 [US3] Implement consumer lag monitoring via sarama ClusterAdmin in `internal/monitoring/metrics.go` — poll topic offsets vs committed offsets, export as pipeline_queue_depth{topic, partition}
- [X] T026 [P] [US3] Create Grafana dashboard JSON in `deployments/configs/grafana/dashboards/pipeline.json` — panels: per-stage throughput, per-stage latency (p50/p95/p99), queue depth, backpressure status, active connections
- [X] T027 [US3] Update `deployments/configs/prometheus.yml` — add scrape targets for pipeline-router, pipeline-sender, pipeline-status instances

**Checkpoint**: Grafana дашборд показывает throughput, latency и queue depth по каждой стадии в реальном времени. Замедление одной стадии видно на дашборде.

---

## Phase 6: User Story 4 — Асинхронное получение статусов доставки (Priority: P3)

**Goal**: DLR от операторов обрабатываются асинхронно, статусы доступны через API в течение 30 секунд

**Independent Test**: Отправить 10 000 сообщений через симулятор, дождаться DLR, проверить все статусы доступны через API

### Implementation for User Story 4

- [X] T028 [US4] Add sms.dlr topic consumption to status writer stage in `internal/pipeline/status/stage.go` — parse DLRMessage, map dlr_stat (DELIVRD→delivered, UNDELIV→failed, EXPIRED→expired) to StatusUpdate
- [X] T029 [US4] Implement idempotent status upsert in `internal/pipeline/status/stage.go` — `INSERT ... ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at WHERE messages.updated_at < EXCLUDED.updated_at` to handle duplicate DLRs
- [X] T030 [US4] Implement write-ahead buffer in `internal/pipeline/status/stage.go` — when DB write fails, buffer StatusUpdate in memory, retry with exponential backoff, continue consuming from Kafka without blocking

**Checkpoint**: DLR от операторов корректно обновляют статусы сообщений. Дубли DLR обрабатываются идемпотентно. При недоступности БД статусы буферизуются и записываются после восстановления.

---

## Phase 7: User Story 5 — Пакетная отправка (Priority: P3)

**Goal**: Batch API использует AsyncProducer для эффективной публикации пакетов сообщений в Kafka

**Independent Test**: Отправить batch из 1 000 сообщений и сравнить время обработки с 1 000 индивидуальных запросов

### Implementation for User Story 5

- [X] T031 [US5] Optimize batch message publishing in client-gateway — replace SyncProducer.SendMessage() loop with AsyncProducer.Input() batch flush in `internal/gateway/client/handlers/` (messaging handler) per R-008
- [X] T032 [US5] Optimize batch message publishing in API HTTP handler — same AsyncProducer optimization in `internal/api/http/handlers.go` per R-008

**Checkpoint**: Batch API публикует 1 000 сообщений одним вызовом AsyncProducer вместо 1 000 отдельных SyncProducer.SendMessage(). Latency batch-запроса значительно ниже суммы индивидуальных запросов.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Финализация, load-тесты, валидация, обеспечение rollback

- [X] T033 [P] Create pipeline-specific load test in `test/load/pipeline_load_test.go` — 10K msg/sec target, measure end-to-end throughput с 4 sender instances
- [X] T034 [P] Update existing `test/load/api_load_test.go` — add batch API AsyncProducer benchmark comparison
- [X] T035 Run quickstart.md validation — verify all steps (topic creation, build, stage startup, health checks, metrics, load test) work end-to-end
- [X] T036 Verify rollback path — confirm old `cmd/worker/main.go` still builds and processes messages when pipeline-worker is stopped

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Нет зависимостей — можно начинать немедленно
- **Foundational (Phase 2)**: Зависит от Phase 1 — **БЛОКИРУЕТ** все user stories
- **US1 (Phase 3)**: Зависит от Phase 2 — MVP, реализуется первой
- **US2 (Phase 4)**: Зависит от Phase 2. Может выполняться параллельно с US1, но рекомендуется после US1 (нужен работающий pipeline для проверки масштабирования)
- **US3 (Phase 5)**: Зависит от Phase 2. Может выполняться параллельно с US1 (инструментация стадий), но финализация после US1
- **US4 (Phase 6)**: Зависит от Phase 3 (US1) — расширяет status writer stage
- **US5 (Phase 7)**: Зависит от Phase 2 (T004 — AsyncProducer). Может выполняться параллельно с US1
- **Polish (Phase 8)**: Зависит от завершения всех желаемых user stories

### User Story Dependencies

- **US1 (P1)**: После Phase 2. Независима. **MVP scope.**
- **US2 (P2)**: После Phase 2. Рекомендуется после US1 для полноценного тестирования масштабирования
- **US3 (P2)**: После Phase 2. Параллельна с US1/US2 (метрики можно добавлять в стадии по мере их реализации)
- **US4 (P3)**: После US1 (расширяет status writer). Независима от US2/US3/US5
- **US5 (P3)**: После Phase 2 (T004). Независима от US1/US2/US3/US4

### Within Each User Story

- Модели/типы → сервисы/логика → интеграция
- Независимые файлы (помечены [P]) можно реализовывать параллельно
- Задачи без [P] имеют зависимости от предыдущих задач в той же фазе

### Parallel Opportunities

**Phase 2**: T004, T005, T006, T007 — все параллельны (разные файлы)

**Phase 3 (US1)**: T009, T010, T011, T012, T015 — параллельны (разные файлы). T013 ждёт T010+T011+T012. T014 ждёт T013. T016 ждёт все.

**Phase 4 (US2)**: T019 параллельна с T017/T018

**Phase 5 (US3)**: T022, T023, T024, T026 — параллельны. T025 независима. T027 после T020 (нужны имена сервисов из docker-compose)

---

## Parallel Example: User Story 1

```text
# Волна 1 — параллельные задачи (разные файлы):
Agent 1: T009 — Router stage в internal/pipeline/router/stage.go
Agent 2: T010 — Async SMPP sender в internal/smsc/sender.go
Agent 3: T011 — AsyncConnection pool в internal/smsc/pool.go
Agent 4: T012 — Backpressure manager в internal/pipeline/backpressure/manager.go
Agent 5: T015 — Status writer stage в internal/pipeline/status/stage.go

# Волна 2 — после завершения T010, T011, T012:
Agent 1: T013 — Sender stage в internal/pipeline/sender/stage.go

# Волна 3 — после T013:
Agent 1: T014 — Failover logic в internal/pipeline/sender/stage.go

# Волна 4 — после всех задач US1:
Agent 1: T016 — Wiring в cmd/pipeline-worker/main.go
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T008) — **CRITICAL**, блокирует все stories
3. Complete Phase 3: User Story 1 (T009-T016)
4. **STOP и VALIDATE**: Запустить pipeline-worker в 3 режимах (router, sender, status), отправить сообщения через API, проверить прохождение через все стадии до оператора-симулятора
5. Deploy/demo если готово

### Incremental Delivery

1. Setup + Foundational → Инфраструктура готова
2. US1 → Тест с симулятором → Deploy (MVP!)
3. US5 → Batch оптимизация → Deploy (быстрый win, малый scope)
4. US2 → Масштабирование + Docker → Deploy
5. US3 → Мониторинг + Grafana → Deploy
6. US4 → DLR + статусы → Deploy
7. Polish → Load tests + rollback verification

### Parallel Team Strategy

С несколькими разработчиками после завершения Phase 1+2:

- Developer A: US1 (core pipeline) — **приоритет**
- Developer B: US5 (batch optimization) — малый scope, можно параллельно
- Developer C: US3 (metrics/monitoring) — инструментация стадий по мере их реализации
- После завершения US1: Developer A → US2 (scaling), Developer B → US4 (DLR/statuses)

---

## Notes

- [P] задачи = разные файлы, нет зависимостей — можно запускать параллельными агентами
- [Story] label связывает задачу с user story для трaceability
- Каждая user story независимо завершаема и тестируема
- Старый `cmd/worker/` сохраняется для rollback — НЕ УДАЛЯТЬ
- Commit после каждой задачи или логической группы
- Stop на любом checkpoint для валидации story
- Partition key = provider_id для sms.routed/sms.sent обеспечивает per-operator locality

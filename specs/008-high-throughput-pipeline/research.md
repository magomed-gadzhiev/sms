# Research: High-Throughput Pipeline Architecture

**Feature**: 008-high-throughput-pipeline
**Date**: 2026-03-26
**Status**: Complete

## R-001: Pipeline Architecture — Single Binary vs Separate Services

**Decision**: Единый бинарник `pipeline-worker` с флагом `--stage=router|sender|status`

**Rationale**:
- Конституция (Simplicity §VI) требует обосновать создание новых сервисов. Единый бинарник — минимальная сложность деплоя
- Один Docker-образ, одна конфигурация, разные `CMD` в docker-compose
- Каждая стадия запускает только нужный Kafka consumer group — изоляция масштабирования сохраняется
- Go build tags не нужны — выбор стадии через runtime-конфигурацию

**Alternatives considered**:
- Отдельные сервисы (cmd/router-worker, cmd/sender-worker, cmd/status-writer): излишняя сложность деплоя, дублирование кода инициализации (DB, Kafka, metrics)
- Горизонтальное масштабирование монолитного worker: масштабирует routing и sending одинаково, а bottleneck в SMPP sending (I/O bound) требует непропорционального масштабирования

## R-002: Kafka Batch Processing для 10K msg/sec

**Decision**: sarama AsyncProducer + batch consumer с настраиваемым batch size

**Rationale**:
- **AsyncProducer** вместо SyncProducer: SyncProducer блокирует на каждом сообщении (~1-5ms per msg = max 200-1000 msg/sec). AsyncProducer буферизует и отправляет batches
- **Consumer batch processing**: sarama ConsumerGroupHandler.ConsumeClaim() читает из канала Messages() — собираем batch вручную по таймеру или размеру
- **Оптимальные настройки Producer**:
  - `Flush.Messages = 500` — отправлять batch каждые 500 сообщений
  - `Flush.Frequency = 10ms` — или каждые 10ms (что раньше)
  - `Flush.Bytes = 1048576` (1MB) — или по размеру
  - `RequiredAcks = WaitForLocal` — баланс надёжности и скорости
  - `Compression = Snappy` — снижает network I/O
- **Оптимальные настройки Consumer**:
  - `Fetch.Min = 1` (default) — не ждать batch на стороне broker
  - `Fetch.Default = 1048576` (1MB) — fetch size per partition
  - `MaxProcessingTime = 500ms` — допуск на batch обработку
  - `ChannelBufferSize = 1024` — буфер канала сообщений

**Alternatives considered**:
- SyncProducer: проще, но throughput ограничен ~1000 msg/sec
- sarama-cluster (deprecated): устаревшая библиотека
- confluent-kafka-go (librdkafka): быстрее, но CGO dependency ломает простоту сборки

## R-003: Kafka Partition Strategy

**Decision**: 16 партиций для sms.routed и sms.sent, 8 для sms.status. Partition key = provider_id

**Rationale**:
- **16 партиций для sms.routed/sms.sent**: при целевых 4 sender-инстансах = 4 партиции на инстанс. Запас для масштабирования до 8 инстансов
- **8 партиций для sms.status**: status writer менее нагружен (DB batch insert), 8 партиций достаточно
- **Partition key = provider_id**: сообщения для одного оператора попадают на один consumer → упрощает per-operator rate limiting и backpressure. Сохраняет ordering per-provider
- **Существующий sms.outgoing**: увеличить с 1 до 16 партиций (partition key = hash от message_id для равномерного распределения)
- **Topic creation**: добавить Kafka admin client для создания топиков с правильным количеством партиций при старте

**Alternatives considered**:
- Partition key = destination_prefix: неравномерное распределение (80% трафика может идти на один оператор)
- Partition key = message_id: равномерно, но теряется per-operator locality
- 32+ партиций: overhead на metadata и consumer rebalancing при текущих масштабах

## R-004: Consumer Group Rebalancing

**Decision**: CooperativeStickyAssignor для pipeline consumer groups

**Rationale**:
- Текущий RoundRobin вызывает stop-the-world rebalancing при добавлении/удалении инстанса — все consumers останавливаются на время rebalance
- CooperativeStickyAssignor (sarama.NewBalanceStrategySticky() + Cooperative protocol):
  - Инкрементальный rebalance — только перемещаемые партиции останавливаются
  - Sticky — минимизирует перемещение партиций между consumers
  - Типичное время rebalance: <2 секунды вместо 5-10 секунд
- Каждая стадия pipeline — отдельный consumer group:
  - `pipeline-router` для sms.outgoing → sms.routed
  - `pipeline-sender` для sms.routed → sms.sent
  - `pipeline-status` для sms.sent + sms.dlr → DB

**Alternatives considered**:
- RangeAssignor: неравномерное распределение при небольшом количестве consumers
- Сохранить RoundRobin: stop-the-world rebalance неприемлем для 10K msg/sec SLA

## R-005: SMPP Async Sending и Windowing

**Decision**: Async PDU sending с sliding window (window size = 50 per connection)

**Rationale**:
- **Текущая проблема**: `sender.go` отправляет SubmitSM и блокируется на read response. При 5ms RTT = max 200 PDU/sec per connection
- **Async sending**: отправляем PDU, не ждём ответ. Отдельная горутина читает ответы и матчит по sequence number
  - Структура: `pendingResponses map[uint32]chan *SubmitSMResp` — ожидающие ответы по sequence number
  - Writer горутина: пишет PDU в connection
  - Reader горутина: читает ответы, маршрутизирует в каналы
- **Sliding window = 50**: типичное значение для SMPP. Отправляем до 50 PDU без ожидания ответа
  - При 5ms RTT: 50 / 0.005 = 10 000 PDU/sec per connection
  - 2 connections per provider с window 50 = достаточно для 10K msg/sec на один оператор
- **Sequence number management**: atomic counter per connection, wraparound на 0x7FFFFFFF

**Alternatives considered**:
- Увеличение количества connections без windowing: 50 connections × 200 PDU/sec = 10K, но 50 TCP connections per provider — нетипично и может быть отклонено оператором
- Window size 100+: риск overflow у оператора, типичные лимиты 20-100
- Отдельная библиотека SMPP (go-smpp): потеря контроля над custom protocol implementation

## R-006: Backpressure Mechanism

**Decision**: Token bucket per-operator + Kafka consumer pause/resume

**Rationale**:
- **Token bucket** (уже частично реализован в `pool.go` как `Throttler`):
  - Каждый оператор имеет `throughput_per_second` в конфигурации
  - Token bucket rate = throughput_per_second, burst = throughput_per_second × 2
  - Когда tokens исчерпаны → сообщения для этого оператора буферизуются
- **Kafka consumer pause/resume**:
  - Когда буфер для оператора превышает порог → pause consumption с партиций этого оператора
  - sarama: `ConsumerGroupSession.PausePartitions()` — нет в sarama, нужно реализовать через канал
  - Альтернатива: замедление commit offset — consumer продолжает читать, но не коммитит offset для перегруженных партиций
- **Per-operator isolation**: partition key = provider_id гарантирует, что backpressure одного оператора не влияет на другие

**Alternatives considered**:
- Глобальный rate limiter: блокирует всех операторов при перегрузке одного
- Queue depth monitoring только: reactive, не proactive — задержка между обнаружением и реакцией
- Redis-based rate limiter: дополнительный network hop, увеличивает latency

## R-007: Failover Strategy в Pipeline

**Decision**: Failover на стадии router с retry на стадии sender

**Rationale**:
- **Router stage**: определяет primary и fallback provider. Записывает оба в routed message metadata
- **Sender stage**: при неудаче отправки на primary provider:
  1. Проверяет наличие fallback provider в metadata
  2. Если есть — отправляет на fallback без возврата в router
  3. Если нет или fallback тоже failed — публикует в sms.failed topic с retry_count++
  4. Router consumer обрабатывает sms.failed: выбирает новый маршрут и re-publishes в sms.routed
- **Retry logic**: max 3 retries (configurable), exponential backoff

**Alternatives considered**:
- Failover только на router: каждый retry проходит полный цикл router → sender, увеличивает latency
- Failover только на sender: sender не знает routing rules, ограничен fallback из metadata
- Отдельный retry-service: излишняя сложность для текущих требований

## R-008: Existing Batch API Status

**Decision**: Batch API уже реализован — доработка не требуется

**Rationale**:
- HTTP endpoint `/api/v1/sms/batch` полностью реализован в client-gateway и API gateway
- gRPC `SendBatch` в messaging.proto и sms.proto определён и реализован
- Поддержка до 10 000 сообщений в batch (спец FR-002)
- Индивидуальная валидация с partial success (995 ok + 5 failed)
- Load test существует в `test/load/api_load_test.go`
- **Единственное изменение**: оптимизация gateway handler — публикация batch сообщений в Kafka одним вызовом AsyncProducer вместо поштучного SyncProducer.SendMessage()

## R-009: Per-Stage Metrics

**Decision**: Расширение существующих Prometheus-метрик с pipeline stage label

**Rationale**:
- Существующие метрики (`worker_messages_processed_total`, `worker_processing_duration_seconds`) не различают стадии
- Новые метрики с label `stage`:
  - `pipeline_messages_processed_total{stage, status}` — Counter
  - `pipeline_processing_duration_seconds{stage}` — Histogram (p50, p95, p99)
  - `pipeline_batch_size{stage}` — Histogram размера batch
  - `pipeline_queue_depth{topic, partition}` — Gauge глубины очереди (consumer lag)
  - `pipeline_backpressure_active{provider_id}` — Gauge (0/1) backpressure status
  - `pipeline_connections_active{provider_id}` — Gauge активных SMPP connections
- **Consumer lag**: через sarama admin client — разница между latest offset и committed offset

**Alternatives considered**:
- Отдельные метрики для каждой стадии (router_processed_total, sender_processed_total): менее гибко для Grafana queries
- OpenTelemetry вместо Prometheus: breaking change, не оправдан текущими требованиями

## R-010: Current Worker Architecture Analysis

**Decision**: Основа для pipeline-worker — рефакторинг, не переписывание

**Rationale**:
Текущий worker (`cmd/worker/main.go`) выполняет:
1. Инициализация: config → DB pool → repositories → SMSC pool → tarification gRPC client → Kafka consumer
2. Потребление: 3 горутины для sms.outgoing, sms.dlr, sms.failed
3. Обработка sms.outgoing: deserialize → Router.RouteMessage() → Pool.getConnection() → Sender.SendMessage() → update DB status → tarification
4. Graceful shutdown: context.WithCancel + os.Signal

**Что переиспользуется**:
- `internal/router/router.go` — логика маршрутизации (as-is для router stage)
- `internal/smsc/pool.go` — connection pool (с рефакторингом для async)
- `internal/smsc/sender.go` — SMPP sending (с рефакторингом для windowing)
- `internal/queue/consumer.go` — Kafka consumer (с рефакторингом для batch + sticky)
- `internal/queue/producer.go` — Kafka producer (замена на AsyncProducer)

**Что добавляется**:
- `cmd/pipeline-worker/main.go` — entry point с выбором стадии
- `internal/pipeline/router/stage.go` — router stage logic
- `internal/pipeline/sender/stage.go` — sender stage logic
- `internal/pipeline/status/stage.go` — status writer stage logic
- `internal/pipeline/backpressure/manager.go` — per-operator backpressure
- `internal/pipeline/batch/batcher.go` — batch accumulator (by timer + size)

**Конфигурация pipeline-worker**:
```yaml
pipeline:
  stage: "router"           # router | sender | status
  batch_size: 500           # messages per batch
  batch_timeout: "10ms"     # max wait for batch
  worker_count: 4           # goroutines per stage
```

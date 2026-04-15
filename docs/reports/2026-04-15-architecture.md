# Architecture Analysis Report — SMS Platform

**Дата:** 2026-04-15
**Scope:** all (вся платформа)
**Mode:** auto
**Action:** recommend

---

## [Summary]

SMS-платформа имеет **зрелую микросервисную архитектуру** с 15 доменными сервисами, 4 gateway'ами и высокопроизводительным 4-стадийным Kafka-pipeline. Сильные стороны: хорошая DDD-декомпозиция сервисов с изолированными Go-пакетами, продуманный pipeline с batch-обработкой и backpressure, обширная Prometheus-инструментация (70+ метрик), консистентная propagation request_id через HTTP/gRPC/Kafka. **Главные риски:** единая разделяемая БД (smpp_db) и Redis — фундаментальное ограничение масштабируемости и изоляции; полное отсутствие circuit breaker'ов создаёт угрозу каскадных отказов; нет alerting-инфраструктуры — мониторинг работает только в режиме "dashboard watching".

---

## [Service Map]

```
                                    ┌─────────────────────────────────────────────────────────────────┐
                                    │                         HAProxy (:8080-8083)                     │
                                    │   LB for: client-gw×2, admin-gw×2, portal-gw, portal-frontend  │
                                    └────┬──────────┬──────────────┬──────────────┬───────────────────┘
                                         │          │              │              │
                              ┌──────────▼─┐  ┌────▼──────┐  ┌───▼────────┐  ┌──▼──────────────┐
                              │client-gw×2 │  │admin-gw×2 │  │portal-gw   │  │portal-frontend  │
                              │HTTP :8080  │  │HTTP :8081  │  │HTTP :8082  │  │Nginx :80 (SPA)  │
                              │gRPC :9090  │  │gRPC :9091  │  │+Kafka prod │  └─────────────────┘
                              └─────┬──────┘  └─────┬──────┘  │+Kafka cons │
                                    │               │         │+pgxpool    │
                                    │               │         └─────┬──────┘
                 ┌──────────────────┴───────────────┴───────────────┘
                 │  gRPC calls to backend services
                 ▼
    ┌────────────────────────────────────────────────────────────────────────────────────────────────┐
    │  Backend Services (all gRPC, all → shared PostgreSQL smpp_db)                                  │
    │                                                                                                │
    │  auth-service (:9101) ──gRPC──→ client-service (:9095)                                        │
    │  messaging-service (:9092) ──Kafka──→ sms.outgoing                                            │
    │  routing-service (:9093) ──gRPC──→ provider-service (:9094)                                   │
    │  analytics-service (:9096) ←──Kafka──── sms.outgoing/dlr/failed                               │
    │  billing-service (:9097) ←──Kafka──── sms.dlr/failed                                          │
    │  tarification-service (:9100) ──gRPC──→ billing-service, routing-service                      │
    │  cascade-service (:9110) ──gRPC──→ routing, tarification, billing  ──Kafka──→ cascade.*       │
    │  webhook-service (:9098) ←──Kafka──── sms.dlr/failed                                          │
    │  template-service (:9099) [+SenderName +Company gRPC]                                         │
    │  audit-service (:9102)    contact-service (:5012)    campaign-service (:5013)                  │
    │  link-service (:9103)                                                                          │
    └────────────────────────────────────────────────────────────────────────────────────────────────┘
                 │
                 │  Kafka topics
                 ▼
    ┌────────────────────────────────────────────────────────────────────────────────────────────────┐
    │  Pipeline (Kafka-based, multi-replica)                                                          │
    │                                                                                                │
    │  sms.outgoing ──→ [router×2] ──→ sms.routed ──→ [sender×4] ──→ sms.sent ──→ [status×2]      │
    │       │                                  │               │                        │             │
    │       └──→ [persist×2]                   │          sms.dlr ──→ [status×2]       │             │
    │            (batch INSERT)                │                                  sms.status          │
    │                                          └──gRPC──→ billing, tarification      │               │
    │                                                                          portal SSE hub         │
    └────────────────────────────────────────────────────────────────────────────────────────────────┘
                 │
                 │  SMPP connections
                 ▼
    ┌──────────────────────────┐      ┌────────────────────────────────┐
    │ smpp-gateway (:2775)     │      │ Legacy: api, smpp-server,      │
    │ (inbound SMPP protocol)  │      │ worker (Kafka consumer+SMPP)   │
    └──────────────────────────┘      └────────────────────────────────┘

    Infrastructure: PostgreSQL 15 (:5432) → PgBouncer (:6432) | Redis 7 (:6379) | Kafka (:9092)
                    Prometheus → Grafana (:3001) | Loki + Promtail | Directus (:8055)
```

**Сервисов:** 15 микросервисов + 4 gateway'а + 4 pipeline-стадии + 3 legacy = **26 процессов**
**gRPC-связей:** ~50+ (portal-gw→16 сервисов, admin-gw→11, client-gw→9, 5 inter-service)
**Kafka topics:** 15 (6 pipeline core + 4 cascade + 3 tarification/billing + 1 audit + 1 HLR)
**Shared DB:** да — единая smpp_db для всех сервисов
**Shared Redis:** да — единый инстанс для всех сервисов

---

## [Boundary Issues]

### 1. **[HIGH]** portal-gateway — God Service
**Где:** `cmd/portal-gateway`, `internal/gateway/portal/`
**Факты:**
- Подключается к **16 backend-сервисам** по gRPC (больше всех в системе)
- Имеет **собственный pgxpool** к PostgreSQL (помимо gRPC-клиентов)
- Запускает **Kafka producer** (audit.events, cascade topics) и **Kafka consumer** (sms.status для SSE)
- Содержит **SSE hub**, **notification scheduler**, **export job runner**, **payment handler**
- Прямые Go-импорты: `services/cascade`, `services/contact`, `services/routing` (минуя gRPC)
**Impact:** Невозможно масштабировать независимо; изменение любого сервиса может требовать пересборки gateway; единая точка отказа для всего портала.
**Рекомендация:** Разделить на: (1) portal-api-gateway (чистый gRPC-прокси), (2) portal-sse-service (Kafka consumer + WebSocket), (3) portal-jobs-service (export, notifications). Убрать прямые Go-импорты внутренних пакетов сервисов — использовать только gRPC.
**Effort:** XL | **Impact:** Высокий — устраняет SPOF, позволяет независимое масштабирование

### 2. **[HIGH]** template-service вмещает 3 домена
**Где:** `cmd/services/template-service`, `internal/services/template/`
**Факты:**
- Служит gRPC-сервером для **3 разных proto-сервисов**: `TemplateService`, `SenderNameService`, `CompanyService`
- Домены SenderName и Company имеют собственные бизнес-правила, не связанные с шаблонами
- Адрес `template-service:9099` используется для подключения к SenderName и Company клиентам
**Impact:** Нарушение Single Responsibility; развёртывание изменений Company/SenderName требует перезапуска Template; путаница в naming.
**Рекомендация:** Вынести SenderName и Company в отдельные сервисы (или объединить в один `registration-service`). Создать отдельные proto-пакеты с собственными адресами.
**Effort:** L | **Impact:** Средний — правильные границы, проще поддержка

### 3. **[MED]** Legacy-сервисы (api, smpp-server, worker) сосуществуют с новой архитектурой
**Где:** `cmd/api/`, `cmd/smpp-server/`, `cmd/worker/`
**Факты:**
- `api` — HTTP+gRPC сервер с прямым доступом к БД, Kafka producer, rate limiter. Дублирует client-gateway + messaging-service.
- `worker` — Kafka consumer с SMPP-пулом. Дублирует pipeline-sender.
- `smpp-server` — отдельный SMPP-сервер, дублирует smpp-gateway.
**Impact:** Непонятно, какой путь данных активен; дублирование кода и поведения; усложнение отладки.
**Рекомендация:** Определить план миграции: отключить legacy-компоненты когда pipeline полностью покрывает их функционал. Добавить feature flags для переключения трафика.
**Effort:** L | **Impact:** Средний — упрощение системы, устранение дублирования

### 4. **[MED]** portal-gateway содержит прямые Go-импорты пакетов других сервисов
**Где:** `internal/gateway/portal/` → импортирует `services/cascade`, `services/contact`, `services/routing`
**Факты:**
- Импортирует domain-типы и infrastructure-пакеты (Kafka producer cascade, postgres repo contact, domain routing)
- Обходит gRPC-контракт, создавая compile-time coupling
**Impact:** Gateway нельзя собрать без полных зависимостей всех импортированных сервисов; изменение внутренней структуры сервиса ломает gateway.
**Рекомендация:** Заменить прямые Go-импорты на gRPC-вызовы к соответствующим сервисам. Для cascade Kafka producer — вынести в cascade-service как gRPC endpoint.
**Effort:** M | **Impact:** Средний — развязывает compile-time зависимости

---

## [Coupling Issues]

### 1. **[HIGH]** Единая разделяемая база данных (smpp_db)
**Связь:** Все 15+ сервисов → PostgreSQL (smpp_db)
**Факты:**
- 93 миграции в одном каталоге `migrations/` для ~50+ таблиц всех доменов
- Таблицы `messages`, `clients`, `accounts`, `client_routes`, `client_providers` записываются несколькими сервисами
- Миграция любого сервиса затрагивает общую схему
**Риск:** Schema-lock при миграции блокирует все сервисы; невозможна независимая эволюция схем; нет изоляции отказов на уровне данных.
**Рекомендация:** Стратегический план (не одномоментно):
1. **Quick win:** Выделить миграции по доменам в подкаталоги (`migrations/billing/`, `migrations/auth/` и т.д.) для clarity.
2. **Medium-term:** Каждый сервис получает Read-Only view или materialized view на чужие таблицы, пишет только в свои.
3. **Long-term:** Database-per-service для billing, auth, campaign/contact (наиболее изолированные домены).
**Effort:** XL | **Impact:** Критический — фундамент масштабируемости и изоляции

### 2. **[HIGH]** Единый Redis для всех доменов
**Связь:** auth (sessions), api (rate limiting), routing (HLR cache, capacity), cascade (reachability), link (URL cache), client (usage), portal (exports) → Redis :6379
**Факты:**
- Изоляция только по prefix-конвенции (`session:`, `rl:`, `hlr:`, `reachability:`, `link:`)
- Нет пароля (default config)
- `link:` ключи без TTL (permanent) — потенциальный memory leak
**Риск:** Exhaustion памяти одним доменом (например, link cache) убивает sessions для всех пользователей; OOM Redis = каскадный отказ.
**Рекомендация:**
1. **Quick win:** Добавить пароль и maxmemory-policy. Установить TTL на все ключи (включая `link:` и `clicked:`).
2. **Medium-term:** Разделить Redis на 2-3 инстанса по критичности: (a) sessions+rate-limiting, (b) caching (HLR, reachability, links), (c) pipeline (limits, counters).
**Effort:** M | **Impact:** Высокий — устраняет общий SPOF для сессий и кеша

### 3. **[MED]** Синхронная gRPC-цепочка в cascade flow
**Связь:** portal-gw → cascade-service → routing-service → provider-service (3 hop)
cascade-service → tarification-service → billing-service (2 hop)
**Факты:**
- cascade-service синхронно вызывает routing (для определения маршрута), tarification (для тарификации), billing (для списания) на каждую доставку
- Timeout 5s на dial + per-call timeouts
**Риск:** Задержка в billing-service (например, deadlock на accounts) каскадно задерживает cascade → portal → пользователь видит timeout.
**Рекомендация:** Добавить circuit breaker на cascade→billing и cascade→tarification gRPC-вызовы. Рассмотреть async тарификацию через Kafka (уже есть `tarification.results` topic — использовать его).
**Effort:** M | **Impact:** Высокий — предотвращает каскадные отказы

### 4. **[MED]** messaging-service использует порт 9092 (конфликт с Kafka)
**Связь:** Все gRPC-клиенты → messaging-service:9092; Kafka broker → kafka:9092
**Факты:**
- Docker-compose разводит по hostname (kafka:9092 vs messaging-service:9092), но порт одинаковый
- Путаница при чтении конфигурации и отладке
**Риск:** Ошибочное подключение к Kafka вместо messaging-service при неправильной конфигурации.
**Рекомендация:** Переназначить gRPC-порт messaging-service на уникальный (например, :9105).
**Effort:** S | **Impact:** Низкий — устранение путаницы

### 5. **[LOW]** internal/storage — shared data layer
**Связь:** api, gateway, pipeline, router, services/messaging, services/provider, services/routing → `internal/storage`
**Факты:**
- 15+ repository-файлов для разных доменов (messages, clients, providers, routes, etc.)
- `storage` импортирует `services/company/domain` — обратная зависимость
**Риск:** Изменение общего repository ломает несколько сервисов; невозможна изоляция.
**Рекомендация:** Постепенно мигрировать каждый сервис на собственные repository (DDD-стиль из `services/*/infrastructure/repository`). Многие сервисы уже это сделали — завершить для оставшихся.
**Effort:** L | **Impact:** Средний — чистая декомпозиция

---

## [Data Flow Issues]

### 1. **[MED]** SMS отправка: 8+ hop'ов в pipeline
**Поток:** Client → client-gateway → messaging-service (gRPC) → Kafka `sms.outgoing` → pipeline-router → Kafka `sms.routed` → pipeline-sender (+ gRPC to billing, tarification) → Kafka `sms.sent`/`sms.dlr` → pipeline-status → Kafka `sms.status`
**Проблема:** 8+ промежуточных шагов, 5 Kafka topics. Каждый hop добавляет latency. Однако каждый stage обоснован (routing, sending, status tracking, persistence).
**Оценка:** Количество hop'ов **обосновано** для high-throughput pipeline с batch-обработкой. Это **не проблема**, а сознательный архитектурный выбор.
**Рекомендация:** Не сокращать pipeline. Но: (1) убедиться, что pipeline-sender не блокируется на синхронных gRPC-вызовах к billing/tarification — рассмотреть pre-tarification в router stage. (2) Мониторить end-to-end latency через существующий trace_id.
**Effort:** M | **Impact:** Средний — снижение latency на hot path

### 2. **[MED]** Двойная запись в таблицу messages
**Поток:** pipeline-persist (INSERT from sms.outgoing) и pipeline-status (UPDATE from sms.sent/sms.dlr) пишут в одну таблицу
**Проблема:** Potential race condition — status update может прийти раньше, чем persist завершит INSERT. Защита: `ON CONFLICT DO NOTHING` для persist, `WHERE updated_at < new_updated_at` для status.
**Текущая защита:** Достаточная — idempotent writes с temporal ordering.
**Рекомендация:** Добавить метрику для "status update arrived before persist" событий для мониторинга race condition frequency.
**Effort:** S | **Impact:** Низкий — observability improvement

### 3. **[MED]** Audit flow разделён между shared и audit-service
**Поток:** portal-gateway → Kafka `audit.events` → `shared/audit/consumer.go` (INSERT) → PostgreSQL ← audit-service (SELECT/query)
**Проблема:** Audit consumer (`shared/audit`) — это отдельный пакет от `services/audit`. Consumer пишет в audit_log, а audit-service только читает. Нет единого владельца домена.
**Рекомендация:** Переместить Kafka consumer из `shared/audit` в `services/audit` (audit-service). Audit-service должен быть единственным писателем в audit_log.
**Effort:** S | **Impact:** Низкий — правильные границы владения

### 4. **[LOW]** DLR path дублируется
**Поток 1:** Provider DLR → SMPP → Kafka `sms.dlr` → pipeline-status → Kafka `sms.status` → portal SSE
**Поток 2:** Kafka `sms.dlr` → analytics-service (статистика)
**Поток 3:** Kafka `sms.dlr` → billing-service (refund failed)
**Поток 4:** Kafka `sms.dlr` → webhook-service (DLR to client)
**Проблема:** 4 разных consumer'а на одном topic — это нормальный fan-out pattern. Не проблема.

---

## [Resilience Gaps]

### 1. **[HIGH]** Полное отсутствие circuit breaker
**SPOF:** Любой медленный gRPC-сервис (billing, tarification, routing)
**Сценарий отказа:** billing-service входит в deadlock → tarification-service ждёт 5s timeout → pipeline-sender блокирован → Kafka consumer lag растёт → backlog на всех стадиях → платформа перестаёт отправлять SMS
**Факты:** Документация упоминает circuit breaker как желаемый паттерн, но реализации нет нигде.
**Рекомендация:** Внедрить `sony/gobreaker` на критичных путях:
- pipeline-sender → billing (ChargeMessage)
- pipeline-sender → tarification (TarifyMessage)
- cascade-service → routing, tarification, billing
- portal-gateway → все backend-сервисы
Настройки: порог 5 ошибок за 60s, half-open после 30s.
**Effort:** M | **Impact:** Критический — предотвращает каскадные отказы

### 2. **[HIGH]** portal-gateway — единая точка отказа для портала
**SPOF:** portal-gateway (1 инстанс в docker-compose)
**Сценарий отказа:** OOM / panic в portal-gateway → портал полностью недоступен (frontend получает 502 от HAProxy).
**Рекомендация:** (1) Увеличить до 2+ реплик portal-gateway за HAProxy (уже настроено для client-gw и admin-gw). (2) Реализовать graceful degradation — если billing-service недоступен, показывать cached balance вместо ошибки.
**Effort:** S | **Impact:** Высокий — устранение SPOF

### 3. **[MED]** Нет retry на gRPC-вызовах из gateway
**Сценарий отказа:** Transient network error на portal-gw → billing-service gRPC → immediate error → пользователь видит ошибку
**Факты:** Все gateways имеют dial timeout 5s и per-call timeouts, но zero retry logic.
**Рекомендация:** Добавить gRPC client interceptor с retry для idempotent операций (GET/List/Query). Использовать `grpc-middleware/retry` с max 2 attempts, 100ms backoff. НЕ ретраить мутации (Charge, Deduct).
**Effort:** S | **Impact:** Средний — улучшение user experience

### 4. **[MED]** Webhook retries в памяти — потеря при рестарте
**Сценарий отказа:** webhook-service рестартует → все pending retries (до 5 на каждый webhook, schedule 15s-15m) потеряны.
**Факты:** Документировано как принятый trade-off. Schedule: `[15s, 30s, 1m, 5m, 15m]` через `time.AfterFunc`.
**Рекомендация:** Персистить retry state в Redis (HSet `webhook:retry:<delivery_id>` с next_attempt timestamp). При старте — восстановить pending retries.
**Effort:** M | **Impact:** Средний — надёжная доставка webhook'ов

### 5. **[MED]** SMPP dial timeout 30 секунд
**Сценарий отказа:** Провайдер SMPP недоступен → goroutine блокирована на 30s → при масштабировании множество blocked goroutines → memory pressure.
**Где:** `internal/smsc/pool_connections.go:78`, `internal/smsc/pool_async.go:447`
**Рекомендация:** Снизить до 5-10s. 30s — неприемлемо для connection timeout. Добавить connection pool с health-check goroutine.
**Effort:** S | **Impact:** Средний — предотвращает goroutine leak

### 6. **[LOW]** Kafka producer: линейный backoff вместо экспоненциального
**Где:** `internal/queue/producer.go:216-249`
**Факты:** `attempt * RetryBackoff` (linear: 1s, 2s, 3s) вместо `2^attempt * base` (exponential: 1s, 2s, 4s, 8s).
**Рекомендация:** Заменить на exponential backoff с jitter для предотвращения thundering herd.
**Effort:** S | **Impact:** Низкий — улучшение retry behavior

---

## [Scalability Bottlenecks]

### 1. **[HIGH]** Единая PostgreSQL — потолок throughput
**Компонент:** PostgreSQL 15 (single instance), PgBouncer (connection pooling)
**При нагрузке:** При >10K msg/sec таблица `messages` (даже с monthly partitioning) становится bottleneck на INSERT/UPDATE. Все сервисы конкурируют за connection pool.
**Факты:**
- PgBouncer используется только 4 high-throughput сервисами (pipeline, messaging, routing); остальные 12+ подключаются напрямую
- `SMPP_DATABASE_MAX_OPEN_CONNS=100` на gateway
- pipeline-persist использует `COPY` для batch INSERT (хорошо), но 2 реплики с `BATCH_SIZE=2000`
**Рекомендация:**
1. **Quick win:** Подключить ВСЕ сервисы через PgBouncer (не только pipeline).
2. **Medium-term:** Read replicas для analytics-service, audit-service, portal read queries.
3. **Long-term:** Отдельные БД для billing (critical writes), campaigns/contacts (bulk operations).
**Effort:** S→XL (поэтапно) | **Impact:** Критический

### 2. **[HIGH]** Единый Redis без clustering
**Компонент:** Redis 7 (single instance, appendonly)
**При нагрузке:** При >50K concurrent sessions + high rate limiting + active HLR cache — memory pressure. Нет failover.
**Рекомендация:**
1. **Quick win:** Установить `maxmemory` и `maxmemory-policy allkeys-lru`. Добавить TTL на `link:` и `clicked:` ключи.
2. **Medium-term:** Redis Sentinel для HA. Разделение на 2+ инстанса (sessions vs cache).
**Effort:** S→M | **Impact:** Высокий

### 3. **[MED]** Kafka: 1 broker, replication factor 1
**Компонент:** Kafka (single broker confluent 7.5)
**При нагрузке:** Single broker = SPOF для всего pipeline. Потеря Kafka = полная остановка отправки SMS.
**Рекомендация:** Минимум 3 брокера с replication factor 2 для production. Topics с 32 partitions уже готовы к распределению.
**Effort:** M | **Impact:** Критический — устранение SPOF

### 4. **[MED]** pipeline-sender делает синхронные gRPC-вызовы на hot path
**Компонент:** pipeline-sender (4 реплики) → billing + tarification gRPC
**При нагрузке:** Каждое SMS требует 2 gRPC-вызова (tarify + charge) с timeout 5s. При 10K msg/sec = 20K gRPC calls/sec. Billing-service становится bottleneck.
**Рекомендация:** Pre-tarify в router stage (batch): тарифицировать пакет сообщений одним вызовом перед отправкой в sender. Sender только списывает (или списание тоже async через Kafka).
**Effort:** L | **Impact:** Высокий — убирает sync bottleneck с hot path

### 5. **[LOW]** Route cache — in-memory с периодическим refresh
**Компонент:** `internal/pipeline/cache/route_cache.go` — sync.RWMutex cache
**При нагрузке:** Refresh из PostgreSQL может быть тяжёлым при >10K routes. Но между refresh'ами — zero DB load (хорошо).
**Рекомендация:** Мониторить duration refresh через метрику `route_cache_refresh_duration_seconds` (уже существует). Если >1s — рассмотреть incremental refresh.
**Effort:** S | **Impact:** Низкий

---

## [Observability Gaps]

### 1. **[HIGH]** Нет alerting-инфраструктуры
**Факты:**
- Prometheus: 70+ метрик, scrape 17 targets каждые 15s — но **нет rule_files**, **нет recording/alerting rules**
- Нет Alertmanager, нет PagerDuty, нет Slack-интеграции для алертов
- Единственный comment: `// Use this in Alertmanager rules: sms_gateway_hlr_all_providers_down == 1` (TODO)
**Рекомендация:**
1. Развернуть Alertmanager с Slack/Telegram-нотификацией
2. Создать critical rules: `pipeline_queue_depth > 10000`, `billing error rate > 5%`, `smpp_connections_active == 0`, `up{job=~".*-service"} == 0`
3. Warning rules: `http_request_duration_seconds p99 > 2s`, `database_connections_active > 80% pool`, `redis memory > 80%`
**Effort:** M | **Impact:** Критический — без алертов проблемы обнаруживаются только вручную

### 2. **[MED]** Нет distributed tracing (OpenTelemetry)
**Факты:**
- Кастомный `trace_id` propagation через zerolog + Kafka message fields — работает для pipeline
- Loki derivedFields делают trace_id кликабельным в Grafana — хорошее решение
- НО: нет span trees, нет flame graphs, нет automatic propagation через gRPC interceptors
- gRPC `request_id` и pipeline `trace_id` — это **разные ID**, не связанные между собой
**Рекомендация:**
1. **Quick win:** Объединить request_id и trace_id — при входе в messaging-service, trace_id = request_id из gRPC metadata.
2. **Medium-term:** Добавить OpenTelemetry SDK: `go.opentelemetry.io/otel` + Jaeger exporter. Начать с pipeline stages (инструментация уже есть через trace.Log).
**Effort:** M→L | **Impact:** Средний — ускоряет debugging

### 3. **[MED]** request_id и trace_id не связаны
**Факты:**
- HTTP middleware генерирует `request_id` (UUID) и пропагирует через gRPC metadata
- Pipeline stage получает `TraceID` из Kafka message — это **другой** ID
- Невозможно проследить путь: HTTP request → gRPC call → Kafka message → pipeline → status
**Рекомендация:** При публикации в Kafka sms.outgoing, устанавливать `TraceID = request_id` из gRPC context. Проверить: `internal/api/grpc/server.go:110` — если уже делает это, задокументировать. Если нет — добавить.
**Effort:** S | **Impact:** Средний — end-to-end traceability

### 4. **[LOW]** Health checks не единообразны
**Факты:**
- provider-service, analytics-service, contact-service: `/health` + `/health/live` + `/health/ready`
- api-gateway: только `/health/live` + `/health/ready` (нет `/health`)
- Многие cmd/services/* не имеют явно зарегистрированных health endpoints в коде (хотя docker healthcheck их предполагает)
**Рекомендация:** Стандартизировать: все сервисы регистрируют `/health`, `/health/live`, `/health/ready` через `monitoring.HealthChecker`. Добавить проверку зависимостей (DB, Redis, gRPC) в readiness check.
**Effort:** S | **Impact:** Низкий — operational consistency

---

## [Recommendations Summary]

| # | Рекомендация | Severity | Effort | Impact | Категория |
|---|-------------|----------|--------|--------|-----------|
| 1 | Внедрить circuit breaker (sony/gobreaker) на pipeline-sender→billing/tarification и cascade→backend | HIGH | M | Предотвращает каскадные отказы — критично | Resilience |
| 2 | Развернуть Alertmanager + critical/warning rules для 70+ существующих метрик | HIGH | M | Обнаружение проблем до impact на пользователей | Observability |
| 3 | Добавить 2+ реплики portal-gateway за HAProxy (аналогично client-gw) | HIGH | S | Устранение SPOF портала | Resilience |
| 4 | Подключить все сервисы через PgBouncer (сейчас только 4 из 16+) | HIGH | S | Контроль connections, подготовка к read replicas | Scalability |
| 5 | Redis: установить maxmemory + TTL на все ключи + пароль | HIGH | S | Предотвращение OOM и несанкционированного доступа | Scalability |
| 6 | Kafka: минимум 3 брокера, replication factor 2 | HIGH | M | Устранение SPOF message bus | Scalability |
| 7 | Разделить Redis на 2+ инстанса (sessions vs cache) | HIGH | M | Изоляция отказов, масштабирование | Scalability |
| 8 | Снизить SMPP dial timeout с 30s до 5-10s | MED | S | Предотвращение goroutine leak | Resilience |
| 9 | Добавить gRPC retry interceptor для idempotent operations (GET/List) | MED | S | Tolerance к transient failures | Resilience |
| 10 | Объединить request_id и trace_id в pipeline | MED | S | End-to-end traceability | Observability |
| 11 | Вынести SenderName и Company из template-service | MED | L | Правильные bounded contexts | Boundaries |
| 12 | Переместить audit Kafka consumer из shared/ в audit-service | MED | S | Единый owner домена | Boundaries |
| 13 | Убрать прямые Go-импорты сервисов из portal-gateway | MED | M | Развязка compile-time зависимостей | Coupling |
| 14 | Pre-tarification в router stage (batch) вместо sync вызовов в sender | MED | L | Убирает sync bottleneck на hot path | Scalability |
| 15 | Переназначить порт messaging-service с 9092 на уникальный | MED | S | Устранение путаницы с Kafka | Coupling |
| 16 | Webhook retries: персистить в Redis вместо in-memory | MED | M | Надёжная доставка webhook'ов | Resilience |
| 17 | Plan миграции legacy (api, smpp-server, worker) → deprecation | MED | L | Устранение дублирования | Boundaries |
| 18 | Стандартизировать health checks во всех сервисах | LOW | S | Operational consistency | Observability |
| 19 | Kafka producer: exponential backoff с jitter | LOW | S | Улучшение retry behavior | Resilience |
| 20 | OpenTelemetry SDK + Jaeger для distributed tracing | MED | L | Ускорение debugging, flame graphs | Observability |

### Quick wins (Effort S, можно сделать за 1-2 дня):
- #3 (portal-gw replicas), #4 (PgBouncer), #5 (Redis config), #8 (SMPP timeout), #9 (gRPC retry), #10 (trace_id), #12 (audit consumer), #15 (port rename), #18 (health checks), #19 (exponential backoff)

### Strategic (Effort L-XL, требуют планирования):
- #1→#6→#7 (resilience + infrastructure), #11→#13→#14 (boundaries + performance), #17 (legacy deprecation), #20 (tracing)

---

## Приложение A: Полная карта Kafka topics

| Topic | Partitions | Producers | Consumers |
|-------|-----------|-----------|-----------|
| `sms.outgoing` | 32 | messaging-service | pipeline-router, pipeline-persist |
| `sms.routed` | 32 | pipeline-router | pipeline-sender |
| `sms.sent` | 16 | pipeline-sender | pipeline-status |
| `sms.dlr` | 16 | pipeline-sender, stub | pipeline-status, analytics, billing, webhook |
| `sms.status` | 16 | pipeline-status | portal SSE hub |
| `sms.failed` | 8 | queue.PublishFailed | pipeline-router, analytics, billing, webhook |
| `audit.events` | — | portal-gateway | shared/audit consumer |
| `cascade.start` | — | cascade producer | cascade orchestrator |
| `cascade.attempt.send` | — | cascade producer | cascade channel senders |
| `cascade.attempt.result` | — | cascade producer | cascade orchestrator |
| `cascade.billing` | — | cascade producer | cascade billing consumer |
| `billing.balance.changed` | — | billing-service | client-service (cache invalidation) |
| `billing.transaction.created` | — | billing-service | analytics-service |
| `tarification.results` | — | tarification-service | billing correlation |
| `lookup.completed` | — | routing HLR publisher | analytics/monitoring |

## Приложение B: Граф gRPC зависимостей

```
portal-gateway (16 connections):
  → auth, client, billing, messaging, analytics, webhook, routing, audit,
    provider, contact, campaign, template, tarification, link, cascade, company

admin-gateway (11):
  → auth, client, provider, routing, analytics, billing, webhook, template,
    tarification, sender-name, audit

client-gateway (9):
  → auth, messaging, analytics, billing, webhook, template, routing, client, cascade

smpp-gateway (1):
  → auth

Inter-service gRPC:
  auth-service → client-service
  tarification-service → billing-service, routing-service
  cascade-service → routing-service, tarification-service, billing-service
  campaign-service → contact-service, template-service
  routing-service → provider-service
  pipeline-sender → billing-service, tarification-service
```

## Приложение C: Redis ключи по доменам

| Домен | Prefix | TTL | Сервис |
|-------|--------|-----|--------|
| Auth/Sessions | `session:*` | 24h | auth-service |
| Auth/2FA | `login_ticket:*` | 5m | auth-service |
| Rate Limiting | `rl:*` | 1s/1m/1h | api middleware |
| HLR Cache | `hlr:*` | 24h | routing-service |
| Reachability | `reachability:*` | 1h | cascade-service |
| Link Cache | `link:*` | **no TTL** | link-service |
| Click Tracking | `clicked:*` | **no TTL** | link-service |
| Pipeline Limits | `limits:*` | configurable | pipeline |
| Provider Counters | `provider:*:tps_current` | 1s | routing |
| Provider Counters | `provider:*:daily_count` | midnight | routing |
| Provider Counters | `provider:*:monthly_count` | end of month | routing |
| Usage | `usage:*` | end of month +24h | client-service |
| Export Jobs | `export:job:*` | 1h | portal-gateway |

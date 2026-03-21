# Research: Unit & Functional Tests

**Feature**: 005-unit-functional-tests
**Date**: 2026-03-21

## Decision 1: Подход к мокированию

**Decision**: testify/mock для всех новых моков. Существующие function-field моки в `internal/testutil/mocks.go` остаются.

**Rationale**: testify/mock даёт AssertExpectations (проверка что все ожидаемые вызовы произошли), гибкие матчеры (mock.Anything, mock.MatchedBy), автоматическую генерацию через mockery. Существующие 5 function-field моков работают и используются в 28 тест-файлах — миграция создаёт ненужный риск.

**Alternatives considered**:
- Полная миграция на testify/mock — слишком большой скоуп, рефакторинг 28 файлов
- gomock/mockgen — менее идиоматичен для testify-based проекта
- Только function-field — нет AssertExpectations, сложнее проверять порядок вызовов

## Decision 2: Организация моков

**Decision**: Моки размещаются рядом с интерфейсами в файлах `mock_*.go` внутри пакета `mocks/` каждого сервиса: `internal/services/{service}/mocks/`.

**Rationale**: Следует принципу locality — моки рядом с интерфейсами, которые они реализуют. Каждый сервис самодостаточен. Избегаем god-object в testutil.

**Alternatives considered**:
- Всё в `internal/testutil/mocks.go` — уже перегружен, нарушает DDD bounded contexts
- В тех же пакетах что и интерфейсы — загрязняет production код

## Decision 3: Структура тестов (Describe/Context/It)

**Decision**: Использовать вложенные `t.Run()` с конвенцией именования:
```go
func TestAuthService(t *testing.T) {                    // Describe
    t.Run("AuthenticateByCredentials", func(t *testing.T) { // Method
        t.Run("with valid credentials", func(t *testing.T) { // Context
            // It: returns user and tokens
        })
        t.Run("with invalid password", func(t *testing.T) { // Context
            // It: returns ErrInvalidCredentials
        })
    })
}
```

**Rationale**: Нативный Go без доп. зависимостей (ginkgo/goconvey). Читаемый вывод `go test -v`. Совместимо с testify/assert.

**Alternatives considered**:
- ginkgo/gomega — тяжёлая зависимость, другая парадигма
- goconvey — устаревший, меньше поддержки
- Плоские Test* функции — нет группировки, сложно читать

## Decision 4: Kafka в функциональных тестах

**Decision**: Kafka мокируется через domain интерфейсы (EventPublisher, EventConsumer). Функциональные тесты не поднимают реальный Kafka.

**Rationale**: EventPublisher/EventConsumer — чистые интерфейсы в domain-слое. Мок проверяет что сервис вызвал PublishMessageCreated с правильными аргументами. Сериализация Kafka-сообщений тестируется отдельно в existing `internal/queue/message_test.go`.

**Alternatives considered**:
- testcontainers с реальным Kafka — медленно (10+ секунд на старт), избыточно для функциональных тестов
- Embedded Kafka (franz-go) — сложная настройка, Go не имеет встроенного embedded broker

## Decision 5: Изоляция функциональных тестов (БД)

**Decision**: Каждый функциональный тест оборачивается в транзакцию с rollback.

**Rationale**: Быстрее TRUNCATE (нет DDL операций), полная изоляция между тестами, данные гарантированно не утекают. Существующий `testutil.SetupTestDB` возвращает cleanup func — расширим его для поддержки транзакционной изоляции.

**Alternatives considered**:
- TRUNCATE в teardown — медленнее, проблемы с FK constraints
- Отдельная БД на тест — избыточный overhead, сложность миграций

## Decision 6: Количество интерфейсов для мокирования

**Decision**: 43 интерфейса в domain-слоях 7 сервисов + gateway gRPC клиенты.

**Breakdown**:
| Сервис | Интерфейсы | Ключевые |
|--------|-----------|---------|
| Auth | 2 | PasswordHasher, APIKeyGenerator |
| Messaging | 4 | MessageRepository, DLRRepository, EventPublisher, EventSubscriber |
| Routing | 12 | RouteRepository, HLRCache, HLRProviderAdapter, AdapterFactory, ProviderSelector и др. |
| Billing | 5 | AccountRepository, TransactionRepository, TransferRepository, PricingRuleRepository, EventPublisher |
| Tarification | 9 | TariffPlanRepository, TariffPeriodRepository, UsageCounterRepository, BillingStrategy и др. |
| Analytics | 3 | MetricRepository, ReportRepository, EventConsumer |
| Client | 2 | ClientRepository, ConfigRepository (infrastructure, но мокируются) |
| Provider | 1 | ProviderRepository |
| Webhook | 1 | CacheInvalidator |
| Gateway | 4+ | gRPC generated clients (MessagingServiceClient, BillingServiceClient и т.д.) |

## Decision 7: Рефакторинг существующих тестов

**Decision**: Существующие 28 тест-файлов рефакторятся для единообразной структуры Describe/Context/It через t.Run. Логика тестов не меняется, только организация.

**Scope**:
- `internal/router/router_test.go` (7 тестов) → группировка по методам
- `internal/api/http/handlers_test.go` → группировка по эндпоинтам
- `internal/api/middleware/*_test.go` (5 файлов) → Context по сценариям
- `internal/shared/errors_test.go` (4 теста) → Describe/Context
- `internal/smpp/protocol/*_test.go` (3 файла) → группировка по PDU типам
- `test/integration/*_test.go` (5 файлов) → единая структура
- `test/load/*_test.go` (2 файла) → единая структура

**Alternatives considered**:
- Не трогать существующие — нарушает FR-015/FR-016, неединообразный стиль
- Полностью переписать — избыточный риск, тесты работают

# Data Model: Unit & Functional Tests

**Feature**: 005-unit-functional-tests
**Date**: 2026-03-21

## Test Infrastructure Entities

### Mock (testify/mock)

Тестовый двойник domain-интерфейса. Создаётся через `mock.Mock` embedding.

**Attributes**:
- `mock.Mock` — embedded struct для recording/assertion
- Методы интерфейса — каждый вызывает `m.Called(args...)` и возвращает результат

**Relationships**: Один Mock реализует один domain-интерфейс. Mock инжектируется в application-сервис вместо реальной реализации.

**Validation**: `mock.AssertExpectations(t)` — проверяет что все `.On()` expectations были вызваны.

### TestFixture

Фабрика тестовых данных для создания валидных domain-сущностей.

**Attributes**:
- Builder-методы с предсказуемыми значениями по умолчанию
- Overridable через functional options или explicit setters

**Relationships**: Используется в setup-фазе тестов для создания входных данных.

**Existing fixtures** (расширяются, не заменяются):
- `NewTestMessage()` → shared.Message
- `NewTestClient()` → shared.Client
- `NewTestProvider()` → shared.Provider
- `NewTestRoute()` → shared.Route
- `NewTestDLRReceipt()` → shared.DLRReceipt

**New fixtures needed**:
- `NewTestUser()` → auth/domain.User
- `NewTestAPIKey()` → auth/domain.APIKey
- `NewTestAccount()` → billing/domain.Account
- `NewTestTransaction()` → billing/domain.Transaction
- `NewTestTariffPlan()` → tarification/domain.TariffPlan
- `NewTestTariffPeriod()` → tarification/domain.TariffPeriod
- `NewTestTariffTier()` → tarification/domain.TariffTier
- `NewTestLookupResult()` → routing/domain.LookupResult
- `NewTestHLRProvider()` → routing/domain.HLRProvider
- `NewTestOperator()` → routing/domain.Operator
- `NewTestSmartRouteWeight()` → routing/domain.SmartRouteWeight
- `NewTestMetric()` → analytics/domain.Metric
- `NewTestSenderRegistration()` → tarification/domain.SenderRegistration
- `NewTestUsageCounter()` → tarification/domain.UsageCounter

### TestDB (транзакционная изоляция)

Обёртка для тестовой PostgreSQL с автоматическим rollback.

**Attributes**:
- `db` — pgx connection pool
- `tx` — активная транзакция для текущего теста
- `cleanup` — функция отката

**Lifecycle**:
1. `SetupTestDB(t)` → подключение к тестовой БД
2. `BeginTestTx(t, db)` → начало транзакции, возврат tx-wrapped db
3. Тест выполняется внутри транзакции
4. `t.Cleanup()` → автоматический ROLLBACK

**State transitions**: Init → Connected → InTransaction → RolledBack

## Mock Organization

```text
internal/services/
├── auth/
│   └── mocks/
│       ├── mock_password_hasher.go
│       └── mock_api_key_generator.go
├── messaging/
│   └── mocks/
│       ├── mock_message_repository.go
│       ├── mock_dlr_repository.go
│       └── mock_event_publisher.go
├── routing/
│   └── mocks/
│       ├── mock_route_repository.go
│       ├── mock_hlr_cache.go
│       ├── mock_hlr_provider_adapter.go
│       ├── mock_hlr_provider_repository.go
│       ├── mock_lookup_log_repository.go
│       ├── mock_adapter_factory.go
│       ├── mock_provider_repository.go
│       ├── mock_operator_repository.go
│       ├── mock_operator_prefix_repository.go
│       ├── mock_country_repository.go
│       ├── mock_smart_route_weight_repository.go
│       ├── mock_event_publisher.go
│       └── mock_provider_selector.go
├── billing/
│   └── mocks/
│       ├── mock_account_repository.go
│       ├── mock_transaction_repository.go
│       ├── mock_transfer_repository.go
│       ├── mock_pricing_rule_repository.go
│       └── mock_event_publisher.go
├── tarification/
│   └── mocks/
│       ├── mock_tariff_plan_repository.go
│       ├── mock_tariff_period_repository.go
│       ├── mock_tariff_tier_repository.go
│       ├── mock_usage_counter_repository.go
│       ├── mock_sender_registration_repository.go
│       ├── mock_tarification_log_repository.go
│       ├── mock_prepaid_fee_repository.go
│       ├── mock_pricing_period_repository.go
│       ├── mock_event_publisher.go
│       └── mock_billing_strategy.go
├── analytics/
│   └── mocks/
│       ├── mock_metric_repository.go
│       ├── mock_report_repository.go
│       └── mock_event_consumer.go
├── provider/
│   └── mocks/
│       └── mock_provider_repository.go
├── client/
│   └── mocks/
│       ├── mock_client_repository.go
│       └── mock_config_repository.go
└── webhook/
    └── mocks/
        └── mock_cache_invalidator.go
```

## Interface-to-Mock Mapping (43 interfaces)

| Service | Interface | Mock File | Methods |
|---------|-----------|-----------|---------|
| auth | PasswordHasher | mock_password_hasher.go | HashPassword, CheckPassword |
| auth | APIKeyGenerator | mock_api_key_generator.go | GenerateAPIKey, GetKeyPrefix |
| messaging | MessageRepository | mock_message_repository.go | Create, GetByID, GetByMessageID, Update, UpdateStatus, GetByClientID, GetPendingForRetry, GetScheduledReady, GetStuckPending, CancelByIDAndStatus, GetSentExpired, BulkUpdateStatusToExpired, GetByExternalID, GetBySMPPMessageID |
| messaging | DLRRepository | mock_dlr_repository.go | Create, GetByMessageID, GetBySMPPMessageID |
| messaging | EventPublisher | mock_event_publisher.go | PublishMessageCreated, PublishMessageQueued, PublishMessageStatusChanged |
| messaging | EventSubscriber | mock_event_subscriber.go | SubscribeToDLRReceived, SubscribeToMessageStatusChanged |
| routing | RouteRepository | mock_route_repository.go | Create, GetByID, Update, Delete, List, GetActiveByDestination |
| routing | ProviderRepository | mock_provider_repository.go | GetByID, GetAllActive, GetHealth |
| routing | HLRCache | mock_hlr_cache.go | Get, Set, Delete |
| routing | HLRProviderRepository | mock_hlr_provider_repository.go | Create, Update, Delete, GetByID, ListActive, GetByPriority |
| routing | HLRProviderAdapter | mock_hlr_provider_adapter.go | Lookup, Ping, Name |
| routing | AdapterFactory | mock_adapter_factory.go | CreateAdapter |
| routing | LookupLogRepository | mock_lookup_log_repository.go | Insert, ListByClient, CountByClient |
| routing | OperatorRepository | mock_operator_repository.go | Create, GetByID, GetByCode, Update, List |
| routing | OperatorPrefixRepository | mock_operator_prefix_repository.go | Create, Delete, ListByOperatorID, FindByNumber |
| routing | CountryRepository | mock_country_repository.go | Create, GetByID, GetByISOCode, Update, List |
| routing | SmartRouteWeightRepository | mock_smart_route_weight_repository.go | Upsert, GetByOperatorAndCountry, List, Delete |
| routing | ProviderSelector | mock_provider_selector.go | SelectProvider |
| routing | EventPublisher | mock_event_publisher.go | PublishMessageRouted |
| billing | AccountRepository | mock_account_repository.go | Create, GetByClientID, Update, UpdateBalance |
| billing | TransactionRepository | mock_transaction_repository.go | Create, GetByID, GetByClientID, GetByClientIDAndType, GetByClientIDAndPeriod, GetByMessageID |
| billing | TransferRepository | mock_transfer_repository.go | Create, GetByID |
| billing | PricingRuleRepository | mock_pricing_rule_repository.go | Create, GetByID, GetByClientID, GetGlobalRules, GetMatchingRule, Update, Delete |
| billing | EventPublisher | mock_event_publisher.go | PublishBalanceChanged, PublishTransactionCompleted |
| tarification | TariffPlanRepository | mock_tariff_plan_repository.go | Create, GetByID, GetActiveByOperatorAndCategory, Update, List |
| tarification | TariffPeriodRepository | mock_tariff_period_repository.go | Create, GetByID, GetActiveByPlanID, ListByPlanID, HasActivePeriod |
| tarification | TariffTierRepository | mock_tariff_tier_repository.go | Create, GetByID, Update, ListByPeriodID |
| tarification | UsageCounterRepository | mock_usage_counter_repository.go | GetOrCreate, IncrementAndGet, GetByClient |
| tarification | SenderRegistrationRepository | mock_sender_registration_repository.go | Create, GetByID, GetByClientAndOperator, GetActiveByClientOperatorName, Update, List |
| tarification | TarificationLogRepository | mock_tarification_log_repository.go | Create, GetByIdempotencyKey, GetByMessageID |
| tarification | PrepaidFeeRepository | mock_prepaid_fee_repository.go | Create, GetByID, GetByPeriodID, GetUncharged, MarkCharged |
| tarification | PricingPeriodRepository | mock_pricing_period_repository.go | Create, GetByID, GetActiveByTariffPeriodID, ListByTariffPeriodID |
| tarification | EventPublisher | mock_event_publisher.go | PublishTarificationResult, PublishRecalcEvent, PublishPrepaidEvent, Close |
| tarification | BillingStrategy | mock_billing_strategy.go | Calculate |
| analytics | MetricRepository | mock_metric_repository.go | Create, CreateAggregated, GetAggregated, GetStatistics, GetProviderPerformance, UpdateAggregated |
| analytics | ReportRepository | mock_report_repository.go | Create, GetByID, List |
| analytics | EventConsumer | mock_event_consumer.go | ConsumeMessageCreated, ConsumeMessageSent, ConsumeMessageDelivered, ConsumeMessageFailed |
| provider | ProviderRepository | mock_provider_repository.go | Create, Update, GetByID, GetByName, List, Delete |
| client | ClientRepository | mock_client_repository.go | (infrastructure, mock by interface) |
| client | ConfigRepository | mock_config_repository.go | (infrastructure, mock by interface) |
| webhook | CacheInvalidator | mock_cache_invalidator.go | InvalidateCache |

# Contract: TarificationService gRPC API

**Proto file**: `api/proto/tarification/tarification.proto`
**Package**: `tarification`
**Go package**: `api/proto/tarificationv1`

## Service Definition

### TarificationService

#### Core Operations

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| TarifyMessage | TarifyMessageRequest | TarifyMessageResponse | Тарифицировать сообщение при отправке |

#### Sender Registration Management

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| CreateSenderRegistration | CreateSenderRegistrationRequest | SenderRegistration | Зарегистрировать имя отправителя |
| ListSenderRegistrations | ListSenderRegistrationsRequest | ListSenderRegistrationsResponse | Список регистраций (с фильтрами) |
| UpdateSenderRegistration | UpdateSenderRegistrationRequest | SenderRegistration | Обновить статус/тип регистрации |

#### Tariff Plan Management

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| CreateTariffPlan | CreateTariffPlanRequest | TariffPlan | Создать тарифный план |
| GetTariffPlan | GetTariffPlanRequest | TariffPlan | Получить план по ID |
| ListTariffPlans | ListTariffPlansRequest | ListTariffPlansResponse | Список планов (фильтр по operator_id) |
| UpdateTariffPlan | UpdateTariffPlanRequest | TariffPlan | Обновить план (активация/деактивация) |

#### Period & Tier Management

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| CreateTariffPeriod | CreateTariffPeriodRequest | TariffPeriod | Создать тарифный период |
| CreateTariffTier | CreateTariffTierRequest | TariffTier | Создать порог внутри периода |
| UpdateTariffTier | UpdateTariffTierRequest | TariffTier | Обновить порог (только неактивный период) |
| CreatePricingPeriod | CreatePricingPeriodRequest | PricingPeriod | Создать ценовой период |

#### Prepaid & Usage

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| CreatePrepaidFee | CreatePrepaidFeeRequest | PrepaidFee | Задать предоплату для периода |
| GetUsageCounter | GetUsageCounterRequest | UsageCounter | Получить счётчик использования |
| ListUsageCounters | ListUsageCountersRequest | ListUsageCountersResponse | Список счётчиков (фильтр по client_id) |

## Key Messages

### TarifyMessageRequest

```
client_id:       string (UUID)
message_id:      string (UUID)
operator_id:     string (UUID)
sender_name:     string (sender ID or empty for shared)
segment_count:   int32
idempotency_key: string
```

### TarifyMessageResponse

```
approved:           bool
total_amount:       string (decimal as string for precision)
currency:           string (ISO 4217)
strategy:           string (applied strategy name)
tariff_plan_id:     string (UUID)
rejection_reason:   string (if approved=false)
threshold_crossed:  bool
recalc_amount:      string (recalculation amount, if any)
```

## Error Codes

| gRPC Code | Condition |
|-----------|-----------|
| INVALID_ARGUMENT | Missing required fields, invalid UUID format |
| NOT_FOUND | No active tariff plan for operator + category |
| FAILED_PRECONDITION | No active period, insufficient balance, plan deactivation blocked |
| ALREADY_EXISTS | Duplicate tariff plan for operator + category, duplicate idempotency key |
| INTERNAL | Saga failure, billing-service unavailable |

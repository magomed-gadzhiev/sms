# Contract: RoutingService gRPC API Extension

**Proto file**: `api/proto/routing/routing.proto` (extends existing)
**Package**: `routing`
**Go package**: `api/proto/routingv1`

## New RPCs (added to existing RoutingService)

### Country Management

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| CreateCountry | CreateCountryRequest | Country | Создать страну |
| GetCountry | GetCountryRequest | Country | Получить по ID |
| ListCountries | ListCountriesRequest | ListCountriesResponse | Список стран |
| UpdateCountry | UpdateCountryRequest | Country | Обновить страну |

### Operator Management

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| CreateOperator | CreateOperatorRequest | Operator | Создать оператора |
| GetOperator | GetOperatorRequest | Operator | Получить по ID |
| ListOperators | ListOperatorsRequest | ListOperatorsResponse | Список (фильтр по country_id) |
| UpdateOperator | UpdateOperatorRequest | Operator | Обновить оператора |

### Operator Prefix Management

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| CreateOperatorPrefix | CreateOperatorPrefixRequest | OperatorPrefix | Привязать префикс |
| ListOperatorPrefixes | ListOperatorPrefixesRequest | ListOperatorPrefixesResponse | Список (фильтр по operator_id) |
| DeleteOperatorPrefix | DeleteOperatorPrefixRequest | Empty | Удалить префикс |

### Operator Resolution

| RPC | Request | Response | Description |
|-----|---------|----------|-------------|
| ResolveOperator | ResolveOperatorRequest | ResolveOperatorResponse | Определить оператора по номеру |

## Key Messages

### ResolveOperatorRequest

```
phone_number: string (full international format, e.g. "+79001234567")
use_mnp:      bool   (attempt MNP lookup if available)
```

### ResolveOperatorResponse

```
operator_id:   string (UUID, empty if not found)
country_id:    string (UUID, empty if not found)
operator_code: string (e.g. "mts-ru")
country_code:  string (ISO 3166-1 alpha-2, e.g. "RU")
currency:      string (ISO 4217, e.g. "RUB")
resolved_by:   string ("prefix" or "mnp")
```

## Admin API Endpoints (admin-gateway)

### Countries

```
POST   /admin/v1/countries
GET    /admin/v1/countries
GET    /admin/v1/countries/{id}
PUT    /admin/v1/countries/{id}
```

### Operators

```
POST   /admin/v1/operators
GET    /admin/v1/operators?country_id={id}
GET    /admin/v1/operators/{id}
PUT    /admin/v1/operators/{id}
```

### Operator Prefixes

```
POST   /admin/v1/operators/{id}/prefixes
GET    /admin/v1/operators/{id}/prefixes
DELETE /admin/v1/operators/{id}/prefixes/{prefix_id}
```

### Tarification (proxied to tarification-service)

```
POST   /admin/v1/tarification/sender-registrations
GET    /admin/v1/tarification/sender-registrations?client_id=&operator_id=
PUT    /admin/v1/tarification/sender-registrations/{id}

POST   /admin/v1/tarification/tariff-plans
GET    /admin/v1/tarification/tariff-plans?operator_id=
PUT    /admin/v1/tarification/tariff-plans/{id}

POST   /admin/v1/tarification/tariff-periods
POST   /admin/v1/tarification/tariff-tiers
PUT    /admin/v1/tarification/tariff-tiers/{id}

POST   /admin/v1/tarification/pricing-periods
POST   /admin/v1/tarification/prepaid-fees

GET    /admin/v1/tarification/usage?client_id=&tariff_plan_id=
```

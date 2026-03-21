# Quickstart: Number Lookup (HLR/MNP) + Smart Routing

**Feature**: 004-hlr-smart-routing | **Date**: 2026-03-21

## Что делает эта фича

Добавляет HLR-lookup в pipeline отправки SMS: перед маршрутизацией определяется реальный оператор получателя (учитывая MNP), и SMS направляется через оптимального провайдера. Также предоставляется standalone API для валидации номеров как отдельный тарифицируемый продукт.

## Архитектурное решение

Расширение **routing-service** (не новый сервис). HLR-lookup — часть домена маршрутизации. Кеширование в Redis (TTL 24h). Логирование в PostgreSQL (partitioned, 90 дней retention).

## Ключевые компоненты

1. **HLR Service** (routing-service/application) — оркестрация: cache check → HLR provider query → fallback
2. **Smart Routing Service** (routing-service/application) — взвешенный выбор провайдера
3. **HLR Provider Adapter** (routing-service/infrastructure) — абстракция для внешних HLR-провайдеров
4. **HLR Cache** (routing-service/infrastructure) — Redis-based cache с TTL и invalidation
5. **Lookup API** (client-gateway) — HTTP endpoints для клиентов
6. **HLR Admin** (admin-gateway) — управление провайдерами и весами
7. **Lookup Portal** (portal-gateway) — история и статистика

## Поток данных

```
Client SMS Request
    → client-gateway (HTTP)
    → messaging-service (gRPC: SendMessage)
    → routing-service (gRPC: RouteMessage)
        → HLR Service:
            1. Check Redis cache (hlr:{msisdn})
            2. Cache miss → Query HLR provider (sync, ≤200ms timeout)
            3. Store result in Redis (TTL 24h)
            4. Log to lookup_log table
        → Smart Routing Service:
            5. Get operator from HLR result (or prefix fallback)
            6. Get available providers for this operator
            7. Calculate weighted score (cost_weight * cost + quality_weight * delivery_rate)
            8. Select provider with highest score
    → provider-service (gRPC: SendToProvider)
    → SMSC delivery
```

## Новые таблицы БД

- `hlr_providers` — конфигурация HLR-провайдеров
- `smart_route_weights` — настраиваемые веса per-operator/region
- `lookup_log` — аудит-лог (partitioned monthly, 90 дней retention)

## Новые Redis ключи

- `hlr:{msisdn}` — кешированный результат HLR (TTL 24h)

## Новые API endpoints

**Client Gateway** (`/api/v1`):
- `POST /lookup` — single number lookup
- `POST /lookup/bulk` — batch lookup (≤1000)
- `GET /lookup/history` — история запросов

**Admin Gateway** (`/admin/v1`):
- `POST|GET /hlr/providers` — CRUD HLR-провайдеров
- `PUT|DELETE /hlr/providers/{id}` — управление провайдером
- `POST|GET|DELETE /routing/weights` — CRUD весов smart routing

## Proto расширения

Файл `api/proto/routing/routing.proto` — новые RPC:
- `NumberLookup`, `BulkNumberLookup`
- `CreateHLRProvider`, `UpdateHLRProvider`, `DeleteHLRProvider`, `GetHLRProvider`, `ListHLRProviders`
- `SetSmartRouteWeights`, `GetSmartRouteWeights`, `ListSmartRouteWeights`, `DeleteSmartRouteWeights`
- `GetLookupHistory`
- Расширение `RouteMessageResponse` полями `hlr_result`, `hlr_used`, `routing_score`

## Kafka события

- `lookup.completed` — результат HLR-запроса (для analytics)
- Потребление `message.delivery_failed` — для cache invalidation при wrong_operator

## Метрики Prometheus

- `hlr_lookup_duration_seconds` (histogram) — latency HLR-запросов
- `hlr_cache_hits_total` / `hlr_cache_misses_total` (counters)
- `hlr_provider_requests_total` (counter, labels: provider, status)
- `hlr_provider_health` (gauge, labels: provider)
- `smart_routing_score` (histogram) — distribution of selected scores
- `lookup_api_requests_total` (counter, labels: type=single|bulk)

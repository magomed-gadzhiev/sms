# Research: Max Messenger Channel

**Feature**: 013-max-messenger-channel  
**Date**: 2026-04-01

## 1. Max Messenger Bot API Integration

**Decision**: Использовать HTTP REST API аналогично паттерну Flash Call adapter — прямой HTTP-вызов к провайдеру из Send().

**Rationale**: 
- Max Messenger Bot API работает по HTTP REST (отправка сообщений, проверка регистрации)
- Паттерн Flash Call adapter уже проверен: синхронный HTTP POST → асинхронный webhook с результатом
- SMS adapter использует Kafka (для SMPP), что избыточно для HTTP-based мессенджера

**Alternatives considered**:
- Kafka-based (как SMS): избыточно, добавляет лишний hop для HTTP API
- gRPC к отдельному микросервису: нарушает принцип VI (Simplicity) — нет нужды в отдельном сервисе

## 2. Проверка доступности (Reachability Check)

**Decision**: Использовать существующий `ReachabilityService` + расширить `operator_channel_support` записями для `max_messenger`. Дополнительно: Max Messenger API предоставляет endpoint проверки регистрации MSISDN — вызывать его из адаптера перед отправкой.

**Rationale**:
- `operator_channel_support` определяет "может ли оператор в принципе доставить через канал" — для Max Messenger это всегда true (канал не зависит от оператора, работает через интернет)
- Реальная проверка "зарегистрирован ли MSISDN в Max" — специфична для Max и должна быть в адаптере
- Redis-кеш с TTL 1h уже реализован в ReachabilityService

**Alternatives considered**:
- Только operator_channel_support: не покрывает проверку регистрации конкретного номера в Max
- Отдельный ReachabilityService для Max: нарушает принцип VI, можно интегрировать в существующий

## 3. Webhook обработка

**Decision**: Добавить HTTP endpoint `/webhooks/cascade/max_messenger` в cascade-service для приёма webhook-уведомлений от Max API. Верификация через HMAC-SHA256.

**Rationale**:
- Паттерн уже используется для Flash Call webhook
- HMAC-SHA256 — стандарт для верификации webhook (указан в spec FR-005)
- Endpoint публикует `CascadeAttemptResultEvent` в Kafka — существующий orchestrator обрабатывает дальше

**Alternatives considered**:
- Polling Max API за статусами: неэффективно, увеличивает нагрузку и латентность
- Отдельный webhook-сервис: нарушает принцип VI

## 4. Retry при Rate-Limiting (HTTP 429)

**Decision**: Экспоненциальный retry внутри `Send()` — до 3 попыток с backoff 1s→2s→4s (FR-013).

**Rationale**:
- Retry в Send() изолирует логику от orchestrator — адаптер сам обрабатывает rate-limit
- Экспоненциальный backoff — стандартная практика
- При исчерпании retry — возвращаем ошибку, orchestrator помечает attempt как failed

**Alternatives considered**:
- Retry на уровне orchestrator: усложняет общую логику, rate-limit специфичен для Max API

## 5. Типы контента (текст, изображения, кнопки)

**Decision**: Расширить `DeliveryAttempt` / delivery payload для поддержки rich content. В Send() адаптера формировать payload в формате Max Bot API (text + attachments + inline_keyboard).

**Rationale**:
- FR-002 требует: текст, изображения, кнопки (inline keyboard)
- Существующая модель `Delivery.Text` — только текст
- Дополнительные поля можно передавать через metadata/extras в delivery

**Alternatives considered**:
- Отдельная модель для rich messages: избыточно, достаточно расширить payload

## 6. Prometheus метрики

**Decision**: Использовать существующий `CascadeMetrics` — метрики автоматически получают label `channel_type=max_messenger`.

**Rationale**:
- `AttemptsTotal` уже параметризован по `channel_type`
- Дополнительно: добавить channel-specific метрики для Max (webhook counter, reachability cache hit/miss)

**Alternatives considered**:
- Отдельный metrics namespace: нарушает единообразие, существующие dashboard-ы не увидят метрики

## 7. Конфигурация канала

**Decision**: Хранить в `delivery_channels.config` (JSONB) со следующими полями:
```json
{
  "provider_url": "https://api.max.ru/bot/v1",
  "api_key": "bot_api_key_encrypted",
  "bot_id": "bot_identifier",
  "webhook_secret": "hmac_secret_encrypted",
  "check_registration_url": "https://api.max.ru/bot/v1/check"
}
```

**Rationale**:
- Соответствует паттерну Flash Call (JSONB config с encrypted sensitive fields)
- Все параметры подключения в одном месте
- Администратор настраивает через существующий UI каналов

**Alternatives considered**:
- Отдельная таблица для Max config: нарушает единообразие с другими каналами

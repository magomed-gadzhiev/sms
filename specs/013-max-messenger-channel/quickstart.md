# Quickstart: Max Messenger Channel

**Feature**: 013-max-messenger-channel

## Предусловия

- Cascade service (фича 012) полностью развёрнут и работает
- Docker Compose стек запущен (`docker compose up -d`)
- Max Messenger бот создан и зарегистрирован (есть API key, bot_id)

## Шаги запуска

### 1. Применить миграцию

```bash
scripts/server.sh migrate
```

Миграция 000071 добавит канал `max_messenger` (inactive) и записи в `operator_channel_support`.

### 2. Пересобрать cascade-service

```bash
scripts/server.sh deploy cascade-service
```

Новый адаптер Max Messenger компилируется вместе с сервисом.

### 3. Настроить канал через админ-панель

1. Войти в портал администратора
2. Перейти в **Каналы** → найти "Max Messenger"
3. Редактировать конфигурацию:
   ```json
   {
     "provider_url": "https://api.max.ru/bot/v1/messages",
     "api_key": "your_bot_api_key",
     "bot_id": "your_bot_id",
     "webhook_secret": "your_webhook_secret_min_32_chars",
     "check_registration_url": "https://api.max.ru/bot/v1/check"
   }
   ```
4. Активировать канал

### 4. Создать стратегию с Max Messenger

1. Перейти в **Стратегии доставки** → Создать
2. Задать имя, например "Max → SMS"
3. Режим: sequential
4. Шаги:
   - Шаг 1: Max Messenger, таймаут 20с, billable: true
   - Шаг 2: SMS, таймаут 30с, billable: true
5. Сохранить

### 5. Проверить работу

```bash
# Создать тестовую доставку через gRPC
grpcurl -plaintext -d '{
  "client_id": "test-client-uuid",
  "strategy_id": "strategy-uuid",
  "recipient": "+79001234567",
  "text": "Тестовое сообщение через Max",
  "sender_name": "TestBot",
  "request_id": "test-001"
}' localhost:9110 cascade.CascadeService/CreateDelivery
```

### 6. Проверить метрики

```bash
curl -s localhost:2132/metrics | grep max_messenger
```

Ожидаемые метрики:
- `cascade_attempts_total{channel_type="max_messenger",status="sent"}`
- `max_messenger_webhooks_received_total{status="valid"}`
- `max_messenger_reachability_checks_total{result="registered"}`

## Webhook endpoint

Max Messenger будет отправлять статусы доставки на:
```
POST https://your-domain/webhooks/cascade/max_messenger
```

Настройте этот URL в панели управления ботом Max Messenger.

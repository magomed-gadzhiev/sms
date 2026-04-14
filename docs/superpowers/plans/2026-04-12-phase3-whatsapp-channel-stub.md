# WhatsApp Business Channel — Implementation Plan (STUB)

> **STATUS: BLOCKED — требуется бизнес-решение перед реализацией**

**Goal:** Добавить WhatsApp Business Messages как канал в cascade-стратегии и standalone API. Клиенты смогут отправлять сообщения через WhatsApp с автоматическим fallback на SMS.

**Architecture:** Новый provider adapter `whatsapp-provider` подключается к существующей cascade-инфраструктуре (`delivery_channels`, `delivery_strategies`). Сообщения проходят через messaging service → routing → whatsapp provider → Meta Cloud API или BSP.

**Tech Stack:** Go 1.24, Meta WhatsApp Business API (Cloud API или BSP), existing cascade service, IBM/sarama Kafka.

---

## BLOCKERS — решить до начала разработки

### Блокер 1: Интеграционная модель
**Выбрать одну из двух опций:**

**Option A — Meta Cloud API (рекомендуется для старта)**
- Быстрый старт: регистрация через Meta Business Manager
- Зависимость от Meta: все запросы идут через Meta Cloud
- Pricing: платим Meta напрямую + наша маржа
- Timeline до старта разработки: 1-2 недели (получить WABA + phone number)
- Требует: Meta Business Verification (2-5 дней)

**Option B — BSP (Business Solution Provider)**
- Полный контроль: трафик через нашу инфраструктуру
- Сложнее: нужна сертификация BSP от Meta (3-6 месяцев, дорого)
- Маржа выше: контролируем pricing
- Рекомендуется только при объёмах >10M сообщений/месяц

**→ ACTION: Product Owner решает BSP vs Cloud API до 2026-04-20**

### Блокер 2: WhatsApp Business Account
Необходимо создать:
- [ ] Meta Business Manager account
- [ ] WhatsApp Business Account (WABA)
- [ ] Верифицированный бизнес-телефон для API
- [ ] Приложение в Meta Developer Portal с правами `whatsapp_business_messaging`
- [ ] Access Token (временный или system user token)

---

## Файловая карта (будет уточнена после разблокировки)

| Файл | Действие | Что делаем |
|---|---|---|
| `internal/services/provider/whatsapp/provider.go` | Create | WhatsApp provider adapter |
| `internal/services/provider/whatsapp/client.go` | Create | Meta Cloud API HTTP client |
| `internal/services/provider/whatsapp/webhook.go` | Create | Обработка входящих webhook от Meta |
| `api/proto/messaging/messaging.proto` | Modify | Добавить WhatsApp-специфичные поля |
| `migrations/` | Create | Добавить WhatsApp в delivery_channels |
| `internal/gateway/client/handlers/whatsapp.go` | Create | Webhook receiver для Meta |
| `portal-frontend/src/pages/cascade/` | Modify | Добавить WhatsApp в UI стратегий |

---

## Высокоуровневые задачи (детализируются после разблокировки)

1. **Реализовать WhatsApp Provider Adapter** — имплементировать интерфейс Provider с методами Send, GetStatus, HandleDLR
2. **Meta Cloud API Client** — аутентификация, отправка template messages, media, text
3. **Template Sync** — WhatsApp требует pre-approved templates; синхронизировать с нашей template-системой
4. **Webhook обработчик** — Meta посылает DLR на наш endpoint; обработать и публиковать в Kafka `sms.dlr`
5. **DB migration** — добавить `whatsapp` в delivery_channels с конфигурацией
6. **Cascade integration** — зарегистрировать провайдер в routing service
7. **Frontend** — добавить WhatsApp в UI построения cascade-стратегий

---

## Meta Cloud API — краткий справочник

После разблокировки использовать:

```
Base URL: https://graph.facebook.com/v18.0/{phone-number-id}/messages
Auth: Bearer {ACCESS_TOKEN}
Content-Type: application/json
```

Отправка template message:
```json
{
  "messaging_product": "whatsapp",
  "to": "79991234567",
  "type": "template",
  "template": {
    "name": "hello_world",
    "language": { "code": "ru" }
  }
}
```

Отправка text message (только для активных conversations):
```json
{
  "messaging_product": "whatsapp",
  "to": "79991234567",
  "type": "text",
  "text": { "body": "Ваш заказ подтверждён!" }
}
```

Webhook событие DLR:
```json
{
  "entry": [{
    "changes": [{
      "value": {
        "statuses": [{
          "id": "wamid.xxx",
          "status": "delivered",
          "timestamp": "1713859200",
          "recipient_id": "79991234567"
        }]
      }
    }]
  }]
}
```

---

## Как вернуться к этому плану

Когда бизнес-решения приняты и WABA настроен:
1. Запустить `/plan` или `superpowers:writing-plans` для детализации этого плана
2. Передать конкретную интеграционную модель и credentials для sandbox-тестирования
3. Исполнить через `superpowers:subagent-driven-development`

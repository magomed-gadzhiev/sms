# Contract: Cascade Service → Max Messenger Bot API

## Send Message

```
POST {provider_url}
Authorization: Bearer {api_key}
Content-Type: application/json
```

### Request Body

```json
{
  "recipient": "+79001234567",
  "text": "Ваш код: 1234",
  "image_url": "https://example.com/image.png",
  "inline_keyboard": [
    [
      {"text": "Подтвердить", "callback_data": "confirm"},
      {"text": "Отмена", "callback_data": "cancel"}
    ]
  ],
  "callback_url": "https://our-domain/webhooks/cascade/max_messenger",
  "external_id": "attempt-uuid"
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| recipient | string | Yes | MSISDN получателя (E.164) |
| text | string | No* | Текст сообщения |
| image_url | string | No | URL изображения |
| inline_keyboard | [][]Button | No | Кнопки (inline keyboard) |
| callback_url | string | Yes | URL для webhook-уведомлений о статусе |
| external_id | string | Yes | attempt_id для корреляции |

*Хотя бы одно из text/image_url обязательно.

### Button Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| text | string | Yes | Текст кнопки |
| url | string | No | URL для открытия |
| callback_data | string | No | Данные callback (макс. 64 символа) |

### Response (200)

```json
{
  "message_id": "max-msg-12345",
  "status": "sent"
}
```

### Error Responses

| HTTP | Описание | Действие адаптера |
|------|----------|-------------------|
| 400 | Bad request | Return error → attempt failed |
| 401 | Invalid API key | Return error → attempt failed |
| 403 | Bot blocked by user | Return error → attempt failed |
| 404 | User not found | Return error → attempt failed |
| 429 | Rate limited | Retry с backoff 1s→2s→4s (до 3 раз) |
| 500+ | Server error | Return error → attempt failed |

## Check Registration

```
POST {check_registration_url}
Authorization: Bearer {api_key}
Content-Type: application/json
```

### Request Body

```json
{
  "msisdn": "+79001234567"
}
```

### Response (200)

```json
{
  "registered": true,
  "user_id": "max-user-456"
}
```

### Error Handling

| HTTP | Описание | Действие |
|------|----------|----------|
| 200 + registered=false | Не зарегистрирован | Attempt → skipped |
| 200 + registered=true | Зарегистрирован | Продолжить отправку |
| 4xx/5xx | Ошибка API | Fail-open: считать доступным (FR-012) |
| Timeout | API не отвечает | Fail-open: считать доступным |

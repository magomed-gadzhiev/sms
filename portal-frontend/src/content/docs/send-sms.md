---
title: "Send SMS"
order: 3
---

# Send SMS

## Single message

```bash
POST /v1/messages

{
  "to": "+1234567890",
  "text": "Your verification code: 1234",
  "sender": "MyApp"
}
```

## Batch send

```bash
POST /v1/messages/batch

{
  "messages": [
    { "to": "+1234567890", "text": "Hello" },
    { "to": "+0987654321", "text": "World" }
  ],
  "sender": "MyApp"
}
```

## Response

```json
{
  "id": "msg_abc123",
  "status": "queued",
  "created_at": "2026-04-15T10:00:00Z"
}
```

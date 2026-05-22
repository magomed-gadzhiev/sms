---
title: "Authentication"
order: 2
---

# Authentication

All API requests require an API key passed in the `Authorization` header.

## API Keys

Generate keys in the portal: **Settings → API Keys**.

```bash
Authorization: Bearer YOUR_API_KEY
```

## Rate Limits

| Plan | Requests/second |
|------|----------------|
| Starter | 10 |
| Business | 100 |
| Enterprise | Custom |

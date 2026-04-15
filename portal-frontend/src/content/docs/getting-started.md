---
title: "Getting Started"
order: 1
---

# Getting Started

Welcome to the SMS Platform API. This guide will help you send your first SMS in minutes.

## Prerequisites

- An active account on SMS Platform
- An API key (generate in Settings → API Keys)

## Quick Start

### 1. Get your API key

After registering, navigate to **Settings → API Keys** and create a new key.

### 2. Send your first SMS

```bash
curl -X POST https://api.smsplatform.com/v1/messages \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "+1234567890",
    "text": "Hello from SMS Platform!",
    "sender": "MyCompany"
  }'
```

### 3. Check delivery status

```bash
curl https://api.smsplatform.com/v1/messages/{message_id} \
  -H "Authorization: Bearer YOUR_API_KEY"
```

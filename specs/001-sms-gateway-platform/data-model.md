# Data Model: SMS Gateway Platform

**Date**: 2026-03-20
**Feature**: 001-sms-gateway-platform

## Entities

### Message

Central entity representing a single SMS (or multipart logical message).

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique message identifier |
| client_id | UUID | FK → Client. Owner of the message |
| external_id | String (optional) | Client-provided idempotency/reference ID |
| source | String | Sender address (alphanumeric or phone number) |
| destination | String | Recipient phone number (E.164 format) |
| body | Text | Message content |
| status | Enum | pending, queued, sent, delivered, failed, expired, scheduled, cancelled |
| segment_count | Integer | Number of SMS segments (1 for <=160 chars) |
| provider_id | UUID (optional) | FK → Provider. Assigned after routing |
| smpp_message_id | String (optional) | Provider-assigned message ID |
| template_id | UUID (optional) | FK → Template. If sent via template |
| scheduled_at | Timestamp (optional) | Future delivery time (max 30 days ahead) |
| sent_at | Timestamp (optional) | When submitted to provider |
| delivered_at | Timestamp (optional) | When DLR received with success |
| failed_at | Timestamp (optional) | When marked as failed |
| expired_at | Timestamp (optional) | When DLR timeout triggered |
| created_at | Timestamp | Creation time (partition key) |
| updated_at | Timestamp | Last status change |

**Partitioning**: Monthly on `created_at`
**Retention**: 90 days (drop old partitions)

**State Transitions**:
```
                    ┌─────────────┐
                    │   pending    │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │  queued   │ │scheduled │ │  failed   │
        └────┬─────┘ └────┬─────┘ └──────────┘
             │            │
             │     ┌──────┴──────┐
             │     │             │
             ▼     ▼             ▼
        ┌──────────┐      ┌───────────┐
        │   sent    │      │ cancelled  │
        └────┬─────┘      └───────────┘
             │
     ┌───────┼───────┐
     │       │       │
     ▼       ▼       ▼
┌─────────┐ ┌──────┐ ┌────────┐
│delivered│ │failed│ │expired │
└─────────┘ └──────┘ └────────┘
```

**Valid transitions**:
- pending → queued (immediate send)
- pending → scheduled (has scheduled_at)
- pending → failed (validation/balance error)
- queued → sent (submitted to provider)
- queued → failed (routing/provider error)
- scheduled → queued (scheduler picks up)
- scheduled → cancelled (client cancels)
- sent → delivered (DLR success)
- sent → failed (DLR failure)
- sent → expired (DLR timeout, default 24h)

### DLR Receipt

Delivery receipt log from SMSC providers.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique receipt identifier |
| message_id | UUID | FK → Message |
| provider_id | UUID | FK → Provider |
| smpp_message_id | String | Provider message reference |
| status | String | Raw DLR status from provider |
| error_code | Integer (optional) | Provider error code |
| received_at | Timestamp | When receipt was received |
| raw_payload | Text | Original DLR PDU content |

### Client

API consumer account.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique client identifier |
| name | String | Display name |
| api_key | String | Authentication key (hashed) |
| api_secret | String | Authentication secret (hashed) |
| status | Enum | active, inactive, suspended |
| rate_limit_per_second | Integer | Max messages per second |
| rate_limit_per_minute | Integer | Max messages per minute |
| rate_limit_per_hour | Integer | Max messages per hour |
| allowed_sources | String[] | Allowed sender addresses |
| created_at | Timestamp | Account creation time |
| updated_at | Timestamp | Last modification |

### Provider

SMSC provider connection configuration.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique provider identifier |
| name | String | Display name |
| host | String | SMSC host address |
| port | Integer | SMSC port |
| system_id | String | SMPP bind credentials |
| password | String | SMPP bind password (encrypted) |
| bind_type | Enum | transmitter, receiver, transceiver |
| max_throughput | Integer | Max messages per second |
| status | Enum | active, inactive, degraded |
| priority | Integer | Selection priority |
| created_at | Timestamp | Creation time |
| updated_at | Timestamp | Last modification |

### Route

Routing rule mapping destinations to providers.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique route identifier |
| name | String | Display name |
| pattern | String | Destination regex/prefix pattern |
| priority | Integer | Rule evaluation order |
| strategy | Enum | round_robin, least_loaded, cheapest, explicit |
| provider_ids | UUID[] | Ordered list of provider IDs |
| failover_enabled | Boolean | Whether to try next provider on failure |
| max_retries | Integer | Max retry attempts |
| is_active | Boolean | Whether route is enabled |
| created_at | Timestamp | Creation time |
| updated_at | Timestamp | Last modification |

### Template

Pre-approved message template.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique template identifier |
| client_id | UUID | FK → Client. Template owner |
| name | String | Template display name |
| body | Text | Template body with `{{variable}}` placeholders |
| variables | String[] | List of required variable names |
| status | Enum | draft, approved, rejected |
| approved_by | UUID (optional) | Admin user who approved |
| approved_at | Timestamp (optional) | Approval timestamp |
| rejection_reason | String (optional) | Reason if rejected |
| created_at | Timestamp | Creation time |
| updated_at | Timestamp | Last modification |

**State Transitions**: draft → approved | draft → rejected | rejected → draft (re-submit)

### Webhook Subscription

Client-registered HTTP endpoint for delivery notifications.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique subscription identifier |
| client_id | UUID | FK → Client |
| url | String | Callback endpoint URL (HTTPS) |
| secret | String | HMAC-SHA256 signing secret |
| events | String[] | Subscribed event types (sent, delivered, failed, expired) |
| is_active | Boolean | Whether subscription is enabled |
| max_retries | Integer | Max delivery attempts (default: 5) |
| timeout_ms | Integer | HTTP request timeout in milliseconds |
| created_at | Timestamp | Creation time |
| updated_at | Timestamp | Last modification |

### Account (Billing)

Client billing account.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique account identifier |
| client_id | UUID | FK → Client (1:1) |
| balance | Decimal | Current balance |
| currency | String | Currency code (default: RUB) |
| created_at | Timestamp | Creation time |
| updated_at | Timestamp | Last balance change |

### Transaction

Billing transaction record.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique transaction identifier |
| account_id | UUID | FK → Account |
| type | Enum | credit, debit |
| amount | Decimal | Transaction amount |
| message_id | UUID (optional) | FK → Message (for debit) |
| segment_count | Integer | Number of segments charged |
| description | String | Transaction description |
| created_at | Timestamp | Transaction time |

### Pricing Rule

Destination-based pricing configuration.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique rule identifier |
| pattern | String | Destination regex pattern |
| price_per_segment | Decimal | Price per SMS segment |
| currency | String | Currency code |
| priority | Integer | Rule evaluation order |
| is_active | Boolean | Whether rule is enabled |
| created_at | Timestamp | Creation time |
| updated_at | Timestamp | Last modification |

### Audit Log

Administrative operation audit trail.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique log entry identifier |
| actor_id | UUID | User who performed the action |
| actor_type | Enum | admin, system |
| action | String | Operation performed |
| entity_type | String | Target entity type |
| entity_id | UUID | Target entity identifier |
| details | JSONB | Operation details / diff |
| created_at | Timestamp | Event time (partition key) |

**Partitioning**: Monthly on `created_at`
**Retention**: 1 year (drop old partitions)

## Relationships

```
Client 1──1 Account
Client 1──* Message
Client 1──* Template
Client 1──* Webhook Subscription
Account 1──* Transaction
Message *──1 Provider (optional, after routing)
Message *──1 Template (optional)
Message 1──* DLR Receipt
Route *──* Provider (ordered list)
Pricing Rule → matches destination pattern
```

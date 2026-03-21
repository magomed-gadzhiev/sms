# Data Model: Multi-tenant Self-Service Portal + Sub-accounts

**Feature Branch**: `003-self-service-portal`
**Date**: 2026-03-21

## Расширения существующих таблиц

### clients (расширение)

Новые поля к существующей таблице `clients`:

| Field | Type | Nullable | Default | Description |
|-------|------|----------|---------|-------------|
| parent_client_id | uuid | YES | NULL | FK → clients.id. NULL = обычный клиент/реселлер. Заполнено = sub-account |
| is_reseller | boolean | NO | false | Реселлерский статус (активируется администратором) |
| max_sub_accounts | integer | NO | 0 | Лимит sub-accounts (задаётся администратором при активации) |

**Constraints**:
- FK: `parent_client_id REFERENCES clients(id) ON DELETE RESTRICT`
- CHECK: `parent_client_id` не может ссылаться на клиента, у которого `parent_client_id IS NOT NULL` (один уровень)
- CHECK: `is_reseller = true` только если `parent_client_id IS NULL`
- INDEX: `idx_clients_parent_client_id ON clients(parent_client_id) WHERE parent_client_id IS NOT NULL`

**State transitions**:
- Обычный клиент → Реселлер: admin устанавливает `is_reseller=true`, `max_sub_accounts>0`
- Реселлер → Обычный клиент: только если нет активных sub-accounts

### users (расширение)

Новые поля к существующей таблице `users`:

| Field | Type | Nullable | Default | Description |
|-------|------|----------|---------|-------------|
| client_id | uuid | YES | NULL | FK → clients.id. Связь пользователя с клиентом |
| totp_secret_encrypted | bytea | YES | NULL | Зашифрованный TOTP-секрет (AES-256-GCM) |
| totp_enabled | boolean | NO | false | 2FA активирована |
| totp_verified_at | timestamptz | YES | NULL | Когда 2FA была подтверждена |

**Примечание**: Поле `client_id` может уже существовать в системе (user привязан к client через роль). Если нет — добавляется. Связь user ↔ client необходима для определения tenant'а.

### api_keys (расширение)

Новые поля к существующей таблице `api_keys`:

| Field | Type | Nullable | Default | Description |
|-------|------|----------|---------|-------------|
| allowed_ips | text[] | YES | NULL | Массив разрешённых IP/CIDR. NULL = без ограничений |

## Новые таблицы

### totp_recovery_codes

| Field | Type | Nullable | Default | Description |
|-------|------|----------|---------|-------------|
| id | uuid | NO | gen_random_uuid() | PK |
| user_id | uuid | NO | | FK → users.id ON DELETE CASCADE |
| code_hash | varchar(64) | NO | | SHA256 хеш recovery-кода |
| used | boolean | NO | false | Использован ли код |
| used_at | timestamptz | YES | NULL | Когда был использован |
| created_at | timestamptz | NO | NOW() | |

**Indexes**:
- UNIQUE: `(user_id, code_hash)`
- INDEX: `idx_totp_recovery_user_id ON totp_recovery_codes(user_id)`

### password_reset_tokens

| Field | Type | Nullable | Default | Description |
|-------|------|----------|---------|-------------|
| id | uuid | NO | gen_random_uuid() | PK |
| user_id | uuid | NO | | FK → users.id ON DELETE CASCADE |
| token_hash | varchar(64) | NO | | SHA256 хеш токена |
| expires_at | timestamptz | NO | | Время истечения (1 час) |
| used | boolean | NO | false | Использован ли |
| created_at | timestamptz | NO | NOW() | |

**Indexes**:
- UNIQUE: `(token_hash)`
- INDEX: `idx_password_reset_user_id ON password_reset_tokens(user_id)`

### sessions

| Field | Type | Nullable | Default | Description |
|-------|------|----------|---------|-------------|
| id | varchar(64) | NO | | PK. Session ID (crypto-random) |
| user_id | uuid | NO | | FK → users.id ON DELETE CASCADE |
| client_id | uuid | NO | | FK → clients.id |
| ip_address | inet | NO | | IP при создании сессии |
| user_agent | text | YES | NULL | User-Agent браузера |
| expires_at | timestamptz | NO | | Время истечения |
| created_at | timestamptz | NO | NOW() | |

**Примечание**: Основное хранилище сессий — Redis. Таблица `sessions` используется для отображения активных сессий пользователю и управления ими (logout all). Redis — source of truth для валидации, PostgreSQL — для аудита и UI.

**Indexes**:
- INDEX: `idx_sessions_user_id ON sessions(user_id)`
- INDEX: `idx_sessions_expires_at ON sessions(expires_at)`

### audit_log (monthly partitioned)

| Field | Type | Nullable | Default | Description |
|-------|------|----------|---------|-------------|
| id | uuid | NO | gen_random_uuid() | PK |
| tenant_id | uuid | NO | | Client/tenant ID |
| user_id | uuid | YES | NULL | Пользователь (NULL для системных действий) |
| action | varchar(50) | NO | | Тип действия (api_key.created, webhook.updated, etc.) |
| resource_type | varchar(50) | NO | | Тип ресурса (api_key, webhook, sub_account, etc.) |
| resource_id | varchar(255) | YES | NULL | ID ресурса |
| details | jsonb | YES | NULL | Дополнительные данные (JSON) |
| ip_address | inet | YES | NULL | IP-адрес |
| created_at | timestamptz | NO | NOW() | Partition key |

**Partitioning**: `PARTITION BY RANGE (created_at)` — ежемесячные партиции.

**Indexes**:
- INDEX: `idx_audit_log_tenant_id ON audit_log(tenant_id, created_at DESC)`
- INDEX: `idx_audit_log_action ON audit_log(action, created_at DESC)`
- INDEX: `idx_audit_log_user_id ON audit_log(user_id, created_at DESC) WHERE user_id IS NOT NULL`

**Retention**: 1 год (автоматическое удаление партиций старше 12 месяцев).

**Audit Actions (предопределённые)**:
- `auth.login`, `auth.logout`, `auth.password_reset`, `auth.totp_enabled`, `auth.totp_disabled`
- `api_key.created`, `api_key.revoked`
- `webhook.created`, `webhook.updated`, `webhook.deleted`, `webhook.test_sent`
- `sub_account.created`, `sub_account.deleted`, `sub_account.limit_updated`
- `balance.transfer_out`, `balance.transfer_in`
- `profile.updated`

### balance_transfers

| Field | Type | Nullable | Default | Description |
|-------|------|----------|---------|-------------|
| id | uuid | NO | gen_random_uuid() | PK |
| from_client_id | uuid | NO | | FK → clients.id (реселлер) |
| to_client_id | uuid | NO | | FK → clients.id (sub-account) |
| amount | numeric(15,4) | NO | | Сумма перевода |
| currency | varchar(3) | NO | | Валюта |
| from_transaction_id | uuid | NO | | FK → transactions.id (списание) |
| to_transaction_id | uuid | NO | | FK → transactions.id (зачисление) |
| created_at | timestamptz | NO | NOW() | |

**Indexes**:
- INDEX: `idx_balance_transfers_from ON balance_transfers(from_client_id, created_at DESC)`
- INDEX: `idx_balance_transfers_to ON balance_transfers(to_client_id, created_at DESC)`

## Entity Relationships

```text
clients (extended)
├── 1:N → clients (parent_client_id) [sub-accounts, max 1 level]
├── 1:1 → accounts (billing)
├── 1:N → users (auth)
│   ├── 1:N → api_keys (extended with allowed_ips)
│   ├── 1:N → totp_recovery_codes
│   ├── 1:N → password_reset_tokens
│   └── 1:N → sessions
├── 1:N → webhook_subscriptions
├── 1:N → messages
├── 1:N → transactions
├── 1:N → audit_log
└── 1:N → balance_transfers (as from/to)
```

## Domain Models (Go structs)

### Sub-account (client-service domain)

```go
// Расширение Client
type Client struct {
    // ... существующие поля ...
    ParentClientID *uuid.UUID `db:"parent_client_id"`
    IsReseller     bool       `db:"is_reseller"`
    MaxSubAccounts int        `db:"max_sub_accounts"`
}

func (c *Client) IsSubAccount() bool {
    return c.ParentClientID != nil
}

func (c *Client) CanCreateSubAccount(currentCount int) bool {
    return c.IsReseller && !c.IsSubAccount() && currentCount < c.MaxSubAccounts
}
```

### TOTP (auth-service domain)

```go
type TOTPConfig struct {
    UserID          uuid.UUID  `db:"user_id"`
    SecretEncrypted []byte     `db:"totp_secret_encrypted"`
    Enabled         bool       `db:"totp_enabled"`
    VerifiedAt      *time.Time `db:"totp_verified_at"`
}

type TOTPRecoveryCode struct {
    ID       uuid.UUID  `db:"id"`
    UserID   uuid.UUID  `db:"user_id"`
    CodeHash string     `db:"code_hash"`
    Used     bool       `db:"used"`
    UsedAt   *time.Time `db:"used_at"`
}
```

### Audit Event (shared)

```go
type AuditEvent struct {
    ID           string            `json:"event_id"`
    TenantID     uuid.UUID         `json:"tenant_id"`
    UserID       *uuid.UUID        `json:"user_id,omitempty"`
    Action       string            `json:"action"`
    ResourceType string            `json:"resource_type"`
    ResourceID   *string           `json:"resource_id,omitempty"`
    Details      map[string]any    `json:"details,omitempty"`
    IPAddress    *string           `json:"ip_address,omitempty"`
    Timestamp    time.Time         `json:"timestamp"`
}
```

### Balance Transfer (billing-service domain)

```go
type BalanceTransfer struct {
    ID                uuid.UUID `db:"id"`
    FromClientID      uuid.UUID `db:"from_client_id"`
    ToClientID        uuid.UUID `db:"to_client_id"`
    Amount            string    `db:"amount"`
    Currency          string    `db:"currency"`
    FromTransactionID uuid.UUID `db:"from_transaction_id"`
    ToTransactionID   uuid.UUID `db:"to_transaction_id"`
    CreatedAt         time.Time `db:"created_at"`
}
```

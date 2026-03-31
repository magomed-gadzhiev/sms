# Data Model: Sender Names & Message Templates (010)

## Overview

This document describes the data model for the sender name registration and message template management feature. The feature adds:

- `sender_names` — new table for registered alphanumeric/numeric sender names per client
- `sender_name_status_history` — audit trail for all sender name status transitions
- `templates.sender_name_id` — optional link from a template to an approved sender name

---

## Tables

### sender_names

Stores registered sender names belonging to clients. Each name is unique per client.

```sql
CREATE TABLE sender_names (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id        UUID         NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name             VARCHAR(15)  NOT NULL,
    status           VARCHAR(20)  NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'approved', 'rejected', 'deactivated')),
    rejection_reason TEXT,
    reviewer_id      UUID         REFERENCES users(id),
    reviewed_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (client_id, name)
);
```

**Indexes:**

```sql
-- Primary lookup: all sender names for a client
CREATE INDEX idx_sender_names_client_id ON sender_names (client_id);

-- Filter by status (e.g. admin review queue)
CREATE INDEX idx_sender_names_status ON sender_names (status);

-- Admin review queue: pending names ordered by submission time
CREATE INDEX idx_sender_names_client_status ON sender_names (client_id, status);
```

**Column notes:**

| Column            | Notes                                                                    |
|-------------------|--------------------------------------------------------------------------|
| `id`              | UUID v4, surrogate PK                                                    |
| `client_id`       | Owner client; cascades on client deletion                                |
| `name`            | 1–11 chars alphanumeric or 1–15 digits; see validation rules below       |
| `status`          | Lifecycle state; constrained by CHECK; default `pending`                 |
| `rejection_reason`| Populated by admin when status moves to `rejected`                       |
| `reviewer_id`     | Admin user who last changed the status; NULL until first admin action    |
| `reviewed_at`     | Timestamp of last admin action                                           |
| `created_at`      | Set once at INSERT                                                       |
| `updated_at`      | Updated on every modification via trigger or application layer           |

---

### sender_name_status_history

Append-only audit log. One row is inserted for every status transition on a sender name.

```sql
CREATE TABLE sender_name_status_history (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    sender_name_id UUID        NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
    old_status     VARCHAR(20),
    new_status     VARCHAR(20) NOT NULL,
    actor_id       UUID,
    actor_type     VARCHAR(20) NOT NULL CHECK (actor_type IN ('client', 'admin', 'system')),
    comment        TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Indexes:**

```sql
-- Fetch full history for a sender name
CREATE INDEX idx_snsh_sender_name_id ON sender_name_status_history (sender_name_id);

-- Audit queries by actor
CREATE INDEX idx_snsh_actor_id ON sender_name_status_history (actor_id)
    WHERE actor_id IS NOT NULL;
```

**Column notes:**

| Column          | Notes                                                                         |
|-----------------|-------------------------------------------------------------------------------|
| `old_status`    | NULL for the initial `pending` entry (no previous state)                      |
| `new_status`    | The status after the transition                                               |
| `actor_id`      | UUID of the user or client who triggered the change; NULL for system actions  |
| `actor_type`    | Discriminator: `client` (resubmit), `admin` (approve/reject/deactivate), `system` |
| `comment`       | Optional human-readable note attached to the transition                       |

---

### templates (modification)

The existing `templates` table gains one nullable foreign key column.

```sql
ALTER TABLE templates
    ADD COLUMN sender_name_id UUID REFERENCES sender_names(id) ON DELETE SET NULL;
```

**Index:**

```sql
-- Look up all templates using a specific sender name
CREATE INDEX idx_templates_sender_name_id ON templates (sender_name_id)
    WHERE sender_name_id IS NOT NULL;
```

**Existing columns** (for reference — not changed):

| Column             | Type         | Notes                                        |
|--------------------|--------------|----------------------------------------------|
| `id`               | UUID         | PK                                           |
| `client_id`        | UUID         | Owner client                                 |
| `name`             | TEXT         | Human-readable template name                 |
| `body`             | TEXT         | Template body with `{{variable}}` placeholders |
| `variables`        | JSONB        | Variable schema                              |
| `status`           | VARCHAR      | draft / pending / approved / rejected / review / revision_requested |
| `rejection_reason` | TEXT         |                                              |
| `reviewer_id`      | UUID         |                                              |
| `review_comment`   | TEXT         |                                              |
| `reviewed_at`      | TIMESTAMPTZ  |                                              |
| `created_at`       | TIMESTAMPTZ  |                                              |
| `updated_at`       | TIMESTAMPTZ  |                                              |
| `sender_name_id`   | UUID         | **NEW** — nullable FK to `sender_names.id`; SET NULL on delete |

---

## Go Domain Models

```go
// SenderName represents a registered sender identifier belonging to a client.
type SenderName struct {
    ID              uuid.UUID  `db:"id"`
    ClientID        uuid.UUID  `db:"client_id"`
    Name            string     `db:"name"`
    Status          string     `db:"status"`          // pending | approved | rejected | deactivated
    RejectionReason string     `db:"rejection_reason"`
    ReviewerID      *uuid.UUID `db:"reviewer_id"`
    ReviewedAt      *time.Time `db:"reviewed_at"`
    CreatedAt       time.Time  `db:"created_at"`
    UpdatedAt       time.Time  `db:"updated_at"`
}

// SenderNameStatusHistory is one immutable record of a status transition.
type SenderNameStatusHistory struct {
    ID           uuid.UUID  `db:"id"`
    SenderNameID uuid.UUID  `db:"sender_name_id"`
    OldStatus    *string    `db:"old_status"`   // nil for the initial creation record
    NewStatus    string     `db:"new_status"`
    ActorID      *uuid.UUID `db:"actor_id"`
    ActorType    string     `db:"actor_type"`   // client | admin | system
    Comment      string     `db:"comment"`
    CreatedAt    time.Time  `db:"created_at"`
}
```

Status constants:

```go
const (
    SenderNameStatusPending     = "pending"
    SenderNameStatusApproved    = "approved"
    SenderNameStatusRejected    = "rejected"
    SenderNameStatusDeactivated = "deactivated"

    ActorTypeClient = "client"
    ActorTypeAdmin  = "admin"
    ActorTypeSystem = "system"
)
```

---

## Validation Rules

### Name format

Two mutually exclusive formats are accepted:

| Format      | Pattern              | Max length | Example      |
|-------------|----------------------|------------|--------------|
| Alphanumeric | `^[A-Za-z0-9 ]{1,11}$` | 11 chars  | `MyBrand`    |
| Numeric      | `^\d{1,15}$`         | 15 digits  | `79001234567` |

Additional constraints:
- An alphanumeric name must not consist entirely of spaces.
- Name is case-sensitive at the DB level (the UNIQUE constraint on `(client_id, name)` is case-sensitive by default in PostgreSQL).

### Uniqueness

- `(client_id, name)` must be unique across all statuses, including `rejected` and `deactivated` entries. A client cannot re-register a name that already exists in any state; they must resubmit the existing rejected record instead.

### Template ↔ SenderName link

When a template is created or updated with a non-NULL `sender_name_id`:

1. The referenced `sender_names.id` must exist.
2. `sender_names.client_id` must equal `templates.client_id`.
3. `sender_names.status` must be `approved`.

Violations are rejected at the application layer (HTTP 422) before the INSERT/UPDATE reaches the database.

---

## SenderName Status Lifecycle

```
             ┌─────────────────────────────────────────┐
             │                                         │
      submit │                                         │ resubmit (client)
             ▼                                         │
          ┌──────────┐   approve (admin)   ┌───────────┴──┐
 [new] ──►│ pending  │───────────────────►│   approved   │
          └──────────┘                    └──────────────┘
               │                                │
               │ reject (admin)                 │ deactivate (admin)
               ▼                                ▼
          ┌──────────┐                   ┌─────────────┐
          │ rejected │                   │ deactivated │
          └──────────┘                   └─────────────┘
               │
               └────────────────────────► pending  (resubmit by client)
```

### Allowed transitions

| From          | To            | Actor  | Trigger                          |
|---------------|---------------|--------|----------------------------------|
| *(none)*      | `pending`     | client | Initial registration             |
| `pending`     | `approved`    | admin  | Admin approves the name          |
| `pending`     | `rejected`    | admin  | Admin rejects with reason        |
| `rejected`    | `pending`     | client | Client resubmits for review      |
| `approved`    | `deactivated` | admin  | Admin deactivates an active name |

Any other transition is invalid and must be rejected at the application layer (HTTP 422).

---

## Migration

The schema changes are applied in order:

```sql
-- 001: create sender_names
CREATE TABLE sender_names ( ... );  -- full DDL above

-- 002: create sender_name_status_history
CREATE TABLE sender_name_status_history ( ... );  -- full DDL above

-- 003: add FK to templates
ALTER TABLE templates
    ADD COLUMN sender_name_id UUID REFERENCES sender_names(id) ON DELETE SET NULL;

-- 004: indexes
CREATE INDEX idx_sender_names_client_id       ON sender_names (client_id);
CREATE INDEX idx_sender_names_status          ON sender_names (status);
CREATE INDEX idx_sender_names_client_status   ON sender_names (client_id, status);
CREATE INDEX idx_snsh_sender_name_id          ON sender_name_status_history (sender_name_id);
CREATE INDEX idx_snsh_actor_id                ON sender_name_status_history (actor_id) WHERE actor_id IS NOT NULL;
CREATE INDEX idx_templates_sender_name_id     ON templates (sender_name_id) WHERE sender_name_id IS NOT NULL;
```

All steps run inside a single transaction so the migration is atomic.

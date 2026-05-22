-- Migration 000064: Sender Names & Message Templates Registration
-- Creates sender_names table, sender_name_status_history table,
-- and adds sender_name_id FK to templates.

BEGIN;

-- 1. sender_names: registered sender identifiers per client
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

-- 2. sender_name_status_history: append-only audit log of status transitions
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

-- 3. Add sender_name_id FK to templates
ALTER TABLE templates
    ADD COLUMN sender_name_id UUID REFERENCES sender_names(id) ON DELETE SET NULL;

-- 4. Indexes
CREATE INDEX idx_sender_names_client_id     ON sender_names (client_id);
CREATE INDEX idx_sender_names_status        ON sender_names (status);
CREATE INDEX idx_sender_names_client_status ON sender_names (client_id, status);

CREATE INDEX idx_snsh_sender_name_id ON sender_name_status_history (sender_name_id);
CREATE INDEX idx_snsh_actor_id       ON sender_name_status_history (actor_id)
    WHERE actor_id IS NOT NULL;

CREATE INDEX idx_templates_sender_name_id ON templates (sender_name_id)
    WHERE sender_name_id IS NOT NULL;

COMMIT;

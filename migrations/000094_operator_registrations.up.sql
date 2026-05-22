BEGIN;

CREATE TABLE operator_registrations (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_name_id    UUID         NOT NULL REFERENCES sender_names(id) ON DELETE CASCADE,
    operator_id       UUID         NOT NULL REFERENCES operators(id),
    registration_type VARCHAR(16)  NOT NULL CHECK (registration_type IN ('free', 'paid')),
    status            VARCHAR(32)  NOT NULL DEFAULT 'submitted'
                          CHECK (status IN ('submitted', 'approved', 'rejected', 'revision_requested')),
    approved_type     VARCHAR(16)  CHECK (approved_type IN ('free', 'paid')),
    approved_at       TIMESTAMPTZ,
    moderator_note    TEXT,
    submitted_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT operator_registrations_unique UNIQUE (sender_name_id, operator_id)
);

CREATE TABLE operator_registration_history (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_registration_id UUID        NOT NULL REFERENCES operator_registrations(id) ON DELETE CASCADE,
    old_status               VARCHAR(32),
    new_status               VARCHAR(32) NOT NULL,
    actor_id                 UUID,
    actor_type               VARCHAR(20) NOT NULL CHECK (actor_type IN ('client', 'aggregator', 'admin', 'system')),
    comment                  TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_opreg_sender_name_id ON operator_registrations (sender_name_id);
CREATE INDEX idx_opreg_status         ON operator_registrations (status);
CREATE INDEX idx_opreg_operator_id    ON operator_registrations (operator_id);
CREATE INDEX idx_opreg_hist_reg_id    ON operator_registration_history (operator_registration_id);

COMMIT;

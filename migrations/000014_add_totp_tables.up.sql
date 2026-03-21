-- Add TOTP (Time-based One-Time Password) support to users table
ALTER TABLE users
    ADD COLUMN totp_secret_encrypted BYTEA DEFAULT NULL,
    ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN totp_verified_at TIMESTAMPTZ DEFAULT NULL;

-- Recovery codes for TOTP
CREATE TABLE totp_recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash VARCHAR(64) NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, code_hash)
);

CREATE INDEX idx_totp_recovery_user_id ON totp_recovery_codes(user_id);

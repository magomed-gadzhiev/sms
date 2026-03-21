DROP TABLE IF EXISTS totp_recovery_codes;

ALTER TABLE users
    DROP COLUMN IF EXISTS totp_secret_encrypted,
    DROP COLUMN IF EXISTS totp_enabled,
    DROP COLUMN IF EXISTS totp_verified_at;

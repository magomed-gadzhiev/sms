CREATE TABLE registration_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL,
    token       VARCHAR(128) NOT NULL UNIQUE,
    used        BOOLEAN NOT NULL DEFAULT false,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_registration_tokens_token ON registration_tokens (token) WHERE used = false;
CREATE INDEX idx_registration_tokens_email ON registration_tokens (email);

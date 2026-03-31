CREATE TABLE IF NOT EXISTS operator_prefixes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
    prefix      VARCHAR(20) NOT NULL,
    priority    INT NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(prefix)
);

CREATE INDEX idx_operator_prefixes_active ON operator_prefixes(prefix) WHERE active = true;

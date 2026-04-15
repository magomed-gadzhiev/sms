BEGIN;

CREATE TABLE IF NOT EXISTS aggregator_tariffs (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id    UUID        NOT NULL REFERENCES clients(id),
    sub_account_id   UUID        REFERENCES clients(id),
    operator_id      UUID        NOT NULL REFERENCES operators(id),
    sender_category  VARCHAR(32) NOT NULL DEFAULT 'standard',
    price_per_segment NUMERIC(10,6) NOT NULL,
    active           BOOLEAN     NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agg_tariff_unique
    ON aggregator_tariffs (aggregator_id, COALESCE(sub_account_id, '00000000-0000-0000-0000-000000000000'::uuid), operator_id, sender_category)
    WHERE active = true;

CREATE INDEX IF NOT EXISTS idx_agg_tariff_aggregator ON aggregator_tariffs (aggregator_id);
CREATE INDEX IF NOT EXISTS idx_agg_tariff_sub_account ON aggregator_tariffs (sub_account_id) WHERE sub_account_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS aggregator_margin_log (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregator_id    UUID        NOT NULL REFERENCES clients(id),
    sub_account_id   UUID        NOT NULL REFERENCES clients(id),
    message_id       UUID        NOT NULL,
    operator_id      UUID        NOT NULL,
    segment_count    INT         NOT NULL DEFAULT 1,
    sub_account_price NUMERIC(10,6) NOT NULL,
    aggregator_price  NUMERIC(10,6) NOT NULL,
    sub_account_total NUMERIC(10,6) NOT NULL,
    aggregator_total  NUMERIC(10,6) NOT NULL,
    margin            NUMERIC(10,6) NOT NULL,
    idempotency_key   VARCHAR(255) NOT NULL UNIQUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agg_margin_aggregator ON aggregator_margin_log (aggregator_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agg_margin_sub_account ON aggregator_margin_log (sub_account_id, created_at DESC);

COMMIT;

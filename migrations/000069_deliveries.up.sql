-- Partitioned table: deliveries
CREATE TABLE deliveries (
    id            UUID          NOT NULL DEFAULT gen_random_uuid(),
    client_id     UUID          NOT NULL,
    message_id    UUID,
    strategy_id   UUID          NOT NULL REFERENCES delivery_strategies(id),
    recipient     TEXT          NOT NULL,
    text          TEXT          NOT NULL,
    sender_name   TEXT,
    status        TEXT          NOT NULL DEFAULT 'pending',
    current_step  INT           NOT NULL DEFAULT 1,
    delivered_via TEXT,
    total_cost    NUMERIC(12,4) DEFAULT 0,
    currency      TEXT          NOT NULL DEFAULT 'RUB',
    request_id    TEXT,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE deliveries_2026_03 PARTITION OF deliveries
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE deliveries_2026_04 PARTITION OF deliveries
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE deliveries_2026_05 PARTITION OF deliveries
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE INDEX idx_deliveries_client_created ON deliveries (client_id, created_at);
CREATE INDEX idx_deliveries_status         ON deliveries (status);

-- Partitioned table: delivery_attempts
CREATE TABLE delivery_attempts (
    id            UUID          NOT NULL DEFAULT gen_random_uuid(),
    delivery_id   UUID          NOT NULL,
    channel_id    UUID          NOT NULL,
    channel_type  TEXT          NOT NULL,
    step_order    INT           NOT NULL,
    status        TEXT          NOT NULL DEFAULT 'pending',
    provider_ref  TEXT,
    cost          NUMERIC(12,4) DEFAULT 0,
    currency      TEXT          NOT NULL DEFAULT 'RUB',
    error_message TEXT,
    sent_at       TIMESTAMPTZ,
    result_at     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),

    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE delivery_attempts_2026_03 PARTITION OF delivery_attempts
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE delivery_attempts_2026_04 PARTITION OF delivery_attempts
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE delivery_attempts_2026_05 PARTITION OF delivery_attempts
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE INDEX idx_delivery_attempts_delivery_id ON delivery_attempts (delivery_id);
CREATE INDEX idx_delivery_attempts_status      ON delivery_attempts (status);
CREATE INDEX idx_delivery_attempts_created     ON delivery_attempts (created_at);

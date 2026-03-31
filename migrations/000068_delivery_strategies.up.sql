CREATE TABLE delivery_strategies (
    id          UUID        NOT NULL DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    description TEXT,
    mode        TEXT        NOT NULL CHECK (mode IN ('sequential', 'parallel')),
    active      BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id),
    UNIQUE (name)
);

CREATE TABLE delivery_strategy_steps (
    id          UUID        NOT NULL DEFAULT gen_random_uuid(),
    strategy_id UUID        NOT NULL REFERENCES delivery_strategies(id) ON DELETE CASCADE,
    channel_id  UUID        NOT NULL REFERENCES delivery_channels(id),
    step_order  INT         NOT NULL,
    timeout_s   INT         NOT NULL DEFAULT 30,
    billable    BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id),
    UNIQUE (strategy_id, step_order),
    UNIQUE (strategy_id, channel_id)
);

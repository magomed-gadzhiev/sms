CREATE TABLE IF NOT EXISTS route_set_condition_groups (
    id          BIGSERIAL PRIMARY KEY,
    item_id     UUID NOT NULL REFERENCES reseller_route_set_items(id) ON DELETE CASCADE,
    group_index SMALLINT NOT NULL,
    logic_op    VARCHAR(10) NOT NULL,  -- 'IF', 'AND', 'AND_NOT', 'OR', 'OR_NOT'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rscg_item ON route_set_condition_groups(item_id);

CREATE TABLE IF NOT EXISTS route_set_conditions (
    id              BIGSERIAL PRIMARY KEY,
    group_id        BIGINT NOT NULL REFERENCES route_set_condition_groups(id) ON DELETE CASCADE,
    condition_type  VARCHAR(30) NOT NULL,  -- 'operator','country','traffic_type','paid_name','regex'
    condition_value TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rsc_group ON route_set_conditions(group_id);

CREATE TABLE IF NOT EXISTS route_set_schedules (
    id         BIGSERIAL PRIMARY KEY,
    item_id    UUID NOT NULL REFERENCES reseller_route_set_items(id) ON DELETE CASCADE,
    date_from  DATE,
    date_to    DATE,
    time_from  TIME,
    time_to    TIME,
    weekdays   SMALLINT NOT NULL DEFAULT 127,
    timezone   VARCHAR(50) NOT NULL DEFAULT 'Europe/Moscow',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_rss_item ON route_set_schedules(item_id);

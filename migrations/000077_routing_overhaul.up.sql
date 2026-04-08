-- migrations/000077_routing_overhaul.up.sql

-- 1. Extend client_routes
ALTER TABLE client_routes
  ADD COLUMN name VARCHAR(255),
  ADD COLUMN comment TEXT,
  ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active',
  ADD COLUMN share INTEGER NOT NULL DEFAULT 100,
  ADD COLUMN route_type VARCHAR(10) NOT NULL DEFAULT 'sms';

-- Allow client_id to be NULL (NULL = default route)
ALTER TABLE client_routes ALTER COLUMN client_id DROP NOT NULL;

-- Drop the old unique constraint that requires client_id
-- (existing constraint: UNIQUE(client_id, operator_id, provider_id))
ALTER TABLE client_routes DROP CONSTRAINT IF EXISTS client_routes_client_id_operator_id_provider_id_key;

-- 2. Condition groups
CREATE TABLE route_condition_groups (
  id          BIGSERIAL PRIMARY KEY,
  route_id    UUID NOT NULL REFERENCES client_routes(id) ON DELETE CASCADE,
  group_index SMALLINT NOT NULL,
  logic_op    VARCHAR(10) NOT NULL,  -- 'IF', 'AND', 'AND_NOT', 'OR', 'OR_NOT'
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rcg_route ON route_condition_groups(route_id);

-- 3. Conditions
CREATE TABLE route_conditions (
  id              BIGSERIAL PRIMARY KEY,
  group_id        BIGINT NOT NULL REFERENCES route_condition_groups(id) ON DELETE CASCADE,
  condition_type  VARCHAR(30) NOT NULL,  -- 'operator', 'country', 'traffic_type', 'paid_name', 'regex'
  condition_value TEXT NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rc_group ON route_conditions(group_id);

-- 4. Schedules
CREATE TABLE route_schedules (
  id         BIGSERIAL PRIMARY KEY,
  route_id   UUID NOT NULL REFERENCES client_routes(id) ON DELETE CASCADE,
  date_from  DATE,
  date_to    DATE,
  time_from  TIME,
  time_to    TIME,
  weekdays   SMALLINT NOT NULL DEFAULT 127,  -- bitmask: 1=Mon, 2=Tue, 4=Wed, 8=Thu, 16=Fri, 32=Sat, 64=Sun; 127=all
  timezone   VARCHAR(50) NOT NULL DEFAULT 'Europe/Moscow',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_rs_route ON route_schedules(route_id);

-- 5. Add traffic_type to templates
ALTER TABLE templates ADD COLUMN traffic_type VARCHAR(20) NOT NULL DEFAULT 'transactional';

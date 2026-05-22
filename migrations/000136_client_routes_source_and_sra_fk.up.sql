-- Add source column to client_routes (template = materialized from route-set, override = sub-account custom)
ALTER TABLE client_routes
    ADD COLUMN IF NOT EXISTS source VARCHAR(10) NOT NULL DEFAULT 'override'
        CHECK (source IN ('template','override'));

-- Backfill: existing rows are 'override' (no route-set assignments yet, see spec §3.2).
-- The DEFAULT covers them; nothing else to do.

CREATE INDEX IF NOT EXISTS idx_client_routes_client_source
    ON client_routes (client_id, source) WHERE active = true;

-- Add FK on subaccount_routing_assignment.route_set_id (was loose UUID in Plan 1 task 1).
-- ON DELETE SET NULL: при удалении route-set'а assignment теряет ссылку (нужно ручное переназначение).
ALTER TABLE subaccount_routing_assignment
    ADD CONSTRAINT fk_sra_route_set
    FOREIGN KEY (route_set_id) REFERENCES reseller_route_sets(id) ON DELETE SET NULL;

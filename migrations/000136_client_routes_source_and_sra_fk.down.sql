ALTER TABLE subaccount_routing_assignment DROP CONSTRAINT IF EXISTS fk_sra_route_set;
DROP INDEX IF EXISTS idx_client_routes_client_source;
ALTER TABLE client_routes DROP COLUMN IF EXISTS source;

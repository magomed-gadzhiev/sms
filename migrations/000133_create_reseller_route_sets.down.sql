DROP TRIGGER IF EXISTS update_reseller_route_sets_updated_at ON reseller_route_sets;
DROP INDEX IF EXISTS uq_reseller_route_sets_default;
DROP INDEX IF EXISTS idx_reseller_route_sets_reseller;
DROP TABLE IF EXISTS reseller_route_sets;

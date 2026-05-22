DROP TRIGGER IF EXISTS update_reseller_route_set_items_updated_at ON reseller_route_set_items;
DROP INDEX IF EXISTS idx_reseller_route_set_items_set;
DROP INDEX IF EXISTS idx_reseller_route_set_items_provider;
DROP TABLE IF EXISTS reseller_route_set_items;

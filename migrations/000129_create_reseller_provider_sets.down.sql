DROP TRIGGER IF EXISTS update_reseller_provider_sets_updated_at ON reseller_provider_sets;
DROP INDEX IF EXISTS uq_reseller_provider_sets_default;
DROP INDEX IF EXISTS idx_reseller_provider_sets_reseller;
DROP TABLE IF EXISTS reseller_provider_sets;

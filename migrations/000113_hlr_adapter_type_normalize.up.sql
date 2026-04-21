-- Normalize legacy 'http' adapter_type values to canonical 'http_rest'.
-- Spec 004-hlr-smart-routing/data-model.md defines canonical set: http_rest, diameter, ss7.
-- The old seed (test/load/fixtures/seed.sql) wrote 'http', which the factory in
-- internal/services/routing/infrastructure/hlr_provider_adapter.go does not accept,
-- causing every NumberLookup + health check to fail with
-- "неподдерживаемый тип адаптера: http".
UPDATE hlr_providers
SET adapter_type = 'http_rest', updated_at = NOW()
WHERE adapter_type = 'http';

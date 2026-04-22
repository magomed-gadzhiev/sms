-- Reverse normalization. Not strictly idempotent (loses any genuinely
-- 'http_rest' rows that existed before the up), but acceptable because:
-- (a) seed data is the only source of 'http_rest' in current environments,
-- (b) the factory's alias ("http" -> HTTPHLRAdapter) keeps things functional either way.
UPDATE hlr_providers
SET adapter_type = 'http', updated_at = NOW()
WHERE adapter_type = 'http_rest';

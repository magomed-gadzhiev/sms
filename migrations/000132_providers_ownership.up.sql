-- Plan 1 Task 6: добавляем ownership / source_client_id в providers,
-- чтобы handler'ы /reseller/network/providers могли отличать платформенные
-- провайдеры от приватных (создаваемых агрегатором).
--
-- ownership = 'platform' (default, NOT NULL) | 'private' (создан агрегатором).
-- source_client_id = id агрегатора-владельца для private; NULL для platform.

ALTER TABLE providers
    ADD COLUMN IF NOT EXISTS ownership VARCHAR(20) NOT NULL DEFAULT 'platform',
    ADD COLUMN IF NOT EXISTS source_client_id UUID NULL REFERENCES clients(id) ON DELETE CASCADE;

ALTER TABLE providers
    DROP CONSTRAINT IF EXISTS providers_ownership_check;
ALTER TABLE providers
    ADD CONSTRAINT providers_ownership_check
    CHECK (ownership IN ('platform', 'private'));

-- private обязан иметь владельца, platform — обязан НЕ иметь.
ALTER TABLE providers
    DROP CONSTRAINT IF EXISTS providers_ownership_source_consistency;
ALTER TABLE providers
    ADD CONSTRAINT providers_ownership_source_consistency
    CHECK (
        (ownership = 'private' AND source_client_id IS NOT NULL)
        OR (ownership = 'platform' AND source_client_id IS NULL)
    );

CREATE INDEX IF NOT EXISTS idx_providers_source_client_id
    ON providers (source_client_id)
    WHERE source_client_id IS NOT NULL;

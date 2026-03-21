ALTER TABLE clients DROP CONSTRAINT IF EXISTS chk_reseller_is_top_level;

DROP INDEX IF EXISTS idx_clients_parent_client_id;

ALTER TABLE clients
    DROP COLUMN IF EXISTS parent_client_id,
    DROP COLUMN IF EXISTS is_reseller,
    DROP COLUMN IF EXISTS max_sub_accounts;

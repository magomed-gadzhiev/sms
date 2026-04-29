DROP INDEX IF EXISTS idx_transactions_client_kind_created;

ALTER TABLE transactions
    DROP COLUMN IF EXISTS operation_kind;

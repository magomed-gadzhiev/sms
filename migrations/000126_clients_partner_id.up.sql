-- BUG-83-followup / B.1: deterministic partner_id mapping for tenant isolation
-- in network_stats_hourly. Each client gets a stable bigint used as bucket key.
--
-- Sub-accounts (parent_client_id IS NOT NULL) keep their own partner_id in
-- clients.partner_id (UNIQUE per row), but aggregation queries and the
-- portal-gateway resolver always collapse a sub-account's effective partner_id
-- to its parent's, so reseller analytics aggregate the whole tree under one
-- bucket. See aggregation_worker.go and handlers/network_statistics.go.

CREATE SEQUENCE IF NOT EXISTS clients_partner_id_seq START WITH 1;

ALTER TABLE clients ADD COLUMN IF NOT EXISTS partner_id BIGINT;

UPDATE clients
SET partner_id = nextval('clients_partner_id_seq')
WHERE partner_id IS NULL;

ALTER TABLE clients
    ALTER COLUMN partner_id SET NOT NULL,
    ALTER COLUMN partner_id SET DEFAULT nextval('clients_partner_id_seq');

ALTER SEQUENCE clients_partner_id_seq OWNED BY clients.partner_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_partner_id_unique
    ON clients (partner_id);

-- Pre-migration aggregations were all written with hardcoded partner_id=0
-- (aggregation_worker.go before this fix). After the migration, no real
-- client gets partner_id=0 (sequence starts at 1), so bucket-0 rows are
-- both unreachable AND would constitute a cross-tenant leak surface if any
-- code path ever queried them. Drop the junk bucket only — leave correctly
-- bucketed rows the worker may have already written between deploy and
-- migration apply.
DELETE FROM network_stats_hourly WHERE partner_id = 0;

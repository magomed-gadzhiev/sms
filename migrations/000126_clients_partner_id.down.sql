DROP INDEX IF EXISTS idx_clients_partner_id_unique;
ALTER TABLE clients DROP COLUMN IF EXISTS partner_id;
DROP SEQUENCE IF EXISTS clients_partner_id_seq;

-- Drop dead billing_mode column from clients table.
-- After Phase 2 dual-charge cleanup (commit c19d6f8e) no Go code reads
-- this column for decisions: all sub-accounts go through ChargeMessageDual
-- unconditionally. The column was introduced by 000097_aggregator_quotas
-- and is now a zombie field.
ALTER TABLE clients DROP COLUMN IF EXISTS billing_mode;

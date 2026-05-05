-- No-op down: original CREATE TABLE migrations were destroyed when slots 000095/000096
-- were reused. There is no canonical schema definition to restore. If anyone needs
-- audit-log functionality in the future, write a fresh migration under the current
-- architecture (likely tied to is_reseller / sub-account model), not a restore of
-- the abandoned design.
SELECT 1;

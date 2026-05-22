DROP INDEX IF EXISTS idx_operators_mcc_mnc;
DROP INDEX IF EXISTS idx_countries_mcc;

ALTER TABLE operators
    DROP COLUMN IF EXISTS mnc,
    DROP COLUMN IF EXISTS mcc;

ALTER TABLE countries
    DROP COLUMN IF EXISTS mcc;

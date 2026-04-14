BEGIN;

ALTER TABLE sender_names DROP COLUMN IF EXISTS company_id;
ALTER TABLE accounts     DROP COLUMN IF EXISTS company_id;

DROP TABLE IF EXISTS client_companies;
DROP TABLE IF EXISTS companies;

COMMIT;

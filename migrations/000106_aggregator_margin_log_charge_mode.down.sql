BEGIN;

ALTER TABLE aggregator_margin_log
    DROP CONSTRAINT IF EXISTS chk_segments_sum,
    DROP COLUMN IF EXISTS overage_segments,
    DROP COLUMN IF EXISTS pool_segments,
    DROP COLUMN IF EXISTS charge_mode;

COMMIT;

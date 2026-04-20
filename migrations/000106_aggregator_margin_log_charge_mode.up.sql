BEGIN;

ALTER TABLE aggregator_margin_log
    ADD COLUMN charge_mode VARCHAR(16) NOT NULL DEFAULT 'pool'
        CONSTRAINT chk_charge_mode CHECK (charge_mode IN ('pool', 'overage', 'split')),
    ADD COLUMN pool_segments INTEGER NOT NULL DEFAULT 0
        CONSTRAINT chk_pool_segments_nonneg CHECK (pool_segments >= 0),
    ADD COLUMN overage_segments INTEGER NOT NULL DEFAULT 0
        CONSTRAINT chk_overage_segments_nonneg CHECK (overage_segments >= 0);

-- Backfill: исторические записи получают pool_segments = segment_count (все сегменты в пуле),
-- overage_segments = 0 (charge_mode='pool' уже проставлен дефолтом).
-- Это необходимо до добавления chk_segments_sum, иначе CHECK не пройдёт на существующих rows.
UPDATE aggregator_margin_log
SET pool_segments = segment_count
WHERE pool_segments = 0 AND segment_count > 0;

ALTER TABLE aggregator_margin_log
    ADD CONSTRAINT chk_segments_sum CHECK (pool_segments + overage_segments = segment_count);

COMMIT;

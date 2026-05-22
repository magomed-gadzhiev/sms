-- Revert: make end_date required again
-- NOTE: will fail if any rows have NULL end_date

ALTER TABLE reseller_tariff_periods DROP CONSTRAINT IF EXISTS reseller_periods_no_overlap;
ALTER TABLE reseller_tariff_periods DROP CONSTRAINT IF EXISTS reseller_tariff_periods_check;

ALTER TABLE reseller_tariff_periods ALTER COLUMN end_date SET NOT NULL;

ALTER TABLE reseller_tariff_periods ADD CONSTRAINT reseller_tariff_periods_check
    CHECK (end_date > start_date);

ALTER TABLE reseller_tariff_periods ADD CONSTRAINT reseller_periods_no_overlap
    EXCLUDE USING gist (tariff_plan_id WITH =, daterange(start_date, end_date, '[]') WITH &&);

-- Make end_date optional in reseller_tariff_periods.
-- A NULL end_date means "open-ended" — the next period's start_date closes the previous one.

-- 1. Drop existing constraints that require end_date NOT NULL
ALTER TABLE reseller_tariff_periods DROP CONSTRAINT IF EXISTS reseller_periods_no_overlap;
ALTER TABLE reseller_tariff_periods DROP CONSTRAINT IF EXISTS reseller_tariff_periods_check;

-- 2. Allow NULL end_date
ALTER TABLE reseller_tariff_periods ALTER COLUMN end_date DROP NOT NULL;

-- 3. Re-add CHECK: end_date > start_date only when end_date is set
ALTER TABLE reseller_tariff_periods ADD CONSTRAINT reseller_tariff_periods_check
    CHECK (end_date IS NULL OR end_date > start_date);

-- 4. Re-add overlap exclusion using upper-unbounded range for NULL end_date
ALTER TABLE reseller_tariff_periods ADD CONSTRAINT reseller_periods_no_overlap
    EXCLUDE USING gist (
        tariff_plan_id WITH =,
        daterange(start_date, COALESCE(end_date, 'infinity'::date), '[]') WITH &&
    );

-- MVP-feedback №8: добавляем колонки MCC/MNC в operators и MCC в countries.
-- Frontend (admin.ts:165) уже шлёт эти поля; до миграции они тихо игнорировались.
--
-- Колонки NULL-able: операторы без MCC/MNC возможны (виртуальные/MVNO в стадии
-- выделения, тестовые/sandbox записи).

ALTER TABLE countries
    ADD COLUMN IF NOT EXISTS mcc VARCHAR(3) NULL;

CREATE INDEX IF NOT EXISTS idx_countries_mcc
    ON countries(mcc)
    WHERE mcc IS NOT NULL;

ALTER TABLE operators
    ADD COLUMN IF NOT EXISTS mcc VARCHAR(3) NULL,
    ADD COLUMN IF NOT EXISTS mnc VARCHAR(3) NULL;

-- UNIQUE-индекс — частичный, только для записей с обоими заполненными.
-- Это позволяет хранить MVNO-записи с NULL'ами без конфликтов.
CREATE UNIQUE INDEX IF NOT EXISTS idx_operators_mcc_mnc
    ON operators(mcc, mnc)
    WHERE mcc IS NOT NULL AND mnc IS NOT NULL;

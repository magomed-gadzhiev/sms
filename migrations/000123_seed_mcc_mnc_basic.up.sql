-- MVP-feedback №8: seed справочника MCC/MNC для основных операторов СНГ
-- и нескольких крупных международных. Источник — публичный реестр ITU/
-- mcc-mnc community с сверкой по основным операторам.
--
-- Политика: UPSERT по iso_code (страны) и (mcc, mnc) (операторы).
-- Если запись уже есть с теми же ключами — обновляем имя (на случай
-- переименования/ребрендинга). Прочие поля (description, supports_*sender)
-- не трогаем — пользовательские правки сохраняются.

-- ─── Россия (MCC=250) ──────────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Россия', 'RU', '7', 'RUB', '250')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='RU'), 'МТС',     'MTS_RU',     '250', '01', true),
    ((SELECT id FROM countries WHERE iso_code='RU'), 'МегаФон', 'MEGAFON_RU', '250', '02', true),
    ((SELECT id FROM countries WHERE iso_code='RU'), 'НСС',     'NSS_RU',     '250', '03', true),
    ((SELECT id FROM countries WHERE iso_code='RU'), 'Yota',    'YOTA_RU',    '250', '11', true),
    ((SELECT id FROM countries WHERE iso_code='RU'), 'Tele2',   'TELE2_RU',   '250', '20', true),
    ((SELECT id FROM countries WHERE iso_code='RU'), 'Билайн',  'BEELINE_RU', '250', '99', true)
ON CONFLICT (code) DO UPDATE SET
    mcc = EXCLUDED.mcc,
    mnc = EXCLUDED.mnc,
    name = EXCLUDED.name;

-- ─── Казахстан (MCC=401) ───────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Казахстан', 'KZ', '7', 'KZT', '401')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='KZ'), 'Beeline KZ', 'BEELINE_KZ', '401', '01', true),
    ((SELECT id FROM countries WHERE iso_code='KZ'), 'Kcell',      'KCELL_KZ',   '401', '02', true),
    ((SELECT id FROM countries WHERE iso_code='KZ'), 'Tele2 KZ',   'TELE2_KZ',   '401', '77', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Беларусь (MCC=257) ────────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Беларусь', 'BY', '375', 'BYN', '257')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='BY'), 'A1',  'A1_BY',  '257', '01', true),
    ((SELECT id FROM countries WHERE iso_code='BY'), 'MTS', 'MTS_BY', '257', '02', true),
    ((SELECT id FROM countries WHERE iso_code='BY'), 'life:)', 'LIFE_BY', '257', '04', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Украина (MCC=255) ─────────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Украина', 'UA', '380', 'UAH', '255')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='UA'), 'Kyivstar',  'KYIVSTAR_UA', '255', '03', true),
    ((SELECT id FROM countries WHERE iso_code='UA'), 'Vodafone Ukraine', 'VODAFONE_UA', '255', '01', true),
    ((SELECT id FROM countries WHERE iso_code='UA'), 'lifecell',  'LIFECELL_UA', '255', '06', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Узбекистан (MCC=434) ──────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Узбекистан', 'UZ', '998', 'UZS', '434')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='UZ'), 'Beeline UZ',  'BEELINE_UZ', '434', '04', true),
    ((SELECT id FROM countries WHERE iso_code='UZ'), 'Ucell',       'UCELL_UZ',   '434', '05', true),
    ((SELECT id FROM countries WHERE iso_code='UZ'), 'Mobiuz',      'MOBIUZ_UZ',  '434', '07', true),
    ((SELECT id FROM countries WHERE iso_code='UZ'), 'Uzmobile',    'UZMOBILE_UZ','434', '01', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Кыргызстан (MCC=437) ──────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Кыргызстан', 'KG', '996', 'KGS', '437')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='KG'), 'Beeline KG', 'BEELINE_KG', '437', '01', true),
    ((SELECT id FROM countries WHERE iso_code='KG'), 'O!',         'O_KG',       '437', '09', true),
    ((SELECT id FROM countries WHERE iso_code='KG'), 'MegaCom',    'MEGACOM_KG', '437', '05', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Таджикистан (MCC=436) ─────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Таджикистан', 'TJ', '992', 'TJS', '436')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='TJ'), 'Tcell',     'TCELL_TJ',    '436', '03', true),
    ((SELECT id FROM countries WHERE iso_code='TJ'), 'Megafon TJ','MEGAFON_TJ',  '436', '01', true),
    ((SELECT id FROM countries WHERE iso_code='TJ'), 'Babilon-M', 'BABILON_TJ',  '436', '04', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Армения (MCC=283) ─────────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Армения', 'AM', '374', 'AMD', '283')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='AM'), 'Team Telecom', 'TEAM_AM',  '283', '04', true),
    ((SELECT id FROM countries WHERE iso_code='AM'), 'Viva-MTS',     'VIVAMTS_AM','283', '05', true),
    ((SELECT id FROM countries WHERE iso_code='AM'), 'Ucom',         'UCOM_AM',  '283', '10', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Грузия (MCC=282) ──────────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Грузия', 'GE', '995', 'GEL', '282')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='GE'), 'Magti',     'MAGTI_GE',     '282', '01', true),
    ((SELECT id FROM countries WHERE iso_code='GE'), 'Geocell',   'GEOCELL_GE',   '282', '02', true),
    ((SELECT id FROM countries WHERE iso_code='GE'), 'Beeline GE','BEELINE_GE',   '282', '04', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Азербайджан (MCC=400) ─────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Азербайджан', 'AZ', '994', 'AZN', '400')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='AZ'), 'Azercell',  'AZERCELL_AZ',  '400', '01', true),
    ((SELECT id FROM countries WHERE iso_code='AZ'), 'Bakcell',   'BAKCELL_AZ',   '400', '02', true),
    ((SELECT id FROM countries WHERE iso_code='AZ'), 'Nar',       'NAR_AZ',       '400', '04', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

-- ─── Молдова (MCC=259) ─────────────────────────────────────────────────
INSERT INTO countries (name, iso_code, phone_code, currency, mcc) VALUES
    ('Молдова', 'MD', '373', 'MDL', '259')
ON CONFLICT (iso_code) DO UPDATE SET mcc = EXCLUDED.mcc, name = EXCLUDED.name;

INSERT INTO operators (country_id, name, code, mcc, mnc, active) VALUES
    ((SELECT id FROM countries WHERE iso_code='MD'), 'Orange MD', 'ORANGE_MD', '259', '01', true),
    ((SELECT id FROM countries WHERE iso_code='MD'), 'Moldcell',  'MOLDCELL_MD','259', '02', true)
ON CONFLICT (code) DO UPDATE SET mcc = EXCLUDED.mcc, mnc = EXCLUDED.mnc, name = EXCLUDED.name;

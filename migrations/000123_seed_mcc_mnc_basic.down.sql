-- Откат seed: удаляем только операторы, добавленные миграцией 000123,
-- по их code (он генерируется из <NAME>_<COUNTRY_ISO>). Существующие
-- пользовательские записи не трогаем.

DELETE FROM operators
WHERE code IN (
    'MTS_RU', 'MEGAFON_RU', 'NSS_RU', 'YOTA_RU', 'TELE2_RU', 'BEELINE_RU',
    'BEELINE_KZ', 'KCELL_KZ', 'TELE2_KZ',
    'A1_BY', 'MTS_BY', 'LIFE_BY',
    'KYIVSTAR_UA', 'VODAFONE_UA', 'LIFECELL_UA',
    'BEELINE_UZ', 'UCELL_UZ', 'MOBIUZ_UZ', 'UZMOBILE_UZ',
    'BEELINE_KG', 'O_KG', 'MEGACOM_KG',
    'TCELL_TJ', 'MEGAFON_TJ', 'BABILON_TJ',
    'TEAM_AM', 'VIVAMTS_AM', 'UCOM_AM',
    'MAGTI_GE', 'GEOCELL_GE', 'BEELINE_GE',
    'AZERCELL_AZ', 'BAKCELL_AZ', 'NAR_AZ',
    'ORANGE_MD', 'MOLDCELL_MD'
);

-- countries не удаляем: они могут быть использованы в других ссылках
-- (sender_name_billing_records, providers и т.п.). Только обнуляем mcc.
UPDATE countries SET mcc = NULL
WHERE iso_code IN ('RU','KZ','BY','UA','UZ','KG','TJ','AM','GE','AZ','MD');

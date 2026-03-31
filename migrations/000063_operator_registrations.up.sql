-- Регистрация шаблонов SMS у операторов связи России
CREATE TABLE IF NOT EXISTS operator_template_registrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    operator_id UUID NOT NULL REFERENCES operators(id),
    external_id VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'blocked')),
    rejection_reason TEXT,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (template_id, operator_id)
);

CREATE INDEX IF NOT EXISTS idx_otr_template_id ON operator_template_registrations(template_id);
CREATE INDEX IF NOT EXISTS idx_otr_operator_id ON operator_template_registrations(operator_id);
CREATE INDEX IF NOT EXISTS idx_otr_status       ON operator_template_registrations(status);

CREATE TRIGGER update_otr_updated_at BEFORE UPDATE ON operator_template_registrations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- -------------------------------------------------------------------------
-- Seed: российские операторы связи (МТС, Билайн, МегаФон, Теле2)
-- Используем ON CONFLICT DO NOTHING, чтобы не дублировать при повторном применении
-- -------------------------------------------------------------------------
INSERT INTO countries (id, name, iso_code, phone_code, currency)
VALUES ('00000000-0000-0000-0000-000000000001', 'Россия', 'RU', '7', 'RUB')
ON CONFLICT (iso_code) DO NOTHING;

-- Получаем id страны Россия для вставки операторов
DO $$
DECLARE
    ru_country_id UUID;
BEGIN
    SELECT id INTO ru_country_id FROM countries WHERE iso_code = 'RU' LIMIT 1;

    INSERT INTO operators (id, country_id, name, code, supports_paid_sender, supports_free_sender, active)
    VALUES
        ('10000000-0000-0000-0000-000000000001', ru_country_id, 'МТС',      'MTS',    true,  false, true),
        ('10000000-0000-0000-0000-000000000002', ru_country_id, 'Билайн',   'BEELINE', true, false, true),
        ('10000000-0000-0000-0000-000000000003', ru_country_id, 'МегаФон',  'MEGAFON', true, false, true),
        ('10000000-0000-0000-0000-000000000004', ru_country_id, 'Теле2',    'TELE2',   true, false, true),
        ('10000000-0000-0000-0000-000000000005', ru_country_id, 'Ростелеком', 'ROSTELECOM', true, false, true)
    ON CONFLICT (code) DO NOTHING;

    -- Префиксы МТС
    INSERT INTO operator_prefixes (operator_id, prefix, priority)
    VALUES
        ('10000000-0000-0000-0000-000000000001', '7910', 10),
        ('10000000-0000-0000-0000-000000000001', '7911', 10),
        ('10000000-0000-0000-0000-000000000001', '7912', 10),
        ('10000000-0000-0000-0000-000000000001', '7913', 10),
        ('10000000-0000-0000-0000-000000000001', '7914', 10),
        ('10000000-0000-0000-0000-000000000001', '7915', 10),
        ('10000000-0000-0000-0000-000000000001', '7916', 10),
        ('10000000-0000-0000-0000-000000000001', '7917', 10),
        ('10000000-0000-0000-0000-000000000001', '7918', 10),
        ('10000000-0000-0000-0000-000000000001', '7919', 10)
    ON CONFLICT (prefix) DO NOTHING;

    -- Префиксы Билайн
    INSERT INTO operator_prefixes (operator_id, prefix, priority)
    VALUES
        ('10000000-0000-0000-0000-000000000002', '7960', 10),
        ('10000000-0000-0000-0000-000000000002', '7961', 10),
        ('10000000-0000-0000-0000-000000000002', '7962', 10),
        ('10000000-0000-0000-0000-000000000002', '7963', 10),
        ('10000000-0000-0000-0000-000000000002', '7964', 10),
        ('10000000-0000-0000-0000-000000000002', '7965', 10),
        ('10000000-0000-0000-0000-000000000002', '7966', 10),
        ('10000000-0000-0000-0000-000000000002', '7967', 10),
        ('10000000-0000-0000-0000-000000000002', '7968', 10),
        ('10000000-0000-0000-0000-000000000002', '7969', 10)
    ON CONFLICT (prefix) DO NOTHING;

    -- Префиксы МегаФон
    INSERT INTO operator_prefixes (operator_id, prefix, priority)
    VALUES
        ('10000000-0000-0000-0000-000000000003', '7920', 10),
        ('10000000-0000-0000-0000-000000000003', '7921', 10),
        ('10000000-0000-0000-0000-000000000003', '7922', 10),
        ('10000000-0000-0000-0000-000000000003', '7923', 10),
        ('10000000-0000-0000-0000-000000000003', '7924', 10),
        ('10000000-0000-0000-0000-000000000003', '7925', 10),
        ('10000000-0000-0000-0000-000000000003', '7926', 10),
        ('10000000-0000-0000-0000-000000000003', '7927', 10),
        ('10000000-0000-0000-0000-000000000003', '7928', 10),
        ('10000000-0000-0000-0000-000000000003', '7929', 10)
    ON CONFLICT (prefix) DO NOTHING;

    -- Префиксы Теле2
    INSERT INTO operator_prefixes (operator_id, prefix, priority)
    VALUES
        ('10000000-0000-0000-0000-000000000004', '7900', 10),
        ('10000000-0000-0000-0000-000000000004', '7901', 10),
        ('10000000-0000-0000-0000-000000000004', '7902', 10),
        ('10000000-0000-0000-0000-000000000004', '7903', 10),
        ('10000000-0000-0000-0000-000000000004', '7904', 10),
        ('10000000-0000-0000-0000-000000000004', '7905', 10),
        ('10000000-0000-0000-0000-000000000004', '7906', 10)
    ON CONFLICT (prefix) DO NOTHING;
END $$;

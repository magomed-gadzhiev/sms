BEGIN;

-- 1. companies: юридические реквизиты и флаг оферты
CREATE TABLE companies (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    inn              VARCHAR(12),
    name             VARCHAR(255) NOT NULL,
    full_name        VARCHAR(500),
    kpp              VARCHAR(9),
    ogrn             VARCHAR(15),
    legal_address    TEXT,
    actual_address   TEXT,
    ceo_name         VARCHAR(255),
    ceo_title        VARCHAR(255),
    acting_basis     VARCHAR(255),
    bank_name        VARCHAR(255),
    bank_bik         VARCHAR(9),
    bank_corr_account VARCHAR(20),
    bank_account     VARCHAR(20),
    email            VARCHAR(255),
    phone            VARCHAR(20),
    is_offer         BOOLEAN      NOT NULL DEFAULT FALSE,
    active           BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_companies_inn ON companies (inn) WHERE inn IS NOT NULL;

-- 2. client_companies: N:N связь клиент ↔ компания
CREATE TABLE client_companies (
    client_id  UUID        NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    company_id UUID        NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    is_default BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (client_id, company_id)
);

-- ровно одна основная компания на клиента
CREATE UNIQUE INDEX idx_client_companies_default
    ON client_companies (client_id)
    WHERE is_default = TRUE;

CREATE INDEX idx_client_companies_company_id ON client_companies (company_id);

-- 3. Привязать accounts к компании
ALTER TABLE accounts ADD COLUMN company_id UUID REFERENCES companies(id);
CREATE INDEX idx_accounts_company_id ON accounts (company_id) WHERE company_id IS NOT NULL;

-- 4. Для каждого существующего клиента создать компанию «Оферта»
--    и привязать к ней существующий account
DO $$
DECLARE
    r RECORD;
    new_company_id UUID;
BEGIN
    FOR r IN SELECT id FROM clients LOOP
        INSERT INTO companies (name, is_offer)
        VALUES ('Оферта', TRUE)
        RETURNING id INTO new_company_id;

        INSERT INTO client_companies (client_id, company_id, is_default)
        VALUES (r.id, new_company_id, TRUE);

        UPDATE accounts
        SET company_id = new_company_id
        WHERE client_id = r.id;
    END LOOP;
END $$;

-- 5. sender_names: добавить company_id (nullable сначала, заполнить, затем NOT NULL)
ALTER TABLE sender_names ADD COLUMN company_id UUID REFERENCES companies(id);

UPDATE sender_names sn
SET company_id = cc.company_id
FROM client_companies cc
WHERE cc.client_id = sn.client_id
  AND cc.is_default = TRUE;

ALTER TABLE sender_names ALTER COLUMN company_id SET NOT NULL;
CREATE INDEX idx_sender_names_company_id ON sender_names (company_id);

COMMIT;

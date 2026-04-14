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

-- 3. Привязать accounts к компании (nullable до заполнения)
ALTER TABLE accounts ADD COLUMN company_id UUID REFERENCES companies(id);
CREATE INDEX idx_accounts_company_id ON accounts (company_id) WHERE company_id IS NOT NULL;

-- 4. Для каждого существующего клиента создать компанию «Оферта»
--    и привязать к ней существующий account (set-based через temp-column)

-- Step A: добавить временную колонку для сопоставления клиента → компании
ALTER TABLE companies ADD COLUMN _tmp_client_id UUID;

-- Step B: вставить по одной Оферта-компании на каждого клиента
INSERT INTO companies (name, is_offer, _tmp_client_id)
SELECT 'Оферта', TRUE, id
FROM clients;

-- Step C: создать связи client_companies
INSERT INTO client_companies (client_id, company_id, is_default)
SELECT _tmp_client_id, id, TRUE
FROM companies
WHERE _tmp_client_id IS NOT NULL;

-- Step D: привязать accounts к соответствующей компании
UPDATE accounts a
SET company_id = co.id
FROM companies co
WHERE co._tmp_client_id = a.client_id
  AND co.is_offer = TRUE;

-- Step E: удалить временную колонку
ALTER TABLE companies DROP COLUMN _tmp_client_id;

-- 5. Проверить, что все accounts привязаны, и сделать NOT NULL
DO $$
DECLARE unlinked_count INT;
BEGIN
    SELECT COUNT(*) INTO unlinked_count FROM accounts WHERE company_id IS NULL;
    IF unlinked_count > 0 THEN
        RAISE EXCEPTION 'Migration failed: % account(s) have no company_id', unlinked_count;
    END IF;
END $$;

ALTER TABLE accounts ALTER COLUMN company_id SET NOT NULL;

-- 6. sender_names: добавить company_id (nullable сначала, заполнить, затем NOT NULL)
ALTER TABLE sender_names ADD COLUMN company_id UUID REFERENCES companies(id);

UPDATE sender_names sn
SET company_id = cc.company_id
FROM client_companies cc
WHERE cc.client_id = sn.client_id
  AND cc.is_default = TRUE;

-- Проверить, что все sender_names привязаны
DO $$
DECLARE unlinked_count INT;
BEGIN
    SELECT COUNT(*) INTO unlinked_count FROM sender_names WHERE company_id IS NULL;
    IF unlinked_count > 0 THEN
        RAISE EXCEPTION 'Migration failed: % sender_name(s) have no company_id', unlinked_count;
    END IF;
END $$;

ALTER TABLE sender_names ALTER COLUMN company_id SET NOT NULL;
CREATE INDEX idx_sender_names_company_id ON sender_names (company_id);

-- 7. Триггер updated_at для companies
CREATE TRIGGER update_companies_updated_at
    BEFORE UPDATE ON companies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;

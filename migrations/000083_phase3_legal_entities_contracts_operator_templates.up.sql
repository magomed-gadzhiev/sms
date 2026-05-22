-- Phase 3: Legal entities, contracts, and operator templates

-- Legal entities (юридические лица)
CREATE TABLE IF NOT EXISTS legal_entities (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  inn        VARCHAR(12)  NOT NULL,
  name       VARCHAR(255) NOT NULL,
  full_name  TEXT,
  address    TEXT,
  active     BOOLEAN      NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_legal_entities_inn ON legal_entities(inn);
CREATE INDEX IF NOT EXISTS idx_legal_entities_active ON legal_entities(active);

-- Operator ↔ legal entity associations
CREATE TABLE IF NOT EXISTS operator_legal_entities (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  legal_entity_id UUID NOT NULL REFERENCES legal_entities(id) ON DELETE CASCADE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (operator_id, legal_entity_id)
);

CREATE INDEX IF NOT EXISTS idx_ole_operator ON operator_legal_entities(operator_id);
CREATE INDEX IF NOT EXISTS idx_ole_legal_entity ON operator_legal_entities(legal_entity_id);

-- Contracts (договоры)
CREATE TABLE IF NOT EXISTS contracts (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_number  VARCHAR(100) NOT NULL,
  client_id        UUID         NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
  legal_entity_id  UUID         REFERENCES legal_entities(id) ON DELETE SET NULL,
  status           VARCHAR(20)  NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active', 'expired', 'terminated')),
  start_date       DATE         NOT NULL,
  end_date         DATE,
  description      TEXT,
  created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contracts_client ON contracts(client_id);
CREATE INDEX IF NOT EXISTS idx_contracts_legal_entity ON contracts(legal_entity_id);
CREATE INDEX IF NOT EXISTS idx_contracts_status ON contracts(status);

-- Operator templates (шаблоны операторов)
CREATE TABLE IF NOT EXISTS operator_templates (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name           VARCHAR(255) NOT NULL,
  operator_id    UUID         NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
  sender_name_id UUID         REFERENCES sender_names(id) ON DELETE SET NULL,
  body           TEXT         NOT NULL,
  variables      JSONB        NOT NULL DEFAULT '[]',
  status         VARCHAR(20)  NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'inactive', 'pending')),
  created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_operator_templates_operator ON operator_templates(operator_id);
CREATE INDEX IF NOT EXISTS idx_operator_templates_sender_name ON operator_templates(sender_name_id);
CREATE INDEX IF NOT EXISTS idx_operator_templates_status ON operator_templates(status);

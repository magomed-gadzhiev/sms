-- migrations/000084_phase4_platform_routes.up.sql
-- Phase 4: New operator-based routing table

CREATE TABLE IF NOT EXISTS platform_routes (
  id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  operator_id     UUID        REFERENCES operators(id) ON DELETE SET NULL,
  -- NULL operator_id means "All Networks"
  channel_type    VARCHAR(50) NOT NULL DEFAULT 'sms',
  -- channel_type: 'sms', 'flash', 'viber', 'whatsapp', etc.
  provider_id     UUID        NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
  legal_entity_id UUID        REFERENCES legal_entities(id) ON DELETE SET NULL,
  priority        INT         NOT NULL DEFAULT 0,
  active          BOOLEAN     NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT chk_all_networks_requires_legal_entity
    CHECK (operator_id IS NOT NULL OR legal_entity_id IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_platform_routes_operator   ON platform_routes(operator_id);
CREATE INDEX IF NOT EXISTS idx_platform_routes_provider   ON platform_routes(provider_id);
CREATE INDEX IF NOT EXISTS idx_platform_routes_priority   ON platform_routes(priority);
CREATE INDEX IF NOT EXISTS idx_platform_routes_active     ON platform_routes(active);
CREATE INDEX IF NOT EXISTS idx_platform_routes_legal      ON platform_routes(legal_entity_id);

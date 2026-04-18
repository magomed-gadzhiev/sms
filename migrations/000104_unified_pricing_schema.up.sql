-- Unified Pricing Model — Phase 1 expand schema.
-- Spec: docs/superpowers/specs/2026-04-18-unified-pricing-model-design.md

BEGIN;

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE price_owner_type AS ENUM ('platform', 'aggregator', 'subaccount');
CREATE TYPE price_model_type AS ENUM ('fixed', 'tiered', 'prepaid_threshold');

CREATE TABLE price_rules (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type       price_owner_type NOT NULL,
    owner_id         UUID NULL,
    country          VARCHAR(2)  NULL REFERENCES countries(iso_code),
    operator         VARCHAR(50) NULL REFERENCES operators(code),
    sender_category  VARCHAR(16) NULL,
    traffic_type     VARCHAR(16) NULL,
    valid_from       TIMESTAMPTZ NOT NULL,
    valid_to         TIMESTAMPTZ NULL,
    price_model      price_model_type NOT NULL,
    price_value      NUMERIC(12,6) NULL,
    tiers_json       JSONB NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by       UUID NULL,

    CONSTRAINT owner_id_platform_rule CHECK (
        (owner_type = 'platform' AND owner_id IS NULL)
        OR (owner_type <> 'platform' AND owner_id IS NOT NULL)
    ),
    CONSTRAINT value_xor_tiers CHECK (
        (price_model = 'fixed' AND price_value IS NOT NULL AND tiers_json IS NULL)
        OR (price_model IN ('tiered','prepaid_threshold') AND tiers_json IS NOT NULL AND price_value IS NULL)
    ),
    CONSTRAINT sender_category_valid CHECK (
        sender_category IS NULL OR sender_category IN ('paid','free','none')
    ),
    CONSTRAINT traffic_type_valid CHECK (
        traffic_type IS NULL OR traffic_type IN ('transactional','marketing','service')
    ),
    CONSTRAINT valid_to_after_from CHECK (valid_to IS NULL OR valid_to > valid_from)
);

-- Инвариант: на платформе ровно одна активная catch-all строка
CREATE UNIQUE INDEX platform_catchall_singleton ON price_rules (owner_type)
WHERE owner_type = 'platform'
  AND country IS NULL AND operator IS NULL
  AND sender_category IS NULL AND traffic_type IS NULL
  AND valid_to IS NULL;

-- Запрет перекрытия периодов для одного ключа (owner + dims)
ALTER TABLE price_rules ADD CONSTRAINT no_overlapping_periods
EXCLUDE USING gist (
    owner_type WITH =,
    COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid) WITH =,
    COALESCE(country, '') WITH =,
    COALESCE(operator, '') WITH =,
    COALESCE(sender_category, '') WITH =,
    COALESCE(traffic_type, '') WITH =,
    tstzrange(valid_from, valid_to, '[)') WITH &&
);

CREATE INDEX idx_price_rules_lookup ON price_rules
    (owner_type, owner_id, country, operator, sender_category, traffic_type, valid_from DESC);

-- Глобальный монотонный счётчик версий (инкремент при любом изменении price_rules)
CREATE TABLE price_rules_version (
    id         INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    version    BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO price_rules_version (id) VALUES (1);

-- Денормализованный горячий кеш
CREATE TABLE resolved_rules (
    subaccount_id    UUID NOT NULL,
    country          VARCHAR(2) NOT NULL,
    operator         VARCHAR(50) NOT NULL,
    sender_category  VARCHAR(16) NOT NULL,
    traffic_type     VARCHAR(16) NOT NULL,
    effective_date   DATE NOT NULL,
    price_model      price_model_type NOT NULL,
    price_value      NUMERIC(12,6) NULL,
    tiers_json       JSONB NULL,
    source_rule_id   UUID NOT NULL REFERENCES price_rules(id),
    source_level     price_owner_type NOT NULL,
    aggregator_id    UUID NOT NULL,
    resolved_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    rules_version    BIGINT NOT NULL,

    PRIMARY KEY (subaccount_id, country, operator, sender_category, traffic_type, effective_date)
);

CREATE INDEX idx_resolved_rules_by_aggregator ON resolved_rules (aggregator_id);
CREATE INDEX idx_resolved_rules_by_source ON resolved_rules (source_rule_id);

-- Единый per-subaccount-per-period счётчик потребления
CREATE TABLE subaccount_usage_counters (
    subaccount_id    UUID NOT NULL,
    period_key       TEXT NOT NULL,
    segments_used    BIGINT NOT NULL DEFAULT 0,
    amount_charged   NUMERIC(14,6) NOT NULL DEFAULT 0,
    last_updated     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (subaccount_id, period_key)
);

-- Outbox для ленивой инвалидации resolved_rules
CREATE TABLE resolved_rules_invalidation_outbox (
    id             BIGSERIAL PRIMARY KEY,
    rule_id        UUID NOT NULL,
    owner_type     price_owner_type NOT NULL,
    owner_id       UUID NULL,
    affected_dims  JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at   TIMESTAMPTZ NULL
);

CREATE INDEX idx_outbox_unprocessed ON resolved_rules_invalidation_outbox (created_at)
WHERE processed_at IS NULL;

-- Триггер: инкремент версии + публикация в outbox при изменении price_rules
CREATE OR REPLACE FUNCTION price_rules_after_change() RETURNS TRIGGER AS $$
DECLARE
    target_rule_id    UUID;
    target_owner_type price_owner_type;
    target_owner_id   UUID;
    target_dims       JSONB;
BEGIN
    IF TG_OP = 'DELETE' THEN
        target_rule_id    := OLD.id;
        target_owner_type := OLD.owner_type;
        target_owner_id   := OLD.owner_id;
        target_dims := jsonb_build_object(
            'country', OLD.country, 'operator', OLD.operator,
            'sender_category', OLD.sender_category, 'traffic_type', OLD.traffic_type
        );
    ELSE
        target_rule_id    := NEW.id;
        target_owner_type := NEW.owner_type;
        target_owner_id   := NEW.owner_id;
        target_dims := jsonb_build_object(
            'country', NEW.country, 'operator', NEW.operator,
            'sender_category', NEW.sender_category, 'traffic_type', NEW.traffic_type
        );
    END IF;

    UPDATE price_rules_version SET version = version + 1, updated_at = now() WHERE id = 1;

    INSERT INTO resolved_rules_invalidation_outbox (rule_id, owner_type, owner_id, affected_dims)
    VALUES (target_rule_id, target_owner_type, target_owner_id, target_dims);

    IF TG_OP = 'DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_price_rules_after_change
AFTER INSERT OR UPDATE OR DELETE ON price_rules
FOR EACH ROW EXECUTE FUNCTION price_rules_after_change();

COMMIT;

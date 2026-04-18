BEGIN;

DROP TRIGGER IF EXISTS trg_price_rules_after_change ON price_rules;
DROP FUNCTION IF EXISTS price_rules_after_change();

DROP TABLE IF EXISTS resolved_rules_invalidation_outbox;
DROP TABLE IF EXISTS subaccount_usage_counters;
DROP TABLE IF EXISTS resolved_rules;
DROP TABLE IF EXISTS price_rules_version;
DROP TABLE IF EXISTS price_rules;

DROP TYPE IF EXISTS price_model_type;
DROP TYPE IF EXISTS price_owner_type;

COMMIT;

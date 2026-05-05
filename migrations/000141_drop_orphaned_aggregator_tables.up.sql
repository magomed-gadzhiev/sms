-- Drop orphaned aggregator tables left over from migration slots 000095/000096
-- (commits 478e04c, 3cf1b85). Those slots were later reused for other features
-- (operator_templates_moderation, aggregator_tariffs), so the original aggregator_*
-- table-creation migrations no longer exist in the repo.
--
-- Sandbox DB still has them from when the original migrations were applied; fresh
-- dev DBs via `migrate up` would NOT create them. Plan 3 Task 10 (DB schema gap
-- audit) confirmed: zero consumers in internal/, cmd/, api/, portal-frontend/.
--
-- Aggregator architecture was pivoted to `is_reseller` flag on clients table
-- (project memory: aggregator_decisions 2026-04-20). These tables are dead.
-- Single stub row in aggregator_profiles is sandbox test data, not prod.
--
-- CASCADE drops associated indexes, partition children, and FKs.
DROP TABLE IF EXISTS aggregator_audit_log CASCADE;
DROP TABLE IF EXISTS aggregator_profiles CASCADE;

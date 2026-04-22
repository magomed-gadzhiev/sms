BEGIN;

-- Seed the aggregator_moderator role.
-- This role is held by users who are employed by an aggregator (reseller) client
-- and are responsible for moderating sender-name registrations and operator-template
-- bindings for the aggregator's own sub-accounts.
-- Enforcement of the reseller_id scope is done in the ModerationScope middleware
-- (Phase 1, Task 13) — not here.
INSERT INTO roles (id, name, description)
VALUES (
    '00000000-0000-0000-0000-000000000004',
    'aggregator_moderator',
    'Aggregator moderator — moderates sender names and operator bindings for sub-accounts where reseller_id = current_user.client_id'
)
ON CONFLICT (name) DO NOTHING;

COMMIT;

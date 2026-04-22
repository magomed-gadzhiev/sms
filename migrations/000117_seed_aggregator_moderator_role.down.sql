BEGIN;

-- Remove the aggregator_moderator role.
-- role_permissions rows are deleted automatically via ON DELETE CASCADE on role_id FK.
-- Users assigned this role cannot be deleted (role_id FK on users is ON DELETE RESTRICT),
-- so this migration will fail if any users hold the role — intentional safety guard.
DELETE FROM roles WHERE name = 'aggregator_moderator';

COMMIT;

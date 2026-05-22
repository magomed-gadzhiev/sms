-- Откат Plan 2 Task 7: client_routes.owner_type / owner_id.
DROP INDEX IF EXISTS ix_routes_owner_routetype;

ALTER TABLE client_routes
    DROP CONSTRAINT IF EXISTS chk_owner_id;

-- CASCADE на случай посторонних индексов/constraint'ов, ссылающихся на колонки
-- (например, sandbox-only uq_cell_provider, не создававшийся миграциями).
ALTER TABLE client_routes
    DROP COLUMN IF EXISTS owner_id   CASCADE,
    DROP COLUMN IF EXISTS owner_type CASCADE;

DROP TYPE IF EXISTS route_owner_type;

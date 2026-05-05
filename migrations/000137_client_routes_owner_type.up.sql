-- Plan 2 Task 7 fix: добавляем owner_type/owner_id в client_routes для разделения
-- platform / client / subaccount маршрутов. Sandbox DB уже содержит эти объекты
-- (созданы вне миграционного контроля), поэтому всё идемпотентно.
--
-- owner_type = 'platform' → owner_id IS NULL (общий fallback)
-- owner_type = 'client'   → owner_id = client_id (клиент верхнего уровня)
-- owner_type = 'subaccount' → owner_id = client_id (sub-account под reseller'ом)

-- 1) Enum route_owner_type.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'route_owner_type') THEN
        CREATE TYPE route_owner_type AS ENUM ('platform', 'client', 'subaccount');
    END IF;
END$$;

-- 2) Колонки (NULLABLE для backfill).
ALTER TABLE client_routes
    ADD COLUMN IF NOT EXISTS owner_type route_owner_type,
    ADD COLUMN IF NOT EXISTS owner_id   uuid;

-- 3) Backfill (idempotent: WHERE owner_type IS NULL).
-- platform: client_id IS NULL.
UPDATE client_routes
   SET owner_type = 'platform', owner_id = NULL
 WHERE owner_type IS NULL
   AND client_id IS NULL;

-- subaccount: client_id указывает на клиента с parent_client_id IS NOT NULL.
UPDATE client_routes cr
   SET owner_type = 'subaccount', owner_id = cr.client_id
  FROM clients c
 WHERE cr.owner_type IS NULL
   AND cr.client_id = c.id
   AND c.parent_client_id IS NOT NULL;

-- client: остальные с client_id IS NOT NULL.
UPDATE client_routes
   SET owner_type = 'client', owner_id = client_id
 WHERE owner_type IS NULL
   AND client_id IS NOT NULL;

-- 4) NOT NULL.
ALTER TABLE client_routes
    ALTER COLUMN owner_type SET NOT NULL;

-- 5) chk_owner_id constraint.
ALTER TABLE client_routes
    DROP CONSTRAINT IF EXISTS chk_owner_id;
ALTER TABLE client_routes
    ADD CONSTRAINT chk_owner_id CHECK (
        (owner_type = 'platform' AND owner_id IS NULL)
        OR (owner_type <> 'platform' AND owner_id IS NOT NULL)
    );

-- 6) Index.
CREATE INDEX IF NOT EXISTS ix_routes_owner_routetype
    ON client_routes (owner_type, owner_id, route_type);

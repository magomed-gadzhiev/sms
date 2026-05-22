-- Migration 000089: Fix messages.route_id foreign key
-- The existing FK references routes(id) (legacy table).
-- Routes are now managed in client_routes table.
-- Drop the old FK from the parent and all partitions.
-- route_id is an observability field; no referential integrity needed.

ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m01 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m02 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m03 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m04 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m05 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m06 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m07 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m08 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m09 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m10 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m11 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2024m12 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m01 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m02 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m03 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m04 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m05 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m06 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m07 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m08 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m09 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m10 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m11 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2025m12 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m01 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m02 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m03 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m04 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m05 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m06 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m07 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m08 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m09 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m10 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m11 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;
ALTER TABLE messages_y2026m12 DROP CONSTRAINT IF EXISTS messages_route_id_fkey;

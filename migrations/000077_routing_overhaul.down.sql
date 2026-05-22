-- migrations/000077_routing_overhaul.down.sql
ALTER TABLE templates DROP COLUMN IF EXISTS traffic_type;
DROP TABLE IF EXISTS route_schedules;
DROP TABLE IF EXISTS route_conditions;
DROP TABLE IF EXISTS route_condition_groups;
ALTER TABLE client_routes
  DROP COLUMN IF EXISTS name,
  DROP COLUMN IF EXISTS comment,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS share,
  DROP COLUMN IF EXISTS route_type;
ALTER TABLE client_routes ALTER COLUMN client_id SET NOT NULL;

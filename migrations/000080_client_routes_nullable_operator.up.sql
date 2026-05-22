-- Allow operator_id to be NULL for condition-only routes (e.g. traffic_type-based routing)
ALTER TABLE client_routes ALTER COLUMN operator_id DROP NOT NULL;

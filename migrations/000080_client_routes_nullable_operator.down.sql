-- Revert: set operator_id NOT NULL (will fail if any NULL values exist)
UPDATE client_routes SET operator_id = 'd0000000-0000-0000-0000-000000000001' WHERE operator_id IS NULL;
ALTER TABLE client_routes ALTER COLUMN operator_id SET NOT NULL;

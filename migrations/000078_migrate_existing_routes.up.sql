-- migrations/000078_migrate_existing_routes.up.sql

-- Set name for existing routes based on operator
UPDATE client_routes SET
  name = 'Route ' || id::text,
  status = CASE WHEN active THEN 'active' ELSE 'draft' END,
  share = weight,
  route_type = 'sms'
WHERE name IS NULL;

-- Create IF condition group with operator condition for each existing route
INSERT INTO route_condition_groups (route_id, group_index, logic_op)
SELECT id, 0, 'IF' FROM client_routes WHERE operator_id IS NOT NULL;

-- Create operator condition for each group
INSERT INTO route_conditions (group_id, condition_type, condition_value)
SELECT rcg.id, 'operator', cr.operator_id::text
FROM route_condition_groups rcg
JOIN client_routes cr ON cr.id = rcg.route_id
WHERE rcg.logic_op = 'IF'
  AND NOT EXISTS (SELECT 1 FROM route_conditions rc WHERE rc.group_id = rcg.id);

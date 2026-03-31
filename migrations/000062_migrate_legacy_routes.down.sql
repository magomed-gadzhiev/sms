-- Удаляем только платформенные дефолты (client_id IS NULL), добавленные этой миграцией
DELETE FROM client_routes WHERE client_id IS NULL;
DROP FUNCTION IF EXISTS resolve_operator_from_prefix(TEXT);

-- Вспомогательная функция: находит operator_id по паттерну маршрута
CREATE OR REPLACE FUNCTION resolve_operator_from_prefix(pattern TEXT)
RETURNS UUID AS $$
DECLARE
    result UUID;
BEGIN
    SELECT operator_id INTO result
    FROM operator_prefixes
    WHERE active = true
      AND pattern LIKE (prefix || '%')
    ORDER BY LENGTH(prefix) DESC
    LIMIT 1;
    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Переносим основные маршруты
INSERT INTO client_routes (client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at)
SELECT
    NULL,
    resolve_operator_from_prefix(r.pattern),
    r.provider_id,
    r.priority,
    1,
    r.active,
    r.created_at,
    r.updated_at
FROM routes r
WHERE resolve_operator_from_prefix(r.pattern) IS NOT NULL
ON CONFLICT (client_id, operator_id, provider_id)
    WHERE client_id IS NULL
    DO NOTHING;

-- Переносим failover-провайдеры как маршруты с меньшим приоритетом
INSERT INTO client_routes (client_id, operator_id, provider_id, priority, weight, active, created_at, updated_at)
SELECT
    NULL,
    resolve_operator_from_prefix(r.pattern),
    r.failover_provider_id,
    r.priority - 1,
    1,
    r.active,
    r.created_at,
    r.updated_at
FROM routes r
WHERE r.failover_provider_id IS NOT NULL
  AND resolve_operator_from_prefix(r.pattern) IS NOT NULL
ON CONFLICT (client_id, operator_id, provider_id)
    WHERE client_id IS NULL
    DO NOTHING;

-- При деактивации провайдера автоматически деактивируются связанные маршруты
CREATE OR REPLACE FUNCTION cascade_deactivate_routes()
RETURNS TRIGGER AS $$
BEGIN
    -- Только при изменении active с true на false
    IF OLD.active = true AND NEW.active = false THEN
        UPDATE routes
        SET active = false, updated_at = NOW()
        WHERE provider_id = NEW.id AND active = true;

        -- Также убираем провайдера из failover
        UPDATE routes
        SET failover_provider_id = NULL, updated_at = NOW()
        WHERE failover_provider_id = NEW.id;

        -- Деактивируем client_routes
        UPDATE client_routes
        SET active = false
        WHERE provider_id = NEW.id AND active = true;

        RAISE NOTICE 'Cascade deactivated routes for provider %', NEW.name;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_cascade_deactivate_routes ON providers;
CREATE TRIGGER trg_cascade_deactivate_routes
    BEFORE UPDATE ON providers
    FOR EACH ROW
    WHEN (OLD.active IS DISTINCT FROM NEW.active)
    EXECUTE FUNCTION cascade_deactivate_routes();

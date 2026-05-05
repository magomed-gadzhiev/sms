-- Trigger AFTER UPDATE: автоматически DELETE'ит SRA-row, если оба set_id стали NULL.
-- Контекст: ON DELETE SET NULL на FK provider_set_id/route_set_id оставляет
-- (NULL, NULL) row, который засоряет List/Overview. Plan 3 Task 6.
CREATE OR REPLACE FUNCTION subaccount_routing_assignment_orphan_cleanup() RETURNS trigger AS $$
BEGIN
    IF NEW.provider_set_id IS NULL AND NEW.route_set_id IS NULL THEN
        DELETE FROM subaccount_routing_assignment WHERE client_id = NEW.client_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sra_orphan_cleanup ON subaccount_routing_assignment;
CREATE TRIGGER trg_sra_orphan_cleanup
AFTER UPDATE ON subaccount_routing_assignment
FOR EACH ROW EXECUTE FUNCTION subaccount_routing_assignment_orphan_cleanup();

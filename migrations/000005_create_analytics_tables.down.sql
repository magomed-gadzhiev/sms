-- Удаление триггеров
DROP TRIGGER IF EXISTS update_aggregated_metrics_updated_at ON aggregated_metrics;

-- Удаление таблиц
DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS aggregated_metrics;
DROP TABLE IF EXISTS message_stats;

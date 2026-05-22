-- Функция для автоматического создания партиций lookup_log на следующий месяц
-- и удаления партиций старше 90 дней (PII retention policy)

CREATE OR REPLACE FUNCTION manage_lookup_log_partitions()
RETURNS void AS $$
DECLARE
    next_month_start DATE;
    next_month_end DATE;
    partition_name TEXT;
    old_partition_name TEXT;
    old_date DATE;
BEGIN
    -- Создание партиции на следующий месяц
    next_month_start := date_trunc('month', now() + interval '1 month')::date;
    next_month_end := (next_month_start + interval '1 month')::date;
    partition_name := 'lookup_log_' || to_char(next_month_start, 'YYYY_MM');

    IF NOT EXISTS (
        SELECT 1 FROM pg_class WHERE relname = partition_name
    ) THEN
        EXECUTE format(
            'CREATE TABLE %I PARTITION OF lookup_log FOR VALUES FROM (%L) TO (%L)',
            partition_name, next_month_start, next_month_end
        );
        RAISE NOTICE 'Создана партиция: %', partition_name;
    END IF;

    -- Удаление партиций старше 90 дней
    old_date := (now() - interval '90 days')::date;
    FOR old_partition_name IN
        SELECT tablename FROM pg_tables
        WHERE tablename LIKE 'lookup_log_%'
        AND schemaname = 'public'
    LOOP
        -- Извлекаем дату из имени партиции (формат: lookup_log_YYYY_MM)
        BEGIN
            IF to_date(replace(replace(old_partition_name, 'lookup_log_', ''), '_', '-'), 'YYYY-MM') + interval '1 month' < old_date THEN
                EXECUTE format('DROP TABLE IF EXISTS %I', old_partition_name);
                RAISE NOTICE 'Удалена партиция: %', old_partition_name;
            END IF;
        EXCEPTION WHEN OTHERS THEN
            -- Пропускаем партиции с неожиданным форматом имени
            NULL;
        END;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

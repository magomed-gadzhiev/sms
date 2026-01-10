-- Скрипт для автоматического создания полей Directus на основе структуры таблиц PostgreSQL

DO $$
DECLARE
    tbl_record RECORD;
    col_record RECORD;
    field_id UUID;
    field_type TEXT;
    field_meta JSONB;
BEGIN
    -- Проходим по всем коллекциям (кроме системных и партиций)
    FOR tbl_record IN 
        SELECT collection 
        FROM directus_collections 
        WHERE collection NOT LIKE 'directus%' 
        AND collection NOT LIKE '%_y20%'
    LOOP
        RAISE NOTICE 'Обработка коллекции: %', tbl_record.collection;
        
        -- Проходим по всем колонкам таблицы
        FOR col_record IN
            SELECT 
                column_name,
                data_type,
                udt_name,
                is_nullable,
                column_default,
                character_maximum_length,
                numeric_precision,
                numeric_scale
            FROM information_schema.columns
            WHERE table_schema = 'public'
            AND table_name = tbl_record.collection
            AND column_name NOT IN ('id') -- id добавляется автоматически
        LOOP
            -- Определяем тип поля Directus на основе типа PostgreSQL
            field_type := CASE col_record.udt_name
                WHEN 'uuid' THEN 'uuid'
                WHEN 'text' THEN 'text'
                WHEN 'varchar' THEN 'string'
                WHEN 'integer' THEN 'integer'
                WHEN 'bigint' THEN 'bigInteger'
                WHEN 'numeric' THEN 'decimal'
                WHEN 'boolean' THEN 'boolean'
                WHEN 'timestamp' THEN 'timestamp'
                WHEN 'timestamptz' THEN 'timestamp'
                WHEN 'jsonb' THEN 'json'
                WHEN '_text' THEN 'csv'
                ELSE 'string'
            END;
            
            -- Проверяем, существует ли уже поле
            SELECT id INTO field_id
            FROM directus_fields
            WHERE collection = tbl_record.collection
            AND field = col_record.column_name;
            
            IF field_id IS NULL THEN
                -- Создаем поле
                INSERT INTO directus_fields (
                    collection,
                    field,
                    type,
                    meta,
                    schema
                ) VALUES (
                    tbl_record.collection,
                    col_record.column_name,
                    field_type,
                    '{}'::jsonb,
                    jsonb_build_object(
                        'name', col_record.column_name,
                        'table', tbl_record.collection,
                        'data_type', col_record.data_type,
                        'is_nullable', col_record.is_nullable = 'YES',
                        'default_value', CASE WHEN col_record.column_default IS NOT NULL THEN col_record.column_default ELSE NULL END
                    )
                );
                
                RAISE NOTICE '  Создано поле: %.% (тип: %)', tbl_record.collection, col_record.column_name, field_type;
            ELSE
                RAISE NOTICE '  Поле уже существует: %.%', tbl_record.collection, col_record.column_name;
            END IF;
        END LOOP;
    END LOOP;
    
    RAISE NOTICE 'Завершено создание полей.';
END $$;

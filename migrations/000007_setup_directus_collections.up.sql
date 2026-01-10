-- Миграция для настройки коллекций Directus
--
-- Примечание: Directus автоматически обнаруживает существующие таблицы PostgreSQL
-- и создает коллекции при первом запуске. Эта миграция настраивает метаданные
-- коллекций через SQL, напрямую работая с таблицами Directus.
--
-- ВАЖНО: Эта миграция должна выполняться ПОСЛЕ того, как Directus уже запущен
-- и автоматически создал базовые коллекции. Обычно это делается после первого
-- запуска Directus и создания администратора.
--
-- Альтернативный способ настройки (рекомендуется):
--   Linux/Mac: ./scripts/setup-directus.sh
--   Windows:   .\scripts\setup-directus.ps1
--   Docker:    docker-compose exec dev node scripts/setup-directus.js
--
-- Настраиваемые коллекции:
--   - users: скрыт password_hash (hidden + readonly)
--   - roles: базовая настройка
--   - clients: скрыты secret и api_key (hidden + readonly)
--   - providers: скрыт password (hidden + readonly), readonly system_id
--   - routes: настроить relationships с providers
--   - accounts: настроить relationship с clients
--   - transactions: настроить relationships с clients и messages
--   - pricing_rules: настроить relationship с clients (опциональный)
--   - api_keys: скрыт key_hash (hidden + readonly)
--   - refresh_tokens: скрыт token_hash (hidden + readonly)

-- Проверяем, существуют ли таблицы Directus
DO $$
DECLARE
    directus_exists BOOLEAN;
BEGIN
    -- Проверяем существование таблицы directus_collections
    SELECT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name = 'directus_collections'
    ) INTO directus_exists;
    
    IF NOT directus_exists THEN
        RAISE NOTICE 'Таблицы Directus не найдены. Миграция пропущена. Запустите Directus сначала.';
        RETURN;
    END IF;
    
    RAISE NOTICE 'Начало настройки коллекций Directus...';
END $$;

-- Функция для обновления метаданных поля
CREATE OR REPLACE FUNCTION setup_directus_field(
    p_collection VARCHAR(255),
    p_field VARCHAR(255),
    p_meta JSONB DEFAULT NULL,
    p_interface VARCHAR(255) DEFAULT NULL,
    p_readonly BOOLEAN DEFAULT NULL,
    p_hidden BOOLEAN DEFAULT NULL
) RETURNS VOID AS $$
DECLARE
    v_field_id UUID;
    v_meta JSONB;
BEGIN
    -- Получаем ID поля
    SELECT id INTO v_field_id
    FROM directus_fields
    WHERE collection = p_collection AND field = p_field;
    
    IF v_field_id IS NULL THEN
        RAISE NOTICE 'Поле %.% не найдено, пропускаем', p_collection, p_field;
        RETURN;
    END IF;
    
    -- Получаем текущие метаданные
    SELECT COALESCE(meta, '{}'::jsonb) INTO v_meta
    FROM directus_fields
    WHERE id = v_field_id;
    
    -- Объединяем метаданные
    IF p_meta IS NOT NULL THEN
        v_meta := v_meta || p_meta;
    END IF;
    
    IF p_interface IS NOT NULL THEN
        v_meta := v_meta || jsonb_build_object('interface', p_interface);
    END IF;
    
    IF p_readonly IS NOT NULL THEN
        v_meta := v_meta || jsonb_build_object('readonly', p_readonly);
    END IF;
    
    IF p_hidden IS NOT NULL THEN
        v_meta := v_meta || jsonb_build_object('hidden', p_hidden);
    END IF;
    
    -- Обновляем метаданные
    UPDATE directus_fields
    SET meta = v_meta
    WHERE id = v_field_id;
    
    RAISE NOTICE 'Обновлено поле %.%', p_collection, p_field;
END;
$$ LANGUAGE plpgsql;

-- Функция для настройки relationship (Many-to-One)
CREATE OR REPLACE FUNCTION setup_directus_relation(
    p_collection VARCHAR(255),
    p_field VARCHAR(255),
    p_related_collection VARCHAR(255),
    p_on_delete VARCHAR(50) DEFAULT 'SET NULL'
) RETURNS VOID AS $$
DECLARE
    v_relation_id UUID;
BEGIN
    -- Проверяем, существует ли уже relationship
    SELECT id INTO v_relation_id
    FROM directus_relations
    WHERE many_collection = p_collection 
    AND many_field = p_field
    AND one_collection = p_related_collection;
    
    IF v_relation_id IS NOT NULL THEN
        RAISE NOTICE 'Relationship %.% -> % уже существует', p_collection, p_field, p_related_collection;
        RETURN;
    END IF;
    
    -- Создаем relationship
    INSERT INTO directus_relations (
        many_collection,
        many_field,
        one_collection,
        one_field,
        one_collection_field,
        one_allowed_collections,
        one_deselect_action
    ) VALUES (
        p_collection,
        p_field,
        p_related_collection,
        NULL,
        NULL,
        NULL,
        p_on_delete
    );
    
    RAISE NOTICE 'Создан relationship %.% -> %', p_collection, p_field, p_related_collection;
END;
$$ LANGUAGE plpgsql;

-- Настройка коллекции users
DO $$
BEGIN
    -- Скрыть password_hash полностью
    PERFORM setup_directus_field(
        'users',
        'password_hash',
        '{"options": {"masked": true, "iconRight": "lock"}}'::jsonb,
        'input-hash',
        true,  -- readonly
        true   -- hidden
    );
    
    -- Настроить role_id как relationship с roles (если еще не настроено)
    PERFORM setup_directus_relation('users', 'role_id', 'roles', 'RESTRICT');
    
    -- Настроить метаданные role_id
    PERFORM setup_directus_field(
        'users',
        'role_id',
        '{"options": {"template": "{{name}}"}}'::jsonb,
        'select-dropdown-m2o'
    );
END $$;

-- Настройка коллекции roles
DO $$
BEGIN
    PERFORM setup_directus_field('roles', 'name', '{"width": "half", "required": true}'::jsonb, 'input');
    PERFORM setup_directus_field('roles', 'description', '{"width": "half"}'::jsonb, 'input-multiline');
END $$;

-- Настройка коллекции clients
DO $$
BEGIN
    -- Скрыть secret
    PERFORM setup_directus_field(
        'clients',
        'secret',
        '{"options": {"masked": true, "iconRight": "lock"}}'::jsonb,
        'input',
        true,  -- readonly
        true   -- hidden
    );
    
    -- Скрыть api_key
    PERFORM setup_directus_field(
        'clients',
        'api_key',
        '{"options": {"font": "monospace", "iconRight": "vpn_key"}}'::jsonb,
        'input',
        true,  -- readonly
        true   -- hidden
    );
    
    -- Настроить allowed_source_addresses как теги
    PERFORM setup_directus_field(
        'clients',
        'allowed_source_addresses',
        '{"width": "full"}'::jsonb,
        'tags'
    );
    
    -- Настроить metadata как JSON (если поле существует)
    IF EXISTS (
        SELECT FROM directus_fields WHERE collection = 'clients' AND field = 'metadata'
    ) THEN
        PERFORM setup_directus_field(
            'clients',
            'metadata',
            '{"width": "full"}'::jsonb,
            'input-code'
        );
    END IF;
END $$;

-- Настройка коллекции providers
DO $$
BEGIN
    -- Скрыть password
    PERFORM setup_directus_field(
        'providers',
        'password',
        '{"options": {"masked": true, "iconRight": "lock"}}'::jsonb,
        'input',
        true,  -- readonly
        true   -- hidden
    );
    
    -- system_id только для чтения (не скрыт, но нельзя редактировать)
    PERFORM setup_directus_field(
        'providers',
        'system_id',
        '{"width": "half"}'::jsonb,
        'input',
        true   -- readonly
    );
    
    -- Настроить bind_type как dropdown
    PERFORM setup_directus_field(
        'providers',
        'bind_type',
        '{"options": {"choices": [{"text": "Transceiver", "value": "transceiver"}, {"text": "Transmitter", "value": "transmitter"}, {"text": "Receiver", "value": "receiver"}]}}'::jsonb,
        'select-dropdown',
        false,
        false
    );
END $$;

-- Настройка коллекции routes
DO $$
BEGIN
    -- Настроить provider_id как relationship с providers
    PERFORM setup_directus_relation('routes', 'provider_id', 'providers', 'RESTRICT');
    
    PERFORM setup_directus_field(
        'routes',
        'provider_id',
        '{"width": "half", "required": true, "options": {"template": "{{name}}"}}'::jsonb,
        'select-dropdown-m2o'
    );
    
    -- Настроить failover_provider_id как relationship с providers
    PERFORM setup_directus_relation('routes', 'failover_provider_id', 'providers', 'SET NULL');
    
    PERFORM setup_directus_field(
        'routes',
        'failover_provider_id',
        '{"width": "half", "options": {"template": "{{name}}", "allowNone": true}}'::jsonb,
        'select-dropdown-m2o'
    );
    
    -- Настроить pattern_type как dropdown
    PERFORM setup_directus_field(
        'routes',
        'pattern_type',
        '{"options": {"choices": [{"text": "Prefix", "value": "prefix"}, {"text": "Regex", "value": "regex"}, {"text": "Exact", "value": "exact"}]}}'::jsonb,
        'select-dropdown'
    );
END $$;

-- Настройка коллекции accounts
DO $$
BEGIN
    -- Настроить client_id как relationship с clients
    PERFORM setup_directus_relation('accounts', 'client_id', 'clients', 'CASCADE');
    
    PERFORM setup_directus_field(
        'accounts',
        'client_id',
        '{"width": "half", "required": true, "options": {"template": "{{name}}"}}'::jsonb,
        'select-dropdown-m2o'
    );
    
    -- Настроить balance как число (readonly, изменяется через транзакции)
    PERFORM setup_directus_field(
        'accounts',
        'balance',
        '{"width": "half", "options": {"step": 0.000001, "iconRight": "attach_money"}}'::jsonb,
        'input',
        true  -- readonly
    );
    
    -- Настроить currency как dropdown
    PERFORM setup_directus_field(
        'accounts',
        'currency',
        '{"width": "half", "options": {"choices": [{"text": "USD", "value": "USD"}, {"text": "EUR", "value": "EUR"}, {"text": "RUB", "value": "RUB"}]}}'::jsonb,
        'select-dropdown'
    );
END $$;

-- Настройка коллекции transactions
DO $$
BEGIN
    -- Настроить client_id как relationship с clients
    PERFORM setup_directus_relation('transactions', 'client_id', 'clients', 'CASCADE');
    
    PERFORM setup_directus_field(
        'transactions',
        'client_id',
        '{"width": "half", "required": true, "options": {"template": "{{name}}"}}'::jsonb,
        'select-dropdown-m2o'
    );
    
    -- Настроить message_id как relationship с messages (если возможно)
    -- Примечание: messages - партиционированная таблица, relationship может не работать
    -- В этом случае поле останется как UUID
    
    -- Настроить type как dropdown
    PERFORM setup_directus_field(
        'transactions',
        'type',
        '{"options": {"choices": [{"text": "Charge", "value": "charge"}, {"text": "Credit", "value": "credit"}, {"text": "Refund", "value": "refund"}, {"text": "Adjustment", "value": "adjustment"}]}}'::jsonb,
        'select-dropdown'
    );
    
    -- Настроить amount, balance_before, balance_after как числа
    PERFORM setup_directus_field(
        'transactions',
        'amount',
        '{"width": "half", "options": {"step": 0.000001, "iconRight": "attach_money"}}'::jsonb,
        'input'
    );
    
    PERFORM setup_directus_field(
        'transactions',
        'balance_before',
        '{"width": "half", "readonly": true, "options": {"step": 0.000001, "iconRight": "attach_money"}}'::jsonb,
        'input',
        true
    );
    
    PERFORM setup_directus_field(
        'transactions',
        'balance_after',
        '{"width": "half", "readonly": true, "options": {"step": 0.000001, "iconRight": "attach_money"}}'::jsonb,
        'input',
        true
    );
    
    -- Настроить metadata как JSON (если поле существует)
    IF EXISTS (
        SELECT FROM directus_fields WHERE collection = 'transactions' AND field = 'metadata'
    ) THEN
        PERFORM setup_directus_field(
            'transactions',
            'metadata',
            '{"width": "full"}'::jsonb,
            'input-code'
        );
    END IF;
END $$;

-- Настройка коллекции pricing_rules
DO $$
BEGIN
    -- Настроить client_id как relationship с clients (опциональный)
    PERFORM setup_directus_relation('pricing_rules', 'client_id', 'clients', 'CASCADE');
    
    PERFORM setup_directus_field(
        'pricing_rules',
        'client_id',
        '{"width": "half", "options": {"template": "{{name}}", "allowNone": true}}'::jsonb,
        'select-dropdown-m2o'
    );
    
    -- Настроить price_per_message как число
    PERFORM setup_directus_field(
        'pricing_rules',
        'price_per_message',
        '{"width": "half", "required": true, "options": {"step": 0.000001, "iconRight": "attach_money"}}'::jsonb,
        'input'
    );
    
    -- Настроить currency как dropdown
    PERFORM setup_directus_field(
        'pricing_rules',
        'currency',
        '{"width": "half", "options": {"choices": [{"text": "USD", "value": "USD"}, {"text": "EUR", "value": "EUR"}, {"text": "RUB", "value": "RUB"}]}}'::jsonb,
        'select-dropdown'
    );
    
    -- Настроить priority
    PERFORM setup_directus_field(
        'pricing_rules',
        'priority',
        '{"width": "half", "options": {"step": 1, "min": 0}}'::jsonb,
        'input'
    );
END $$;

-- Настройка коллекции api_keys
DO $$
BEGIN
    -- Скрыть key_hash
    PERFORM setup_directus_field(
        'api_keys',
        'key_hash',
        '{"options": {"masked": true, "iconRight": "lock"}}'::jsonb,
        'input',
        true,  -- readonly
        true   -- hidden
    );
    
    -- Настроить user_id как relationship с users
    PERFORM setup_directus_relation('api_keys', 'user_id', 'users', 'CASCADE');
    
    PERFORM setup_directus_field(
        'api_keys',
        'user_id',
        '{"width": "half", "required": true, "options": {"template": "{{username}} ({{email}})"}}'::jsonb,
        'select-dropdown-m2o'
    );
    
    -- Настроить key_prefix для отображения
    PERFORM setup_directus_field(
        'api_keys',
        'key_prefix',
        '{"width": "half", "readonly": true, "options": {"font": "monospace"}}'::jsonb,
        'input',
        true
    );
END $$;

-- Настройка коллекции refresh_tokens
DO $$
BEGIN
    -- Скрыть token_hash
    PERFORM setup_directus_field(
        'refresh_tokens',
        'token_hash',
        '{"options": {"masked": true, "iconRight": "lock"}}'::jsonb,
        'input',
        true,  -- readonly
        true   -- hidden
    );
    
    -- Настроить user_id как relationship с users
    PERFORM setup_directus_relation('refresh_tokens', 'user_id', 'users', 'CASCADE');
    
    PERFORM setup_directus_field(
        'refresh_tokens',
        'user_id',
        '{"width": "half", "required": true, "options": {"template": "{{username}}"}}'::jsonb,
        'select-dropdown-m2o'
    );
END $$;

-- Настройка коллекции client_configs (если существует)
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM directus_collections WHERE collection = 'client_configs'
    ) THEN
        -- Настроить client_id как relationship с clients
        PERFORM setup_directus_relation('client_configs', 'client_id', 'clients', 'CASCADE');
        
        PERFORM setup_directus_field(
            'client_configs',
            'client_id',
            '{"width": "half", "required": true, "options": {"template": "{{name}}"}}'::jsonb,
            'select-dropdown-m2o'
        );
        
        -- Настроить allowed_sources и blocked_destinations как теги
        PERFORM setup_directus_field(
            'client_configs',
            'allowed_sources',
            '{"width": "half"}'::jsonb,
            'tags'
        );
        
        PERFORM setup_directus_field(
            'client_configs',
            'blocked_destinations',
            '{"width": "half"}'::jsonb,
            'tags'
        );
        
        -- Настроить settings как JSON
        IF EXISTS (
            SELECT FROM directus_fields WHERE collection = 'client_configs' AND field = 'settings'
        ) THEN
            PERFORM setup_directus_field(
                'client_configs',
                'settings',
                '{"width": "full"}'::jsonb,
                'input-code'
            );
        END IF;
    END IF;
END $$;

-- Очистка временных функций (опционально, можно оставить для будущего использования)
-- DROP FUNCTION IF EXISTS setup_directus_field(VARCHAR, VARCHAR, JSONB, VARCHAR, BOOLEAN, BOOLEAN);
-- DROP FUNCTION IF EXISTS setup_directus_relation(VARCHAR, VARCHAR, VARCHAR, VARCHAR);

DO $$
BEGIN
    RAISE NOTICE 'Настройка коллекций Directus завершена.';
END $$;

-- Откат: возвращаем старую функцию (которая падает после миграции 000091
-- из-за accounts.company_id NOT NULL — но это ровно то состояние, которое
-- было до 000125).
--
-- ВНИМАНИЕ оператору: после этого down-перехода register-flow и любая
-- регистрация нового клиента через client-service сломаются на NOT NULL
-- constraint. Down-путь оставлен для `migrate down` cleanup перед накатом
-- исправленной версии, не для штатной эксплуатации.

DO $$ BEGIN
    RAISE NOTICE 'Down-migration 000125: восстановлен сломанный pre-091 триггер. Любые INSERT INTO clients теперь будут падать на accounts.company_id NOT NULL. Накатите 000125.up немедленно, либо patch вручную.';
END $$;

CREATE OR REPLACE FUNCTION public.create_account_for_new_client()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
    INSERT INTO accounts (id, client_id, balance, currency, created_at, updated_at)
    VALUES (uuid_generate_v4(), NEW.id, 0, 'RUB', NOW(), NOW())
    ON CONFLICT (client_id) DO NOTHING;
    RETURN NEW;
END;
$function$;

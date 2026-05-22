-- BUG-5 fix (этап 2/30 UX-аудита): триггер trg_create_account_for_new_client
-- (введён миграцией 000056) делает INSERT в accounts без company_id, но
-- миграция 000091 сделала accounts.company_id NOT NULL. Любая регистрация
-- нового клиента через client-service.CreateClient теперь падает на этом
-- триггере: SQLSTATE 23502 "null value in column company_id".
--
-- Self-service register flow по факту никогда не работал после 000091.
-- demo_seed.sql обходил это через ALTER TABLE clients DISABLE TRIGGER, но
-- production register этого не делает.
--
-- Narrow fix (вариант 3 / гибрид из эскалации): обновить триггер так, чтобы
-- он сам создавал «Оферта» компанию и привязывал её, аналогично backfill'у в
-- миграции 000091.
--
-- ВНИМАНИЕ для будущих авторов кода: пока этот триггер активен, БЕЗОПАСНОЕ
-- application-side создание company/client_companies для нового клиента
-- внутри той же транзакции чревато конфликтом с уникальным частичным
-- индексом idx_client_companies_default (000091:40-42, UNIQUE WHERE
-- is_default=TRUE) — триггер уже создал свою «Оферта»-компанию с
-- is_default=TRUE, и явный AttachCompany(..., is_default=TRUE) с другой
-- company_id упадёт. Перенос логики из триггера в client-service.CreateClient
-- (и удаление триггера) — обязательный follow-up; до этого не добавлять
-- параллельных путей создания client_companies в сервисах.

CREATE OR REPLACE FUNCTION public.create_account_for_new_client()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
DECLARE
    v_company_id UUID;
BEGIN
    -- Если у клиента уже есть дефолтная компания (например, был привязан
    -- через явную бизнес-логику до триггера) — переиспользуем её.
    SELECT cc.company_id INTO v_company_id
    FROM client_companies cc
    WHERE cc.client_id = NEW.id AND cc.is_default = TRUE
    LIMIT 1;

    -- Если нет — создаём «Оферта»-компанию (та же стратегия, что в backfill
    -- миграции 000091 для существующих клиентов на момент введения companies).
    IF v_company_id IS NULL THEN
        INSERT INTO companies (name, is_offer)
        VALUES ('Оферта', TRUE)
        RETURNING id INTO v_company_id;

        INSERT INTO client_companies (client_id, company_id, is_default)
        VALUES (NEW.id, v_company_id, TRUE)
        ON CONFLICT (client_id, company_id) DO NOTHING;
    END IF;

    -- AFTER INSERT-on-clients: NEW.id — только что вставленная строка,
    -- accounts.client_id UNIQUE никогда не может конфликтовать здесь по
    -- определению. Намеренно НЕ добавляем ON CONFLICT: молчаливое
    -- проглатывание здесь означало бы потерянный account при ранее
    -- созданной orphan «Оферта»-компании в этой же функции — лучше
    -- громкая ошибка, чем тихая порча invariants.
    INSERT INTO accounts (id, client_id, company_id, balance, currency, created_at, updated_at)
    VALUES (uuid_generate_v4(), NEW.id, v_company_id, 0, 'RUB', NOW(), NOW());

    RETURN NEW;
END;
$function$;

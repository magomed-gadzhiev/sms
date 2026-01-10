-- Откат миграции настройки коллекций Directus
--
-- ВАЖНО: Эта миграция удаляет только вспомогательные функции.
-- Метаданные коллекций Directus НЕ откатываются автоматически,
-- так как их удаление через SQL может привести к потере данных.
--
-- Для полного отката настроек коллекций используйте:
--   - UI Directus: Settings -> Data Model -> Collections
--   - API Directus: DELETE /fields/{collection}/{field}
--   - Или скрипт setup-directus.js с опцией отката

-- Удаляем вспомогательные функции
DROP FUNCTION IF EXISTS setup_directus_field(VARCHAR, VARCHAR, JSONB, VARCHAR, BOOLEAN, BOOLEAN);
DROP FUNCTION IF EXISTS setup_directus_relation(VARCHAR, VARCHAR, VARCHAR, VARCHAR);

-- Примечание: Метаданные полей и relationships останутся в таблицах Directus.
-- Если требуется полная очистка, выполните вручную через API или UI Directus:
--
-- Примеры SQL для удаления настроек (выполнять только при необходимости):
--
-- Удаление всех relationships для определенной коллекции:
-- DELETE FROM directus_relations WHERE many_collection = 'collection_name';
--
-- Сброс метаданных полей к значениям по умолчанию:
-- UPDATE directus_fields SET meta = '{}'::jsonb WHERE collection = 'collection_name';
--
-- ВАЖНО: Эти операции могут привести к потере настроек, сделанных вручную через UI!

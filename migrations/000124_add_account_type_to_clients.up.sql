-- BUG-4 fix (этап 2/30 UX-аудита): код в
-- internal/services/client/infrastructure/repository/client_repository.go
-- и
-- internal/services/tarification/infrastructure/repository/client_info_repository.go
-- ссылается на колонку clients.account_type, но миграция, добавляющая её,
-- никогда не была написана (введена в коммите be20324, 2026-04-15).
-- Из-за этого UI register flow падает 500 "failed to create client".
--
-- Колонка дублирует информацию из parent_client_id (NULL → direct, NOT NULL →
-- sub_account). Здесь добавляем колонку как narrow fix; рефакторинг кода на
-- derived parent_client_id IS [NOT] NULL — отдельный follow-up (вариант 2 из
-- эскалации, выбран гибрид).

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS account_type VARCHAR(20) NOT NULL DEFAULT 'direct'
        CHECK (account_type IN ('direct', 'sub_account'));

-- Backfill: существующие саб-клиенты должны получить 'sub_account'. Direct
-- остаются на DEFAULT. На 4 demo-клиентах — мгновенно; на проде с большим
-- объёмом partitioning'а нет, обычный UPDATE.
UPDATE clients
SET account_type = 'sub_account'
WHERE parent_client_id IS NOT NULL
  AND account_type = 'direct';

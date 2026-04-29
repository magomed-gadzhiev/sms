-- Откат: убираем колонку. Связанные строки кода
-- (client_repository.go, client_info_repository.go) перестанут работать —
-- это симметрично состоянию до 000124.up.sql.
ALTER TABLE clients DROP COLUMN IF EXISTS account_type;

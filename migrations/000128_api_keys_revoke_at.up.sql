-- Soft-rotate grace period: помечает ключ revoke_at-меткой, после которой он
-- считается невалидным (см. domain.APIKey.IsValid). Активный ключ ещё работает
-- до момента revoke_at, что даёт integration окно безопасной подмены.
--
-- NULL = ключ не в процессе ротации (default для всех существующих).
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS revoke_at TIMESTAMPTZ NULL;

-- Индекс для будущего housekeeping-job'а (Active=true AND revoke_at < NOW)
-- partial index — экономит место (только rotating ключи).
CREATE INDEX IF NOT EXISTS api_keys_revoke_at_idx
    ON api_keys (revoke_at)
    WHERE revoke_at IS NOT NULL AND active = true;

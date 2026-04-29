-- Reset subacc@test.local sandbox QA user password to Admin123! to align with
-- aggregator@test.local and admin@example.com (см. memory project_sandbox_test_credentials.md).
--
-- Прежний пароль (Test1234!) не проходил авторизацию в sandbox после серии
-- ручных правок; этот апсёрт делает sandbox-стейт детерминированным:
-- пароль = Admin123! (тот же bcrypt hash, что и в migration 000116).
--
-- Idempotent: при повторном применении просто перезатирает hash.
--
-- PRODUCTION GUARD: миграция skip'ается, если app.environment='production'.
-- См. 000116 — там настроен тот же guard.

DO $$
BEGIN
    IF current_setting('app.environment', true) = 'production' THEN
        RAISE NOTICE 'Skipping sandbox seed (000118) on production environment';
        RETURN;
    END IF;

    UPDATE users
    SET password_hash = '$2a$10$Od96EF1oN2Cp7JPpMNyLCuHs.mN3dRB82Ukq8TRhEDgKokaDj2jYO',
        active = true,
        updated_at = NOW()
    WHERE email = 'subacc@test.local';
END $$;

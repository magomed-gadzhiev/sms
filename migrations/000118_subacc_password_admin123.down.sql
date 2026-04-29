-- Down: вернуть subacc@test.local на исторический Test1234! hash из seed.sql §10.
-- Реальный rollback бессмысленен (sandbox), но миграционный фреймворк требует .down.

UPDATE users
SET password_hash = '$2a$10$5Eb9tCKwrrqXl5nWs.5l3O.Fl4ej19WeJAZ9.0sJ2b6frKQd2YQay',
    updated_at = NOW()
WHERE email = 'subacc@test.local';

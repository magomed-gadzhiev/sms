-- Rollback sandbox test users seed.
DELETE FROM users WHERE email = 'aggregator@test.local';
DELETE FROM clients WHERE id = 'a0000000-0000-0000-0000-000000000001';

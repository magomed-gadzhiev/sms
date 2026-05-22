ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_client_id_fkey;
ALTER TABLE sessions ADD CONSTRAINT sessions_client_id_fkey
    FOREIGN KEY (client_id) REFERENCES clients(id);
ALTER TABLE sessions ALTER COLUMN client_id SET NOT NULL;

-- Make client_id nullable in sessions (admin users don't have a client record)
ALTER TABLE sessions ALTER COLUMN client_id DROP NOT NULL;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_client_id_fkey;
ALTER TABLE sessions ADD CONSTRAINT sessions_client_id_fkey
    FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE SET NULL;

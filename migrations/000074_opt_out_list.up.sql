CREATE TABLE IF NOT EXISTS opt_out_list (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  client_id    UUID        NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
  phone        TEXT        NOT NULL,
  keyword      TEXT,
  opted_out_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (client_id, phone)
);

CREATE INDEX IF NOT EXISTS idx_opt_out_list_client_phone ON opt_out_list (client_id, phone);

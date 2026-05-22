-- Restores billing_mode column with default; original data is NOT recoverable.
ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS billing_mode VARCHAR(20) NOT NULL DEFAULT 'own'
        CHECK (billing_mode IN ('own', 'aggregator', 'hybrid'));

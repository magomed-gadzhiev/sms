-- Balance transfers between clients (reseller -> sub-account)
CREATE TABLE balance_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_client_id UUID NOT NULL REFERENCES clients(id),
    to_client_id UUID NOT NULL REFERENCES clients(id),
    amount NUMERIC(15, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    from_transaction_id UUID NOT NULL REFERENCES transactions(id),
    to_transaction_id UUID NOT NULL REFERENCES transactions(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_balance_transfers_from ON balance_transfers(from_client_id, created_at DESC);
CREATE INDEX idx_balance_transfers_to ON balance_transfers(to_client_id, created_at DESC);

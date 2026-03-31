CREATE TABLE sender_name_billing_records (
    id                     UUID          NOT NULL DEFAULT uuid_generate_v4(),
    sender_registration_id UUID          NOT NULL REFERENCES sender_registrations(id),
    client_id              UUID          NOT NULL REFERENCES clients(id),
    operator_id            UUID          NOT NULL REFERENCES operators(id),
    billing_month          DATE          NOT NULL,
    amount                 NUMERIC(20,6) NOT NULL CHECK (amount > 0),
    created_at             TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    PRIMARY KEY (id),
    UNIQUE (sender_registration_id, billing_month)
);

CREATE INDEX idx_snbr_client_id   ON sender_name_billing_records(client_id);
CREATE INDEX idx_snbr_operator_id ON sender_name_billing_records(operator_id);
CREATE INDEX idx_snbr_month       ON sender_name_billing_records(billing_month);
CREATE INDEX idx_snbr_reg_month   ON sender_name_billing_records(sender_registration_id, billing_month);

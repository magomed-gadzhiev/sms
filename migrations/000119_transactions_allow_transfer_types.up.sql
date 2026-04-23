-- Расширить transactions_type_check: добавить transfer_out/transfer_in.
-- Нужно для перевода средств между client и sub-account (initial_balance, manual transfer).
-- Раньше billing-service возвращал 500 SQLSTATE 23514 при CreateSubAccount с initial_balance > 0.

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_type_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_type_check
    CHECK (type::text = ANY (ARRAY[
        'charge'::varchar,
        'credit'::varchar,
        'refund'::varchar,
        'adjustment'::varchar,
        'transfer_out'::varchar,
        'transfer_in'::varchar
    ]::text[]));

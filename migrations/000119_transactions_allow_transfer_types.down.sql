ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_type_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_type_check
    CHECK (type::text = ANY (ARRAY[
        'charge'::varchar,
        'credit'::varchar,
        'refund'::varchar,
        'adjustment'::varchar
    ]::text[]));

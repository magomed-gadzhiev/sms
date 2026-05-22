ALTER TABLE messages
    DROP COLUMN IF EXISTS channel,
    DROP COLUMN IF EXISTS operator_id,
    DROP COLUMN IF EXISTS country_id,
    DROP COLUMN IF EXISTS send_method;

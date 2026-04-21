BEGIN;

-- commit_retry_queue — персистентный буфер сообщений, для которых CommitCharge
-- упал на transport/timeout уровне после успешного SUBMIT к провайдеру.
-- Worker в tarification-service периодически забирает batch и ретраит.
-- message_id — PK: двойной enqueue невозможен, гарантируется ON CONFLICT DO NOTHING.
CREATE TABLE commit_retry_queue (
    message_id       UUID PRIMARY KEY,
    client_id        UUID NOT NULL,
    operator_id      UUID NOT NULL,
    sender_name      VARCHAR(255) NOT NULL DEFAULT '',
    segment_count    INTEGER NOT NULL CHECK (segment_count > 0),
    idempotency_key  VARCHAR(255) NOT NULL,
    attempt_count    INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    last_error       TEXT,
    next_retry_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индекс для worker'а: ORDER BY next_retry_at ASC WHERE next_retry_at <= NOW().
CREATE INDEX idx_commit_retry_queue_next_retry ON commit_retry_queue(next_retry_at);

COMMIT;

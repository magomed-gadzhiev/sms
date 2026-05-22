CREATE TABLE notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    type        VARCHAR(64) NOT NULL,
    body        TEXT NOT NULL,
    object_type VARCHAR(64),
    object_id   VARCHAR(255),
    is_read     BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user_unread ON notifications (user_id, is_read)
    WHERE is_read = false;

CREATE INDEX idx_notifications_user_created ON notifications (user_id, created_at DESC);

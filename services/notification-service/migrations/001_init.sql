-- Notification Service — Initial Schema
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Notifications table — stores all notifications for users
CREATE TABLE IF NOT EXISTS notifications (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID           NOT NULL,
    type       VARCHAR(50)    NOT NULL,
    title      VARCHAR(255)   NOT NULL,
    message    TEXT           NOT NULL,
    is_read    BOOLEAN        NOT NULL DEFAULT FALSE,
    metadata   JSONB          DEFAULT '{}',
    event_id   VARCHAR(255)   UNIQUE,  -- idempotency key
    created_at TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Delivery tracking — one notification can have multiple delivery channels
CREATE TABLE IF NOT EXISTS notification_deliveries (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    notification_id UUID           NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    channel         VARCHAR(20)    NOT NULL,  -- websocket, email, push
    status          VARCHAR(20)    NOT NULL DEFAULT 'pending',
    retry_count     INT            NOT NULL DEFAULT 0,
    max_retries     INT            NOT NULL DEFAULT 3,
    last_attempt    TIMESTAMPTZ,
    next_retry      TIMESTAMPTZ,
    error_message   TEXT,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_notifications_user_id     ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read) WHERE is_read = FALSE;
CREATE INDEX IF NOT EXISTS idx_notifications_event_id    ON notifications(event_id);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at  ON notifications(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deliveries_status         ON notification_deliveries(status) WHERE status IN ('pending', 'failed', 'retrying');
CREATE INDEX IF NOT EXISTS idx_deliveries_notification   ON notification_deliveries(notification_id);

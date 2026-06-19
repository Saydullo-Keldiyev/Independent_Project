-- Migration 003: Enhance outbox_events table for production-grade Outbox pattern
-- Adds: idempotency_key, published_at, retry_count, last_error columns
-- Replaces the boolean 'processed' column with proper published_at timestamp tracking.
-- Requirements: 8.6

-- UP
ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255) UNIQUE,
    ADD COLUMN IF NOT EXISTS published_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS retry_count     INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error      TEXT;

-- Migrate existing data: mark processed=TRUE rows as published
UPDATE outbox_events SET published_at = created_at WHERE processed = TRUE AND published_at IS NULL;

-- Drop old index and create new one for unpublished events
DROP INDEX IF EXISTS idx_outbox_unprocessed;
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox_events(created_at)
    WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_outbox_idempotency ON outbox_events(idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- DOWN
-- DROP INDEX IF EXISTS idx_outbox_idempotency;
-- DROP INDEX IF EXISTS idx_outbox_unpublished;
-- ALTER TABLE outbox_events DROP COLUMN IF EXISTS last_error;
-- ALTER TABLE outbox_events DROP COLUMN IF EXISTS retry_count;
-- ALTER TABLE outbox_events DROP COLUMN IF EXISTS published_at;
-- ALTER TABLE outbox_events DROP COLUMN IF EXISTS idempotency_key;
-- CREATE INDEX IF NOT EXISTS idx_outbox_unprocessed ON outbox_events(created_at) WHERE processed = FALSE;

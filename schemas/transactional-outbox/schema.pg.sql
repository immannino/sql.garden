-- transactional-outbox.pg.sql for PostgreSQL

CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,                   -- Use BIGSERIAL for auto-incrementing 64-bit integer
    aggregate_type TEXT NOT NULL,               -- TEXT is suitable
    aggregate_id TEXT NOT NULL,                 -- TEXT is suitable
    event_type TEXT NOT NULL,                   -- TEXT is suitable
    payload JSONB NOT NULL,                     -- Use JSONB for efficient JSON storage and querying
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PUBLISHING', 'PUBLISHED', 'FAILED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Use TIMESTAMPTZ for timezone-aware timestamps, NOW() is standard
    last_attempt_at TIMESTAMPTZ,
    attempt_count INTEGER NOT NULL DEFAULT 0
);

-- Index for the publisher process to find pending events
CREATE INDEX idx_outbox_events_status_created_at ON outbox_events(status, created_at);

-- Optional: Add index if querying by aggregate often
-- CREATE INDEX idx_outbox_events_aggregate ON outbox_events(aggregate_type, aggregate_id);
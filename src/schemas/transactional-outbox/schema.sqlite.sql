-- transactional_outbox.sql
CREATE TABLE outbox_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    aggregate_type TEXT NOT NULL,           -- e.g., 'Order', 'User'
    aggregate_id TEXT NOT NULL,             -- ID of the entity that changed
    event_type TEXT NOT NULL,               -- e.g., 'OrderCreated', 'UserEmailUpdated'
    payload TEXT NOT NULL,                  -- Event data (e.g., JSON)
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PUBLISHING', 'PUBLISHED', 'FAILED')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_attempt_at TIMESTAMP,
    attempt_count INTEGER NOT NULL DEFAULT 0
);

-- Index for the publisher process to find pending events
CREATE INDEX idx_outbox_events_status ON outbox_events(status, created_at);

-- Usage Pattern:
-- 1. BEGIN TRANSACTION;
-- 2. Make changes to your main data tables (e.g., INSERT into orders).
-- 3. INSERT into outbox_events (aggregate_type, aggregate_id, event_type, payload) VALUES (...);
-- 4. COMMIT;
-- 5. A separate process/worker polls the outbox_events table for 'PENDING' events.
-- 6. It attempts to publish the event to a message broker (Kafka, RabbitMQ, etc.).
-- 7. Updates the status to 'PUBLISHED' or 'FAILED' accordingly.

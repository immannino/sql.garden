-- transactional-outbox.mysql.sql for MySQL

CREATE TABLE outbox_events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,       -- Use BIGINT AUTO_INCREMENT
    aggregate_type VARCHAR(255) NOT NULL,       -- Use VARCHAR or TEXT
    aggregate_id VARCHAR(255) NOT NULL,         -- Use VARCHAR or TEXT
    event_type VARCHAR(255) NOT NULL,           -- Use VARCHAR or TEXT
    payload JSON NOT NULL,                      -- Use JSON type (available in MySQL 5.7.8+)
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, -- Standard TIMESTAMP
    last_attempt_at TIMESTAMP NULL DEFAULT NULL, -- Allow NULL explicitly
    attempt_count INT NOT NULL DEFAULT 0,

    -- CHECK constraint (requires MySQL 8.0.16+)
    CONSTRAINT chk_status CHECK (status IN ('PENDING', 'PUBLISHING', 'PUBLISHED', 'FAILED'))
);

-- Index for the publisher process to find pending events
CREATE INDEX idx_outbox_events_status_created_at ON outbox_events(status, created_at);

-- Optional: Add index if querying by aggregate often
-- CREATE INDEX idx_outbox_events_aggregate ON outbox_events(aggregate_type, aggregate_id);
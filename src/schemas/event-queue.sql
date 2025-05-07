-- event_queue.sql
CREATE TABLE message_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    queue_name TEXT NOT NULL DEFAULT 'default', -- Allows for multiple queues
    payload TEXT NOT NULL,                  -- Message content (e.g., JSON)
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSING', 'DONE', 'FAILED')),
    priority INTEGER NOT NULL DEFAULT 0,    -- Higher number means higher priority
    max_attempts INTEGER NOT NULL DEFAULT 3,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, -- When the message can be processed (for delayed jobs)
    processing_started_at TIMESTAMP,        -- Timestamp when a worker picked it up
    last_error TEXT,                        -- Store error message on failure
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for workers to efficiently find the next available message
CREATE INDEX idx_message_queue_polling ON message_queue(queue_name, status, available_at, priority DESC);

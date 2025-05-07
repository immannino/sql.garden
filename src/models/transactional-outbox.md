# Transactional Outbox Pattern

> This is an early first pass prototype of a sql.garden schema guide. Expect these to evolve over time

The Transactional Outbox pattern ensures reliable message delivery for events related to database transactions. Instead of publishing an event directly within the main database transaction (which could fail after the transaction commits, or vice-versa), you write the event to a dedicated `outbox_events` table within the *same* transaction.

A separate process then polls this table, publishes the events to your message broker (like Kafka, RabbitMQ, etc.), and marks the events as published. This guarantees that an event is only published if the corresponding database transaction was successful.

Below are the schema definitions and accompanying SQLC query files for implementing this pattern in SQLite, PostgreSQL, and MySQL.

---

## SQLite

Suitable for simpler applications or environments where an embedded database is preferred.

### Schema (`schema.sqlite.sql`)

```sql run=false
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
```

### SQLC Queries (`queries.sqlite.sql`)

```sql run=false
-- transactional-outbox.queries.sql

-- name: CreateOutboxEvent :one
-- Create a new event to be published. This should be called within the same
-- transaction that modifies the application's state.
INSERT INTO outbox_events (
    aggregate_type,
    aggregate_id,
    event_type,
    payload
) VALUES (
    ?, ?, ?, ?
)
RETURNING *;

-- name: GetEventsToProcess :many
-- Fetch a batch of events that are ready to be processed (PENDING or FAILED with retries left).
-- The caller should handle potential race conditions if multiple publishers are running.
-- Consider adding a max_attempts column to the schema for more robust retry logic.
SELECT *
FROM outbox_events
WHERE status = 'PENDING' OR (status = 'FAILED' AND attempt_count < sqlc.arg(max_attempts))
ORDER BY created_at
LIMIT ?;

-- name: MarkEventForProcessing :execrows
-- Mark a specific event as being processed. This helps prevent other workers
-- from picking up the same event simultaneously. Update attempt count and timestamp.
UPDATE outbox_events
SET status = 'PUBLISHING',
    attempt_count = attempt_count + 1,
    last_attempt_at = CURRENT_TIMESTAMP
WHERE id = ? AND status IN ('PENDING', 'FAILED'); -- Ensure we only mark events that are actually ready

-- name: MarkEventAsPublished :exec
-- Mark an event as successfully published.
UPDATE outbox_events
SET status = 'PUBLISHED'
WHERE id = ?;

-- name: MarkEventAsFailed :exec
-- Mark an event as failed after processing attempt.
UPDATE outbox_events
SET status = 'FAILED'
WHERE id = ?;

-- name: DeletePublishedEventsOlderThan :execrows
-- Clean up events that were successfully published before a certain timestamp.
DELETE FROM outbox_events
WHERE status = 'PUBLISHED' AND created_at < ?;
```

---

## PostgreSQL

Leverages PostgreSQL-specific features like `BIGSERIAL`, `JSONB`, and `TIMESTAMPTZ`.

### Schema (`schema.pg.sql`)

```sql run=false
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
```

### SQLC Queries (`queries.pg.sql`)

```sql run=false
-- transactional-outbox.pg.queries.sql for PostgreSQL

-- name: CreateOutboxEvent :one
-- Create a new event to be published. This should be called within the same
-- transaction that modifies the application's state.
INSERT INTO outbox_events (
    aggregate_type,
    aggregate_id,
    event_type,
    payload
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetEventsToProcess :many
-- Fetch a batch of events that are ready to be processed (PENDING or FAILED with retries left).
-- The caller should handle potential race conditions if multiple publishers are running.
-- Consider adding a max_attempts column to the schema for more robust retry logic.
SELECT *
FROM outbox_events
WHERE status = 'PENDING' OR (status = 'FAILED' AND attempt_count < sqlc.arg(max_attempts))
ORDER BY created_at
LIMIT $1; -- Use $1 for the limit parameter

-- name: MarkEventForProcessing :execrows
-- Mark a specific event as being processed. This helps prevent other workers
-- from picking up the same event simultaneously. Update attempt count and timestamp.
UPDATE outbox_events
SET status = 'PUBLISHING',
    attempt_count = attempt_count + 1,
    last_attempt_at = NOW() -- Use NOW() for current timestamp
WHERE id = $1 AND status IN ('PENDING', 'FAILED'); -- Ensure we only mark events that are actually ready

-- name: MarkEventAsPublished :exec
-- Mark an event as successfully published.
UPDATE outbox_events
SET status = 'PUBLISHED'
WHERE id = $1;

-- name: MarkEventAsFailed :exec
-- Mark an event as failed after processing attempt.
UPDATE outbox_events
SET status = 'FAILED'
WHERE id = $1;

-- name: DeletePublishedEventsOlderThan :execrows
-- Clean up events that were successfully published before a certain timestamp.
DELETE FROM outbox_events
WHERE status = 'PUBLISHED' AND created_at < $1;
```

---

## MySQL

Uses standard MySQL syntax including `AUTO_INCREMENT` and `JSON` types. Note the `CHECK` constraint requires MySQL 8.0.16+.

### Schema (`schema.mysql.sql`)

```sql run=false
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
```

### SQLC Queries (`queries.mysql.sql`)

```sql run=false
-- transactional-outbox.mysql.queries.sql for MySQL

-- name: CreateOutboxEvent :one
-- Create a new event to be published. This should be called within the same
-- transaction that modifies the application's state.
-- NOTE: MySQL doesn't have RETURNING. sqlc typically uses LAST_INSERT_ID()
--       behind the scenes to fetch the inserted row for :one.
INSERT INTO outbox_events (
    aggregate_type,
    aggregate_id,
    event_type,
    payload
) VALUES (
    ?, ?, ?, ?
);

-- name: GetEventsToProcess :many
-- Fetch a batch of events that are ready to be processed (PENDING or FAILED with retries left).
-- The caller should handle potential race conditions if multiple publishers are running.
-- Consider adding a max_attempts column to the schema for more robust retry logic.
SELECT *
FROM outbox_events
WHERE status = 'PENDING' OR (status = 'FAILED' AND attempt_count < sqlc.arg(max_attempts))
ORDER BY created_at
LIMIT ?;

-- name: MarkEventForProcessing :execrows
-- Mark a specific event as being processed. This helps prevent other workers
-- from picking up the same event simultaneously. Update attempt count and timestamp.
UPDATE outbox_events
SET status = 'PUBLISHING',
    attempt_count = attempt_count + 1,
    last_attempt_at = CURRENT_TIMESTAMP -- CURRENT_TIMESTAMP works
WHERE id = ? AND status IN ('PENDING', 'FAILED'); -- Ensure we only mark events that are actually ready

-- name: MarkEventAsPublished :exec
-- Mark an event as successfully published.
UPDATE outbox_events
SET status = 'PUBLISHED'
WHERE id = ?;

-- name: MarkEventAsFailed :exec
-- Mark an event as failed after processing attempt.
UPDATE outbox_events
SET status = 'FAILED'
WHERE id = ?;

-- name: DeletePublishedEventsOlderThan :execrows
-- Clean up events that were successfully published before a certain timestamp.
DELETE FROM outbox_events
WHERE status = 'PUBLISHED' AND created_at < ?;
```
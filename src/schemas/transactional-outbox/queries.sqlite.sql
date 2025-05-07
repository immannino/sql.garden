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
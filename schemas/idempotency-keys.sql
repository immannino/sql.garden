-- idempotency_keys.sql
CREATE TABLE idempotency_keys (
    key TEXT PRIMARY KEY NOT NULL,          -- The unique idempotency key provided by the client
    request_hash TEXT NOT NULL,             -- A hash of the request payload to ensure the key is used for the same request
    user_id INTEGER,                        -- Optional: Associate the key with a specific user
    response_status_code INTEGER,           -- Store the HTTP status code of the original response
    response_body TEXT,                     -- Store the body of the original response (consider size limits)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Optional: Add foreign key constraint if you have a users table
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Index for faster lookups
CREATE INDEX idx_idempotency_keys_last_used ON idempotency_keys(last_used_at);
-- Optional: Index for user-specific lookups
-- CREATE INDEX idx_idempotency_keys_user_id ON idempotency_keys(user_id);

-- key_value_store.sql
CREATE TABLE kv_store (
    key TEXT PRIMARY KEY NOT NULL,          -- The unique key
    value BLOB NOT NULL,                    -- Store value as BLOB to accommodate various types (text, JSON, binary)
    expires_at TIMESTAMP,                   -- Optional: TTL for the key
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster lookup of expiring keys (if used frequently)
CREATE INDEX idx_kv_store_expires_at ON kv_store(expires_at);

-- Trigger to update updated_at timestamp
CREATE TRIGGER kv_store_updated_at
AFTER UPDATE ON kv_store
FOR EACH ROW
BEGIN
    UPDATE kv_store SET updated_at = CURRENT_TIMESTAMP WHERE key = OLD.key;
END;

-- Note: For performance-critical KV operations, dedicated systems like Redis or Memcached are usually preferred.
-- This SQLite version is simpler and integrated but won't match their speed.

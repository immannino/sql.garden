-- feature_flags.sql
CREATE TABLE feature_flags (
    flag_key TEXT PRIMARY KEY NOT NULL,     -- Unique identifier for the feature flag (e.g., 'new-dashboard', 'beta-checkout')
    description TEXT,                       -- Optional description of what the flag controls
    is_enabled INTEGER NOT NULL DEFAULT 0,  -- 0 for disabled, 1 for enabled (globally)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    -- More advanced: Add columns for targeting rules (e.g., user_ids TEXT, percentage REAL)
);

-- Trigger to update updated_at timestamp
CREATE TRIGGER feature_flags_updated_at
AFTER UPDATE ON feature_flags
FOR EACH ROW
BEGIN
    UPDATE feature_flags SET updated_at = CURRENT_TIMESTAMP WHERE flag_key = OLD.flag_key;
END;

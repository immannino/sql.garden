-- short_urls.sql
CREATE TABLE short_urls (
    short_code TEXT PRIMARY KEY NOT NULL,   -- The unique short identifier (e.g., 'aBcDeF')
    original_url TEXT NOT NULL,             -- The full URL to redirect to
    user_id INTEGER,                        -- Optional: User who created the short link
    visit_count INTEGER NOT NULL DEFAULT 0, -- How many times the link has been visited
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_visited_at TIMESTAMP,
    expires_at TIMESTAMP,                   -- Optional: When the link should expire
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- Index for potential lookups by original URL (if needed)
CREATE INDEX idx_short_urls_original_url ON short_urls(original_url);
-- Index for user-specific links
CREATE INDEX idx_short_urls_user_id ON short_urls(user_id);

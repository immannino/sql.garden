-- page_views.sql
CREATE TABLE page_views (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL,                      -- The URL that was viewed
    referrer TEXT,                          -- The referring URL (if available)
    user_agent TEXT,                        -- Browser/client information
    ip_address TEXT,                        -- User's IP address (consider privacy implications/anonymization)
    user_id INTEGER,                        -- Optional: Link to the users table if logged in
    viewed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL -- Keep view record even if user is deleted
);

-- Indexes for common queries
CREATE INDEX idx_page_views_url ON page_views(url);
CREATE INDEX idx_page_views_viewed_at ON page_views(viewed_at);
CREATE INDEX idx_page_views_user_id ON page_views(user_id);

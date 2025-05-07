-- users.sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,   -- Unique user identifier
    email TEXT UNIQUE NOT NULL,             -- User's email address (often used for login)
    username TEXT UNIQUE,                   -- Optional: A unique username
    hashed_password TEXT NOT NULL,          -- Store a securely hashed password, NEVER plaintext
    full_name TEXT,                         -- User's full name
    is_active INTEGER NOT NULL DEFAULT 1,   -- 0 for inactive/banned, 1 for active
    is_admin INTEGER NOT NULL DEFAULT 0,    -- Simple admin flag (0 for regular user, 1 for admin)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common lookups
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);

-- Trigger to update updated_at timestamp
CREATE TRIGGER users_updated_at
AFTER UPDATE ON users
FOR EACH ROW
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- web_crawler.sql
CREATE TABLE crawl_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT UNIQUE NOT NULL,               -- URL to be crawled
    priority INTEGER NOT NULL DEFAULT 0,    -- Higher number means higher priority
    added_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_attempt_at TIMESTAMP,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSING', 'DONE', 'FAILED'))
);

CREATE TABLE crawled_pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT UNIQUE NOT NULL,               -- The URL that was successfully crawled
    http_status_code INTEGER,               -- e.g., 200, 404, 500
    content_type TEXT,                      -- e.g., 'text/html', 'application/json'
    content_hash TEXT,                      -- Hash of the content to detect changes
    page_title TEXT,
    -- raw_content BLOB,                    -- Optional: Store the full raw content (can make DB large)
    extracted_text TEXT,                    -- Optional: Store just the extracted text content
    last_crawled_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    linked_from_url TEXT                    -- Optional: The URL that linked to this page
);

CREATE TABLE page_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_page_id INTEGER NOT NULL,        -- FK to crawled_pages.id
    target_url TEXT NOT NULL,               -- The URL found on the source page
    link_text TEXT,
    FOREIGN KEY (source_page_id) REFERENCES crawled_pages(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX idx_crawl_queue_status_priority ON crawl_queue(status, priority DESC);
CREATE INDEX idx_crawled_pages_last_crawled ON crawled_pages(last_crawled_at);
CREATE INDEX idx_page_links_source_page ON page_links(source_page_id);
CREATE INDEX idx_page_links_target_url ON page_links(target_url); -- To find pages linking to a specific URL

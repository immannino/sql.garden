-- search_index.sql

-- Table to store the original documents/content
CREATE TABLE documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    external_id TEXT UNIQUE,                -- Optional: ID from your main application
    title TEXT,
    body TEXT NOT NULL,
    url TEXT,                               -- Optional: Link back to the source
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create the FTS5 virtual table for indexing
CREATE VIRTUAL TABLE documents_fts USING fts5(
    title,
    body,
    content='documents',                    -- Link to the documents table
    content_rowid='id'                      -- Link using the rowid (primary key)
    -- Add more options like tokenizers if needed: tokenize = 'porter unicode61'
);

-- Triggers to keep the FTS table synchronized with the documents table
CREATE TRIGGER documents_ai AFTER INSERT ON documents BEGIN
  INSERT INTO documents_fts (rowid, title, body) VALUES (new.id, new.title, new.body);
END;

CREATE TRIGGER documents_ad AFTER DELETE ON documents BEGIN
  DELETE FROM documents_fts WHERE rowid = old.id;
END;

CREATE TRIGGER documents_au AFTER UPDATE ON documents BEGIN
  UPDATE documents_fts SET title = new.title, body = new.body WHERE rowid = old.id;
END;

-- Trigger to update updated_at timestamp on documents table
CREATE TRIGGER documents_updated_at
AFTER UPDATE ON documents
FOR EACH ROW
BEGIN
    UPDATE documents SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- Example Search Query:
-- SELECT d.title, d.url
-- FROM documents d
-- JOIN documents_fts fts ON d.id = fts.rowid
-- WHERE fts.documents_fts MATCH 'your search query'
-- ORDER BY rank; -- rank is implicitly provided by FTS5

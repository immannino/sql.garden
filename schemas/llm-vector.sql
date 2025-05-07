-- vector_storage.sql
CREATE TABLE text_embeddings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_document_id TEXT,                -- Optional: Identifier for the original document/source
    chunk_sequence INTEGER,                 -- Optional: Order of this chunk within the source document
    text_content TEXT NOT NULL,             -- The actual text chunk
    embedding BLOB NOT NULL,                -- The vector embedding stored as a binary blob
    metadata TEXT,                          -- Optional: JSON or text metadata associated with the chunk
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster lookup by source document
CREATE INDEX idx_text_embeddings_source_doc ON text_embeddings(source_document_id);

-- Trigger to update updated_at timestamp
CREATE TRIGGER text_embeddings_updated_at
AFTER UPDATE ON text_embeddings
FOR EACH ROW
BEGIN
    UPDATE text_embeddings SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- Note: SQLite doesn't have native vector indexing.
-- Searching would involve retrieving embeddings and calculating distances (e.g., cosine similarity)
-- in the application layer or using an extension like sqlite-vss.

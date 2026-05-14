-- Core Data
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS items (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    type       TEXT NOT NULL CHECK(type IN ('text','image','video','file')),
    content    TEXT NOT NULL,
    filename   TEXT,
    filesize   INTEGER,
    metadata   TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_items_id_desc
    ON items(id DESC);

CREATE INDEX IF NOT EXISTS idx_items_type_id
    ON items(type, id DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_expires
    ON sessions(expires_at);

-- Full-Text Search (FTS5)
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts
    USING fts5(
        content,
        filename,
        content='items',
        content_rowid='id',
        tokenize='unicode61 remove_diacritics 1'
    );

-- Sync triggers
CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
    INSERT INTO items_fts(rowid, content, filename)
    VALUES (new.id, new.content, COALESCE(new.filename, ''));
END;

CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, content, filename)
    VALUES ('delete', old.id, old.content, COALESCE(old.filename, ''));
END;

CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, content, filename)
    VALUES ('delete', old.id, old.content, COALESCE(old.filename, ''));
    INSERT INTO items_fts(rowid, content, filename)
    VALUES (new.id, new.content, COALESCE(new.filename, ''));
END;

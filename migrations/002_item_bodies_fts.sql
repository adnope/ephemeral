CREATE VIRTUAL TABLE IF NOT EXISTS item_bodies_fts
USING fts5(
    filename UNINDEXED,
    body,
    tokenize = 'unicode61 remove_diacritics 1'
);

CREATE TABLE IF NOT EXISTS item_body_index_state (
    item_id    INTEGER PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    status     TEXT NOT NULL CHECK(status IN ('indexed', 'skipped', 'failed')),
    body_bytes INTEGER NOT NULL DEFAULT 0,
    error      TEXT,
    indexed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS item_bodies_ad_cleanup
AFTER DELETE ON items
BEGIN
    DELETE FROM item_bodies_fts WHERE rowid = old.id;
    DELETE FROM item_body_index_state WHERE item_id = old.id;
END;

CREATE TRIGGER IF NOT EXISTS item_bodies_au_cleanup
AFTER UPDATE OF type, content, filename ON items
BEGIN
    DELETE FROM item_bodies_fts WHERE rowid = old.id;
    DELETE FROM item_body_index_state WHERE item_id = old.id;
END;
CREATE TABLE IF NOT EXISTS public_links (
    token      TEXT PRIMARY KEY,
    item_id    INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_public_links_item_id
    ON public_links(item_id);

CREATE INDEX IF NOT EXISTS idx_public_links_expires_at
    ON public_links(expires_at);

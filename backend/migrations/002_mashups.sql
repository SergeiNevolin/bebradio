-- Standalone /mashup section: user-uploaded audio, transcoded on media-service,
-- metadata (source of truth for state) kept here.
CREATE TABLE IF NOT EXISTS mashups (
    id         VARCHAR(8) PRIMARY KEY,
    owner_id   VARCHAR(8) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      VARCHAR(200) NOT NULL,
    artist     VARCHAR(200) DEFAULT '',
    media_id   VARCHAR(64) NOT NULL,
    duration   INTEGER DEFAULT 0,
    size_bytes BIGINT DEFAULT 0,
    status     VARCHAR(16) NOT NULL DEFAULT 'processing',
    error      TEXT DEFAULT '',
    has_cover  BOOLEAN DEFAULT FALSE,
    plays      INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_mashups_created ON mashups (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mashups_owner   ON mashups (owner_id);

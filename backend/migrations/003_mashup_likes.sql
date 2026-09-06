-- Likes for mashups (one row per user per mashup) plus a denormalised counter
-- on mashups so the "top by likes" list can sort without an aggregate join.
-- A user-uploaded cover bumps cover_updated_at so clients can bust their cache.
ALTER TABLE mashups ADD COLUMN IF NOT EXISTS likes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mashups ADD COLUMN IF NOT EXISTS cover_updated_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS mashup_likes (
    mashup_id  VARCHAR(8) NOT NULL REFERENCES mashups(id) ON DELETE CASCADE,
    user_id    VARCHAR(8) NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (mashup_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_mashup_likes_user ON mashup_likes (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mashups_likes     ON mashups (likes DESC, created_at DESC);

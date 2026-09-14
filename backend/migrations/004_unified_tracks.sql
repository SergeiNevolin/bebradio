-- One Track model: uploads move from `mashups` into `tracks` with
-- source='upload' (room queue rows are source='youtube', library rows have
-- room_id NULL). Likes move from `mashup_likes` to `track_likes`.
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS source VARCHAR(16) NOT NULL DEFAULT 'youtube';
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS owner_id VARCHAR(8);
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'ready';
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS error TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS size_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS has_cover BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS cover_updated_at TIMESTAMPTZ;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS plays INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tracks ADD COLUMN IF NOT EXISTS likes INTEGER NOT NULL DEFAULT 0;

-- Existing queue rows predate the column: they are all YouTube entries.
UPDATE tracks SET source = 'youtube' WHERE source IS NULL OR source = '';

-- Move library rows over (guard against coincidental id collision with queue rows).
INSERT INTO tracks (id, room_id, source, owner_id, title, artist, url, thumbnail,
    duration, added_by, position_index, source_url, local_path, media_id, added_at,
    status, error, size_bytes, has_cover, cover_updated_at, plays, likes)
SELECT m.id, NULL, 'upload', m.owner_id, m.title, m.artist, '', '',
    m.duration, u.username, 0, '', '', m.media_id, m.created_at,
    m.status, m.error, m.size_bytes, m.has_cover, m.cover_updated_at, m.plays, m.likes
FROM mashups m JOIN users u ON u.id = m.owner_id
WHERE NOT EXISTS (SELECT 1 FROM tracks t WHERE t.id = m.id);

CREATE TABLE IF NOT EXISTS track_likes (
    track_id   VARCHAR(8) NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    user_id    VARCHAR(8) NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (track_id, user_id)
);

INSERT INTO track_likes (track_id, user_id, created_at)
SELECT l.mashup_id, l.user_id, l.created_at FROM mashup_likes l
WHERE EXISTS (SELECT 1 FROM tracks t WHERE t.id = l.mashup_id)
ON CONFLICT DO NOTHING;

DROP TABLE IF EXISTS mashup_likes;
DROP TABLE IF EXISTS mashups;

CREATE INDEX IF NOT EXISTS idx_tracks_library ON tracks (room_id) WHERE room_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_tracks_owner ON tracks (owner_id) WHERE owner_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_track_likes_user ON track_likes (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tracks_likes ON tracks (likes DESC, added_at DESC) WHERE room_id IS NULL;

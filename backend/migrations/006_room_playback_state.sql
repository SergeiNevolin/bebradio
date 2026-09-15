-- The queue pointer and the playing flag belong to the room: without them a
-- restart loads every room paused at index 0 with no way to resume.
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS current_index INTEGER NOT NULL DEFAULT 0;
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS is_playing BOOLEAN NOT NULL DEFAULT FALSE;
-- Tracks wall-clock time the current track started, used to skip Duration==0
-- tracks (live streams, unresolved) after a timeout.
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS current_started_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

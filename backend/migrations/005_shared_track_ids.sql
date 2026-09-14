-- Queue snapshots of uploads share the library track id, so tracks.id is no
-- longer globally unique: library rows are unique by id, queue rows by
-- (room_id, id). Like counters are maintained manually (see TrackRepo.Delete).
ALTER TABLE track_likes DROP CONSTRAINT IF EXISTS track_likes_track_id_fkey;
ALTER TABLE tracks DROP CONSTRAINT IF EXISTS tracks_pkey;
CREATE UNIQUE INDEX IF NOT EXISTS tracks_library_id_uidx ON tracks (id) WHERE room_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS tracks_queue_uidx ON tracks (room_id, id) WHERE room_id IS NOT NULL;

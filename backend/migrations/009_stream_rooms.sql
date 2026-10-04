-- Stream rooms: created by admins, only admins add tracks, no voting.
-- Readers join and listen exactly like in regular rooms.
ALTER TABLE rooms ADD COLUMN is_stream BOOLEAN NOT NULL DEFAULT FALSE;

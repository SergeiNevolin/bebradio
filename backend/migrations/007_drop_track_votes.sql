-- Room votes live in Redis only: per-track ephemeral state cleared on every
-- track switch. The Postgres mirror was never written by the server and only
-- risked resurrecting stale votes on cold-start hydration.
DROP TABLE IF EXISTS track_votes;

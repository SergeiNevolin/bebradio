package postgres

import (
	"context"
	"fmt"
)

// schema brings an empty database up to the current shape, and is a no-op on a
// database that already has it.
//
// The statements deliberately match the tables the previous Python service
// created through SQLAlchemy, column type for column type, so an existing
// production database is adopted rather than migrated. Every statement is
// idempotent, which is what lets the service run them on every boot without a
// separate migration step or a version table.
var schema = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id            VARCHAR(8) PRIMARY KEY,
		email         VARCHAR(255) NOT NULL UNIQUE,
		username      VARCHAR(30) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		bio           TEXT DEFAULT '',
		avatar_url    TEXT DEFAULT '',
		created_at    DOUBLE PRECISION DEFAULT EXTRACT(EPOCH FROM NOW())
	)`,
	`CREATE INDEX IF NOT EXISTS ix_users_email ON users (email)`,

	`CREATE TABLE IF NOT EXISTS rooms (
		id                  VARCHAR(6) PRIMARY KEY,
		name                VARCHAR(255) NOT NULL DEFAULT 'My Room',
		owner_id            VARCHAR(8) NOT NULL,
		allow_anonymous_add BOOLEAN DEFAULT TRUE,
		is_private          BOOLEAN DEFAULT FALSE,
		password_hash       VARCHAR(255),
		auto_radio          BOOLEAN DEFAULT FALSE,
		created_at          DOUBLE PRECISION DEFAULT EXTRACT(EPOCH FROM NOW())
	)`,

	`CREATE TABLE IF NOT EXISTS tracks (
		id                VARCHAR(8) PRIMARY KEY,
		room_id           VARCHAR(6) NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
		title             VARCHAR(500) DEFAULT '',
		artist            VARCHAR(500) DEFAULT '',
		url               TEXT DEFAULT '',
		thumbnail         TEXT DEFAULT '',
		duration          INTEGER DEFAULT 0,
		added_by          VARCHAR(30) DEFAULT 'Anonymous',
		position_index    INTEGER NOT NULL DEFAULT 0,
		source_url        TEXT DEFAULT '',
		stream_expires_at DOUBLE PRECISION DEFAULT 0,
		added_at          DOUBLE PRECISION DEFAULT EXTRACT(EPOCH FROM NOW())
	)`,

	`CREATE TABLE IF NOT EXISTS chat_messages (
		id         VARCHAR(8) PRIMARY KEY,
		room_id    VARCHAR(6) NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
		user_id    VARCHAR(8) DEFAULT '',
		username   VARCHAR(30) DEFAULT '',
		text       TEXT DEFAULT '',
		created_at DOUBLE PRECISION DEFAULT EXTRACT(EPOCH FROM NOW())
	)`,

	`CREATE TABLE IF NOT EXISTS track_votes (
		id       SERIAL PRIMARY KEY,
		room_id  VARCHAR(6) NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
		user_id  VARCHAR(8) NOT NULL,
		track_id VARCHAR(8) NOT NULL,
		vote     INTEGER NOT NULL DEFAULT 0
	)`,

	// Columns added after the initial schema. CREATE TABLE IF NOT EXISTS never
	// alters an existing table, so a database provisioned before these columns
	// existed needs them added by hand.
	`ALTER TABLE rooms  ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255)`,
	`ALTER TABLE rooms  ADD COLUMN IF NOT EXISTS auto_radio BOOLEAN DEFAULT FALSE`,
	`ALTER TABLE users  ADD COLUMN IF NOT EXISTS bio TEXT DEFAULT ''`,
	`ALTER TABLE users  ADD COLUMN IF NOT EXISTS avatar_url TEXT DEFAULT ''`,
	`ALTER TABLE tracks ADD COLUMN IF NOT EXISTS source_url TEXT DEFAULT ''`,
	`ALTER TABLE tracks ADD COLUMN IF NOT EXISTS stream_expires_at DOUBLE PRECISION DEFAULT 0`,

	// Indexes matching how the tables are actually read: a room's queue in
	// order, its chat in order, and its votes.
	`CREATE INDEX IF NOT EXISTS ix_tracks_room_position ON tracks (room_id, position_index)`,
	`CREATE INDEX IF NOT EXISTS ix_chat_messages_room_created ON chat_messages (room_id, created_at)`,
	`CREATE INDEX IF NOT EXISTS ix_track_votes_room ON track_votes (room_id)`,
}

// Migrate applies the schema. It is safe to run on every boot and against a
// database that already carries data.
func (s *Store) Migrate(ctx context.Context) error {
	for i, stmt := range schema {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("postgres: applying schema statement %d: %w", i+1, err)
		}
	}
	return nil
}

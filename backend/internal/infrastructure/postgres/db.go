package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
	Log  *slog.Logger
}

func New(databaseURL string, log *slog.Logger) (*DB, error) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &DB{Pool: pool, Log: log}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) Migrate() error {
	ctx := context.Background()
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(8) PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(30) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			bio TEXT DEFAULT '',
			avatar_url TEXT DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS rooms (
			id VARCHAR(6) PRIMARY KEY,
			name VARCHAR(255) NOT NULL DEFAULT 'My Room',
			owner_id VARCHAR(8) NOT NULL,
			allow_anonymous_add BOOLEAN DEFAULT TRUE,
			is_private BOOLEAN DEFAULT FALSE,
			password_hash VARCHAR(255),
			auto_radio BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS tracks (
			id VARCHAR(8) PRIMARY KEY,
			room_id VARCHAR(6) REFERENCES rooms(id) ON DELETE CASCADE,
			title VARCHAR(500) DEFAULT '',
			artist VARCHAR(500) DEFAULT '',
			url TEXT DEFAULT '',
			thumbnail TEXT DEFAULT '',
			duration INTEGER DEFAULT 0,
			added_by VARCHAR(30) DEFAULT 'Anonymous',
			position_index INTEGER NOT NULL DEFAULT 0,
			source_url TEXT DEFAULT '',
			local_path TEXT DEFAULT '',
			media_id VARCHAR(64) DEFAULT '',
			added_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id VARCHAR(8) PRIMARY KEY,
			room_id VARCHAR(6) REFERENCES rooms(id) ON DELETE CASCADE,
			user_id VARCHAR(8) DEFAULT '',
			username VARCHAR(30) DEFAULT '',
			text TEXT DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS track_votes (
			id SERIAL PRIMARY KEY,
			room_id VARCHAR(6) REFERENCES rooms(id) ON DELETE CASCADE,
			user_id VARCHAR(8) NOT NULL,
			track_id VARCHAR(8) NOT NULL,
			vote INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS user_room_visits (
			user_id VARCHAR(8) NOT NULL,
			room_id VARCHAR(6) NOT NULL,
			visited_at TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (user_id, room_id)
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Pool.Exec(ctx, m); err != nil {
			db.Log.Error("migration failed", "error", err)
			return err
		}
	}
	db.Log.Info("database migrations completed")
	return nil
}

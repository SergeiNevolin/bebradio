// Package postgres implements store.Store on PostgreSQL via pgx.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/store"
)

// uniqueViolation is PostgreSQL's SQLSTATE for a unique constraint breach.
const uniqueViolation = "23505"

// Options configure the pool.
type Options struct {
	DatabaseURL    string
	MaxConns       int32
	MinConns       int32
	ConnectTimeout time.Duration
}

// Store is a PostgreSQL-backed store.Store.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to PostgreSQL and verifies the connection.
func Open(ctx context.Context, opts Options) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(opts.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: parsing DATABASE_URL: %w", err)
	}
	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	if opts.MinConns > 0 {
		cfg.MinConns = opts.MinConns
	}
	if opts.ConnectTimeout > 0 {
		cfg.ConnConfig.ConnectTimeout = opts.ConnectTimeout
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: creating pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: connecting: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Ping reports whether the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Close releases the connection pool.
func (s *Store) Close() { s.pool.Close() }

// --- Users ---

const userColumns = `id, email, username, password_hash, COALESCE(bio, ''), COALESCE(avatar_url, ''), COALESCE(created_at, 0)`

func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, email, username, password_hash, bio, avatar_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		user.ID, user.Email, user.Username, user.PasswordHash, user.Bio, user.AvatarURL, user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return store.ErrConflict
		}
		return fmt.Errorf("postgres: creating user: %w", err)
	}
	return nil
}

func (s *Store) UserByID(ctx context.Context, id string) (domain.User, error) {
	return s.userBy(ctx, "id", id)
}

func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	return s.userBy(ctx, "email", email)
}

func (s *Store) UserByUsername(ctx context.Context, username string) (domain.User, error) {
	return s.userBy(ctx, "username", username)
}

// userBy looks a user up by one of a fixed set of columns. The column name is
// chosen by this package, never by a caller, so it cannot carry injection.
func (s *Store) userBy(ctx context.Context, column, value string) (domain.User, error) {
	var u domain.User
	err := s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE `+column+` = $1`, value,
	).Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Bio, &u.AvatarURL, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, store.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres: loading user by %s: %w", column, err)
	}
	return u, nil
}

func (s *Store) UpdateProfile(ctx context.Context, id string, bio, avatarURL *string) (domain.User, error) {
	var u domain.User
	// COALESCE leaves the stored value alone when the caller passes NULL, so a
	// partial update does not blank the fields it omits.
	err := s.pool.QueryRow(ctx, `
		UPDATE users
		   SET bio = COALESCE($2, bio),
		       avatar_url = COALESCE($3, avatar_url)
		 WHERE id = $1
		RETURNING `+userColumns,
		id, bio, avatarURL,
	).Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.Bio, &u.AvatarURL, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, store.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("postgres: updating profile: %w", err)
	}
	return u, nil
}

// --- Rooms ---

func (s *Store) SaveRoom(ctx context.Context, room store.RoomRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO rooms (id, name, owner_id, allow_anonymous_add, is_private, password_hash, auto_radio, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			allow_anonymous_add = EXCLUDED.allow_anonymous_add,
			is_private = EXCLUDED.is_private,
			password_hash = EXCLUDED.password_hash,
			auto_radio = EXCLUDED.auto_radio`,
		room.ID, room.Name, room.OwnerID, room.AllowAnonymousAdd, room.IsPrivate,
		nullableString(room.PasswordHash), room.AutoRadio, room.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: saving room: %w", err)
	}
	return nil
}

func (s *Store) LoadRoom(ctx context.Context, id string) (store.RoomContents, error) {
	var out store.RoomContents

	var passwordHash *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, owner_id,
		       COALESCE(allow_anonymous_add, TRUE), COALESCE(is_private, FALSE),
		       password_hash, COALESCE(auto_radio, FALSE), COALESCE(created_at, 0)
		  FROM rooms WHERE id = $1`, id,
	).Scan(&out.Room.ID, &out.Room.Name, &out.Room.OwnerID,
		&out.Room.AllowAnonymousAdd, &out.Room.IsPrivate,
		&passwordHash, &out.Room.AutoRadio, &out.Room.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, store.ErrNotFound
	}
	if err != nil {
		return out, fmt.Errorf("postgres: loading room: %w", err)
	}
	if passwordHash != nil {
		out.Room.PasswordHash = *passwordHash
	}

	if out.Tracks, err = s.loadTracks(ctx, id); err != nil {
		return out, err
	}
	if out.Messages, err = s.loadMessages(ctx, id); err != nil {
		return out, err
	}
	if out.Votes, err = s.loadVotes(ctx, id); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Store) loadTracks(ctx context.Context, roomID string) ([]*domain.Track, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(title, ''), COALESCE(artist, ''), COALESCE(url, ''),
		       COALESCE(thumbnail, ''), COALESCE(duration, 0), COALESCE(added_by, 'Anonymous'),
		       COALESCE(source_url, ''), COALESCE(stream_expires_at, 0), COALESCE(added_at, 0)
		  FROM tracks WHERE room_id = $1 ORDER BY position_index`, roomID)
	if err != nil {
		return nil, fmt.Errorf("postgres: loading tracks: %w", err)
	}
	defer rows.Close()

	var tracks []*domain.Track
	for rows.Next() {
		t := &domain.Track{}
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.URL, &t.Thumbnail,
			&t.Duration, &t.AddedBy, &t.SourceURL, &t.StreamExpiresAt, &t.AddedAt); err != nil {
			return nil, fmt.Errorf("postgres: scanning track: %w", err)
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (s *Store) loadMessages(ctx context.Context, roomID string) ([]domain.ChatMessage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(user_id, ''), COALESCE(username, ''), COALESCE(text, ''), COALESCE(created_at, 0)
		  FROM chat_messages WHERE room_id = $1 ORDER BY created_at`, roomID)
	if err != nil {
		return nil, fmt.Errorf("postgres: loading messages: %w", err)
	}
	defer rows.Close()

	var messages []domain.ChatMessage
	for rows.Next() {
		var m domain.ChatMessage
		if err := rows.Scan(&m.ID, &m.UserID, &m.Username, &m.Text, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: scanning message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *Store) loadVotes(ctx context.Context, roomID string) ([]domain.TrackVote, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id, track_id, vote FROM track_votes WHERE room_id = $1 ORDER BY id`, roomID)
	if err != nil {
		return nil, fmt.Errorf("postgres: loading votes: %w", err)
	}
	defer rows.Close()

	var votes []domain.TrackVote
	for rows.Next() {
		var v domain.TrackVote
		if err := rows.Scan(&v.UserID, &v.TrackID, &v.Vote); err != nil {
			return nil, fmt.Errorf("postgres: scanning vote: %w", err)
		}
		votes = append(votes, v)
	}
	return votes, rows.Err()
}

func (s *Store) DeleteRoom(ctx context.Context, id string) error {
	// The child tables cascade from rooms, but they are deleted explicitly so
	// the behaviour does not depend on constraints an older database may lack.
	return s.inTx(ctx, func(tx pgx.Tx) error {
		for _, stmt := range []string{
			`DELETE FROM track_votes WHERE room_id = $1`,
			`DELETE FROM chat_messages WHERE room_id = $1`,
			`DELETE FROM tracks WHERE room_id = $1`,
			`DELETE FROM rooms WHERE id = $1`,
		} {
			if _, err := tx.Exec(ctx, stmt, id); err != nil {
				return fmt.Errorf("postgres: deleting room: %w", err)
			}
		}
		return nil
	})
}

func (s *Store) ListPublicRooms(ctx context.Context) ([]store.PublicRoom, error) {
	// One query with a lateral count, rather than a track query per room.
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.name, r.password_hash,
		       (SELECT COUNT(*) FROM tracks t WHERE t.room_id = r.id)
		  FROM rooms r
		 WHERE COALESCE(r.is_private, FALSE) = FALSE
		 ORDER BY r.created_at NULLS LAST, r.id`)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing rooms: %w", err)
	}
	defer rows.Close()

	out := []store.PublicRoom{}
	for rows.Next() {
		var (
			room         store.PublicRoom
			passwordHash *string
			trackCount   int64
		)
		if err := rows.Scan(&room.ID, &room.Name, &passwordHash, &trackCount); err != nil {
			return nil, fmt.Errorf("postgres: scanning room: %w", err)
		}
		room.TrackCount = int(trackCount)
		room.HasPassword = passwordHash != nil && *passwordHash != ""
		out = append(out, room)
	}
	return out, rows.Err()
}

// --- Room contents ---

func (s *Store) ReplaceTracks(ctx context.Context, roomID string, tracks []*domain.Track) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM tracks WHERE room_id = $1`, roomID); err != nil {
			return fmt.Errorf("postgres: clearing queue: %w", err)
		}
		if len(tracks) == 0 {
			return nil
		}
		batch := &pgx.Batch{}
		for i, t := range tracks {
			batch.Queue(`
				INSERT INTO tracks (id, room_id, title, artist, url, thumbnail, duration,
				                    added_by, position_index, source_url, stream_expires_at, added_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
				t.ID, roomID, t.Title, t.Artist, t.URL, t.Thumbnail, t.Duration,
				t.AddedBy, i, t.SourceURL, t.StreamExpiresAt, t.AddedAt)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return fmt.Errorf("postgres: writing queue: %w", err)
		}
		return nil
	})
}

func (s *Store) AppendMessage(ctx context.Context, roomID string, msg domain.ChatMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO chat_messages (id, room_id, user_id, username, text, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		msg.ID, roomID, msg.UserID, msg.Username, msg.Text, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: saving chat message: %w", err)
	}
	return nil
}

func (s *Store) ReplaceVotes(ctx context.Context, roomID string, votes []domain.TrackVote) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM track_votes WHERE room_id = $1`, roomID); err != nil {
			return fmt.Errorf("postgres: clearing votes: %w", err)
		}
		if len(votes) == 0 {
			return nil
		}
		batch := &pgx.Batch{}
		for _, v := range votes {
			batch.Queue(
				`INSERT INTO track_votes (room_id, user_id, track_id, vote) VALUES ($1,$2,$3,$4)`,
				roomID, v.UserID, v.TrackID, v.Vote)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return fmt.Errorf("postgres: writing votes: %w", err)
		}
		return nil
	})
}

// inTx runs fn inside a transaction, rolling back on error.
func (s *Store) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: beginning transaction: %w", err)
	}
	// Rollback after a successful commit is a no-op, so this is safe as a
	// blanket deferral.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: committing transaction: %w", err)
	}
	return nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

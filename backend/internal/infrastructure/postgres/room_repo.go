package postgres

import (
	"context"
	"fmt"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoomRepo struct {
	pool *pgxpool.Pool
}

func NewRoomRepo(pool *pgxpool.Pool) *RoomRepo {
	return &RoomRepo{pool: pool}
}

func (r *RoomRepo) Save(room *entity.Room) error {
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO rooms (id, name, owner_id, allow_anonymous_add, is_private, password_hash, auto_radio, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			allow_anonymous_add = EXCLUDED.allow_anonymous_add,
			is_private = EXCLUDED.is_private,
			password_hash = EXCLUDED.password_hash,
			auto_radio = EXCLUDED.auto_radio`,
		room.ID, room.Name, room.OwnerID, room.AllowAnonymousAdd, room.IsPrivate,
		room.PasswordHash, room.AutoRadio, room.CreatedAt,
	)
	return err
}

func (r *RoomRepo) FindByID(id string) (*entity.Room, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT id, name, owner_id, allow_anonymous_add, is_private, password_hash, auto_radio, created_at
		 FROM rooms WHERE id = $1`, id,
	)

	rm := &entity.Room{}
	var passwordHash *string
	err := row.Scan(&rm.ID, &rm.Name, &rm.OwnerID, &rm.AllowAnonymousAdd, &rm.IsPrivate,
		&passwordHash, &rm.AutoRadio, &rm.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("room not found: %w", err)
	}
	rm.PasswordHash = passwordHash
	rm.Queue = make([]*entity.Track, 0)
	rm.Messages = make([]*entity.ChatMessage, 0)
	rm.Votes = make([]*entity.TrackVote, 0)
	rm.SkipVotes = make(map[string]bool)
	rm.Presence = make(map[string]entity.PresenceInfo)
	rm.Users = make(map[string]string)
	rm.RadioSeen = make(map[string]bool)
	return rm, nil
}

func (r *RoomRepo) Delete(id string) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tx.Exec(ctx, `DELETE FROM track_votes WHERE room_id = $1`, id)
	tx.Exec(ctx, `DELETE FROM chat_messages WHERE room_id = $1`, id)
	tx.Exec(ctx, `DELETE FROM tracks WHERE room_id = $1`, id)
	tx.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, id)

	return tx.Commit(ctx)
}

func (r *RoomRepo) ListPublic() ([]map[string]any, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT id, name FROM rooms WHERE is_private = false ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []map[string]any
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		rooms = append(rooms, map[string]any{
			"id":        id,
			"name":      name,
			"user_count": 0,
			"track_count": 0,
			"is_playing": false,
			"has_password": false,
		})
	}
	return rooms, nil
}

func (r *RoomRepo) SaveTracks(room *entity.Room) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tx.Exec(ctx, `DELETE FROM tracks WHERE room_id = $1`, room.ID)

	for i, track := range room.Queue {
		_, err := tx.Exec(ctx,
			`INSERT INTO tracks (id, room_id, title, artist, url, thumbnail, duration, added_by, position_index, source_url, local_path, media_id, added_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			track.ID, room.ID, track.Title, track.Artist, track.URL, track.Thumbnail,
			track.Duration, track.AddedBy, i, track.SourceURL, track.LocalPath,
			track.MediaID, track.AddedAt,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *RoomRepo) LoadTracks(roomID string) ([]*entity.Track, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT id, title, artist, url, thumbnail, duration, added_by, source_url, local_path, media_id, position_index, added_at
		 FROM tracks WHERE room_id = $1 ORDER BY position_index`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []*entity.Track
	for rows.Next() {
		t := &entity.Track{}
		err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.URL, &t.Thumbnail, &t.Duration,
			&t.AddedBy, &t.SourceURL, &t.LocalPath, &t.MediaID, &t.Position, &t.AddedAt)
		if err != nil {
			continue
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func (r *RoomRepo) SaveMessage(roomID string, msg *entity.ChatMessage) error {
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO chat_messages (id, room_id, user_id, username, text, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		msg.ID, roomID, msg.UserID, msg.Username, msg.Text, msg.CreatedAt,
	)
	return err
}

func (r *RoomRepo) LoadMessages(roomID string) ([]*entity.ChatMessage, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT id, user_id, username, text, created_at
		 FROM chat_messages WHERE room_id = $1 ORDER BY created_at`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*entity.ChatMessage
	for rows.Next() {
		m := &entity.ChatMessage{}
		if err := rows.Scan(&m.ID, &m.UserID, &m.Username, &m.Text, &m.CreatedAt); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (r *RoomRepo) SaveVotes(room *entity.Room) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tx.Exec(ctx, `DELETE FROM track_votes WHERE room_id = $1`, room.ID)

	for _, v := range room.Votes {
		_, err := tx.Exec(ctx,
			`INSERT INTO track_votes (room_id, user_id, track_id, vote) VALUES ($1, $2, $3, $4)`,
			room.ID, v.UserID, v.TrackID, v.Vote,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *RoomRepo) LoadVotes(roomID string) ([]*entity.TrackVote, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT user_id, track_id, vote FROM track_votes WHERE room_id = $1`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var votes []*entity.TrackVote
	for rows.Next() {
		v := &entity.TrackVote{}
		if err := rows.Scan(&v.UserID, &v.TrackID, &v.Vote); err != nil {
			continue
		}
		votes = append(votes, v)
	}
	return votes, nil
}

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

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func NewRoomRepo(pool *pgxpool.Pool) *RoomRepo {
	return &RoomRepo{pool: pool}
}

func (r *RoomRepo) Save(room *entity.Room) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
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
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO sessions (kind, id, created_at) VALUES ('room', $1, $2)
		 ON CONFLICT DO NOTHING`, room.ID, room.CreatedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
	return rm, nil
}

func (r *RoomRepo) Delete(id string) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM chat_messages WHERE session_kind = 'room' AND session_id = $1`, id); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM queue_items WHERE session_kind = 'room' AND session_id = $1`, id); err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE kind = 'room' AND id = $1`, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete room: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *RoomRepo) ListPublic() ([]map[string]any, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT id, name, auto_radio, password_hash IS NOT NULL AS has_password
		 FROM rooms WHERE is_private = false ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []map[string]any
	for rows.Next() {
		var id, name string
		var autoRadio, hasPassword bool
		if err := rows.Scan(&id, &name, &autoRadio, &hasPassword); err != nil {
			continue
		}
		rooms = append(rooms, map[string]any{
			"id":           id,
			"name":         name,
			"auto_radio":   autoRadio,
			"user_count":   0,
			"track_count":  0,
			"is_playing":   false,
			"has_password": hasPassword,
		})
	}
	return rooms, nil
}

func (r *RoomRepo) RecordVisit(userID, roomID string) error {
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO user_room_visits (user_id, room_id, visited_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (user_id, room_id) DO UPDATE SET visited_at = NOW()`,
		userID, roomID,
	)
	return err
}

func (r *RoomRepo) RecentRooms(userID string, limit int) ([]map[string]any, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT r.id, r.name, v.visited_at
		 FROM user_room_visits v
		 JOIN rooms r ON r.id = v.room_id
		 WHERE v.user_id = $1 AND r.is_private = false
		 ORDER BY v.visited_at DESC
		 LIMIT $2`, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []map[string]any
	for rows.Next() {
		var id, name string
		var visitedAt interface{}
		if err := rows.Scan(&id, &name, &visitedAt); err != nil {
			continue
		}
		rooms = append(rooms, map[string]any{"id": id, "name": name})
	}
	return rooms, nil
}

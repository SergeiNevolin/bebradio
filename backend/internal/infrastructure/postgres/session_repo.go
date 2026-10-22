package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

func (r *SessionRepo) SaveTracks(ref entity.SessionRef, tracks []*entity.Track) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM queue_items WHERE session_kind = $1 AND session_id = $2`,
		string(ref.Kind), ref.ID,
	); err != nil {
		return fmt.Errorf("delete old queue: %w", err)
	}
	for i, track := range tracks {
		raw, err := json.Marshal(track)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO queue_items (session_kind, session_id, position, track)
			 VALUES ($1, $2, $3, $4::jsonb)`,
			string(ref.Kind), ref.ID, i, string(raw),
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *SessionRepo) LoadTracks(ref entity.SessionRef) ([]*entity.Track, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT track FROM queue_items
		 WHERE session_kind = $1 AND session_id = $2 ORDER BY position`,
		string(ref.Kind), ref.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make([]*entity.Track, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		t := &entity.Track{}
		if err := json.Unmarshal(raw, t); err != nil {
			continue
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func (r *SessionRepo) SaveMessage(ref entity.SessionRef, msg *entity.ChatMessage) error {
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO chat_messages (id, room_id, session_kind, session_id, user_id, username, text, created_at)
		 VALUES ($1, NULL, $2, $3, $4, $5, $6, $7)`,
		msg.ID, string(ref.Kind), ref.ID, msg.UserID, msg.Username, msg.Text, msg.CreatedAt,
	)
	return err
}

func (r *SessionRepo) LoadMessages(ref entity.SessionRef) ([]*entity.ChatMessage, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT id, user_id, username, text, created_at
		 FROM chat_messages WHERE session_kind = $1 AND session_id = $2
		 ORDER BY created_at DESC LIMIT 200`,
		string(ref.Kind), ref.ID,
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
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

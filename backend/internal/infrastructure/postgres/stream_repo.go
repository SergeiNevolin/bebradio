package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StreamRepo struct {
	pool *pgxpool.Pool
}

func NewStreamRepo(pool *pgxpool.Pool) *StreamRepo {
	return &StreamRepo{pool: pool}
}

func (r *StreamRepo) Save(stream *entity.Stream) error {
	ctx := context.Background()
	features, err := json.Marshal(stream.Features)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO streams (id, name, owner_id, features, created_at)
		 VALUES ($1, $2, $3, $4::jsonb, $5)
		 ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			owner_id = EXCLUDED.owner_id,
			features = EXCLUDED.features`,
		stream.ID, stream.Name, stream.OwnerID, string(features), stream.CreatedAt,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO sessions (kind, id, created_at) VALUES ('stream', $1, $2)
		 ON CONFLICT DO NOTHING`, stream.ID, stream.CreatedAt,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *StreamRepo) FindByID(id string) (*entity.Stream, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT id, name, owner_id, features, created_at FROM streams WHERE id = $1`, id,
	)
	st := entity.NewStream("", "", "")
	var raw []byte
	if err := row.Scan(&st.ID, &st.Name, &st.OwnerID, &raw, &st.CreatedAt); err != nil {
		return nil, fmt.Errorf("stream not found: %w", err)
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &st.Features); err != nil {
			return nil, err
		}
	}
	return st, nil
}

func (r *StreamRepo) Delete(id string) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM chat_messages WHERE session_kind = 'stream' AND session_id = $1`, id); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM queue_items WHERE session_kind = 'stream' AND session_id = $1`, id); err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE kind = 'stream' AND id = $1`, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM streams WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete stream: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *StreamRepo) ListPublic() ([]map[string]any, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT id, name, owner_id, features, created_at
		 FROM streams ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []map[string]any
	for rows.Next() {
		st := &entity.Stream{}
		var raw []byte
		if err := rows.Scan(&st.ID, &st.Name, &st.OwnerID, &raw, &st.CreatedAt); err != nil {
			continue
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &st.Features)
		}
		streams = append(streams, map[string]any{
			"id":         st.ID,
			"name":       st.Name,
			"owner_id":   st.OwnerID,
			"created_at": st.CreatedAt,
			"features":   st.Features,
		})
	}
	return streams, nil
}

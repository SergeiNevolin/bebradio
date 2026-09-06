package postgres

import (
	"context"
	"fmt"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MashupRepo struct {
	pool *pgxpool.Pool
}

func NewMashupRepo(pool *pgxpool.Pool) *MashupRepo {
	return &MashupRepo{pool: pool}
}

const mashupColumns = `id, owner_id, title, artist, media_id, duration, size_bytes, status, error, has_cover, plays, created_at`

func scanMashup(row scannable) (*entity.Mashup, error) {
	m := &entity.Mashup{}
	err := row.Scan(&m.ID, &m.OwnerID, &m.Title, &m.Artist, &m.MediaID, &m.Duration,
		&m.SizeBytes, &m.Status, &m.Error, &m.HasCover, &m.Plays, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("mashup not found: %w", err)
	}
	return m, nil
}

func collectMashups(rows pgx.Rows) ([]*entity.Mashup, error) {
	defer rows.Close()
	out := make([]*entity.Mashup, 0)
	for rows.Next() {
		if m, err := scanMashup(rows); err == nil {
			out = append(out, m)
		}
	}
	return out, rows.Err()
}

func (r *MashupRepo) Create(m *entity.Mashup) error {
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO mashups (id, owner_id, title, artist, media_id, duration, size_bytes, status, error, has_cover, plays, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		m.ID, m.OwnerID, m.Title, m.Artist, m.MediaID, m.Duration, m.SizeBytes,
		m.Status, m.Error, m.HasCover, m.Plays, m.CreatedAt,
	)
	return err
}

func (r *MashupRepo) FindByID(id string) (*entity.Mashup, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT `+mashupColumns+` FROM mashups WHERE id = $1`, id)
	return scanMashup(row)
}

func (r *MashupRepo) Delete(id string) error {
	_, err := r.pool.Exec(context.Background(), `DELETE FROM mashups WHERE id = $1`, id)
	return err
}

func (r *MashupRepo) UpdateStatus(id, status, errMsg string, duration int, sizeBytes int64, hasCover bool) error {
	_, err := r.pool.Exec(context.Background(),
		`UPDATE mashups SET status = $1, error = $2, duration = $3, size_bytes = $4, has_cover = $5 WHERE id = $6`,
		status, errMsg, duration, sizeBytes, hasCover, id,
	)
	return err
}

func (r *MashupRepo) List(query string, limit, offset int) ([]*entity.Mashup, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	ctx := context.Background()
	if query == "" {
		rows, err := r.pool.Query(ctx,
			`SELECT `+mashupColumns+` FROM mashups ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset)
		if err != nil {
			return nil, err
		}
		return collectMashups(rows)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+mashupColumns+` FROM mashups WHERE title ILIKE $1 OR artist ILIKE $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		"%"+query+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	return collectMashups(rows)
}

func (r *MashupRepo) ListByOwner(ownerID string) ([]*entity.Mashup, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT `+mashupColumns+` FROM mashups WHERE owner_id = $1 ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	return collectMashups(rows)
}

func (r *MashupRepo) CountByOwner(ownerID string) (int, error) {
	var count int
	err := r.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM mashups WHERE owner_id = $1`, ownerID).Scan(&count)
	return count, err
}

func (r *MashupRepo) ListProcessing() ([]*entity.Mashup, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT `+mashupColumns+` FROM mashups WHERE status = 'processing' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	return collectMashups(rows)
}

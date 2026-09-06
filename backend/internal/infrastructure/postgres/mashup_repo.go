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

// mashupDetailSelect joins the owner's username and derives the viewer's liked
// flag. $1 is always the viewer id ("" for anonymous -> liked is always false).
const mashupDetailSelect = `SELECT m.id, m.owner_id, u.username, m.title, m.artist, m.media_id,
	m.duration, m.size_bytes, m.status, m.error, m.has_cover, m.plays, m.likes, m.created_at,
	m.cover_updated_at,
	($1 <> '' AND EXISTS (SELECT 1 FROM mashup_likes l WHERE l.mashup_id = m.id AND l.user_id = $1)) AS liked
	FROM mashups m JOIN users u ON u.id = m.owner_id`

func scanMashup(row scannable) (*entity.Mashup, error) {
	m := &entity.Mashup{}
	err := row.Scan(&m.ID, &m.OwnerID, &m.Title, &m.Artist, &m.MediaID, &m.Duration,
		&m.SizeBytes, &m.Status, &m.Error, &m.HasCover, &m.Plays, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("mashup not found: %w", err)
	}
	return m, nil
}

func scanMashupDetail(row scannable) (*entity.Mashup, error) {
	m := &entity.Mashup{}
	err := row.Scan(&m.ID, &m.OwnerID, &m.OwnerName, &m.Title, &m.Artist, &m.MediaID,
		&m.Duration, &m.SizeBytes, &m.Status, &m.Error, &m.HasCover, &m.Plays, &m.Likes,
		&m.CreatedAt, &m.CoverUpdatedAt, &m.Liked)
	if err != nil {
		return nil, fmt.Errorf("mashup not found: %w", err)
	}
	return m, nil
}

func collectMashups(rows pgx.Rows) ([]*entity.Mashup, error) {
	defer rows.Close()
	out := make([]*entity.Mashup, 0)
	for rows.Next() {
		if m, err := scanMashupDetail(rows); err == nil {
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

func (r *MashupRepo) Detail(id, viewerID string) (*entity.Mashup, error) {
	row := r.pool.QueryRow(context.Background(), mashupDetailSelect+` WHERE m.id = $2`, viewerID, id)
	return scanMashupDetail(row)
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

func (r *MashupRepo) SetCoverUploaded(id string) error {
	_, err := r.pool.Exec(context.Background(),
		`UPDATE mashups SET has_cover = TRUE, cover_updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *MashupRepo) List(query, sort string, limit, offset int, viewerID string) ([]*entity.Mashup, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	order := ` ORDER BY m.created_at DESC`
	if sort == "top" {
		order = ` ORDER BY m.likes DESC, m.created_at DESC`
	}
	ctx := context.Background()
	if query == "" {
		rows, err := r.pool.Query(ctx,
			mashupDetailSelect+order+` LIMIT $2 OFFSET $3`,
			viewerID, limit, offset)
		if err != nil {
			return nil, err
		}
		return collectMashups(rows)
	}
	rows, err := r.pool.Query(ctx,
		mashupDetailSelect+` WHERE (m.title ILIKE $2 OR m.artist ILIKE $2)`+order+` LIMIT $3 OFFSET $4`,
		viewerID, "%"+query+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	return collectMashups(rows)
}

func (r *MashupRepo) ListByOwner(ownerID string) ([]*entity.Mashup, error) {
	rows, err := r.pool.Query(context.Background(),
		mashupDetailSelect+` WHERE m.owner_id = $1 ORDER BY m.created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	return collectMashups(rows)
}

func (r *MashupRepo) ListLikedByUser(userID string, limit, offset int) ([]*entity.Mashup, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(context.Background(),
		mashupDetailSelect+` JOIN mashup_likes ml ON ml.mashup_id = m.id AND ml.user_id = $1
		 ORDER BY ml.created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
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
	defer rows.Close()
	out := make([]*entity.Mashup, 0)
	for rows.Next() {
		if m, err := scanMashup(rows); err == nil {
			out = append(out, m)
		}
	}
	return out, rows.Err()
}

// recountLikes recomputes the denormalised counter from the source-of-truth
// table, which keeps it correct under concurrent likes and repeated calls.
func (r *MashupRepo) recountLikes(ctx context.Context, mashupID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`UPDATE mashups SET likes = (SELECT COUNT(*) FROM mashup_likes WHERE mashup_id = $1)
		 WHERE id = $1 RETURNING likes`, mashupID).Scan(&n)
	return n, err
}

func (r *MashupRepo) Like(mashupID, userID string) (int, error) {
	ctx := context.Background()
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO mashup_likes (mashup_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		mashupID, userID); err != nil {
		return 0, err
	}
	return r.recountLikes(ctx, mashupID)
}

func (r *MashupRepo) Unlike(mashupID, userID string) (int, error) {
	ctx := context.Background()
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM mashup_likes WHERE mashup_id = $1 AND user_id = $2`,
		mashupID, userID); err != nil {
		return 0, err
	}
	return r.recountLikes(ctx, mashupID)
}

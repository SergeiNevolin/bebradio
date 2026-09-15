package postgres

import (
	"context"
	"fmt"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TrackRepo persists the library slice of the tracks table: user uploads
// (room_id IS NULL). Room queue rows are owned by RoomRepo.
type TrackRepo struct {
	pool *pgxpool.Pool
}

func NewTrackRepo(pool *pgxpool.Pool) *TrackRepo {
	return &TrackRepo{pool: pool}
}

const trackColumns = `id, source, owner_id, title, artist, media_id, duration,
	size_bytes, status, error, has_cover, plays, added_at`

// trackDetailSelect joins the owner's username and derives the viewer's liked
// flag. $1 is always the viewer id ("" for anonymous -> liked is always false).
const trackDetailSelect = `SELECT t.id, t.source, t.owner_id, COALESCE(u.username, ''), t.title, t.artist, t.media_id,
	t.duration, t.size_bytes, t.status, t.error, t.has_cover, t.plays, t.likes, t.added_at,
	t.cover_updated_at,
	($1 <> '' AND EXISTS (SELECT 1 FROM track_likes l WHERE l.track_id = t.id AND l.user_id = $1)) AS liked
	FROM tracks t LEFT JOIN users u ON u.id = t.owner_id`

const libraryFilter = `t.room_id IS NULL AND t.source = 'upload'`

func scanTrack(row scannable) (*entity.Track, error) {
	t := &entity.Track{}
	err := row.Scan(&t.ID, &t.Source, &t.OwnerID, &t.Title, &t.Artist, &t.MediaID, &t.Duration,
		&t.SizeBytes, &t.Status, &t.Error, &t.HasCover, &t.Plays, &t.AddedAt)
	if err != nil {
		return nil, fmt.Errorf("track not found: %w", err)
	}
	return t, nil
}

func scanTrackDetail(row scannable) (*entity.Track, error) {
	t := &entity.Track{}
	err := row.Scan(&t.ID, &t.Source, &t.OwnerID, &t.OwnerName, &t.Title, &t.Artist, &t.MediaID,
		&t.Duration, &t.SizeBytes, &t.Status, &t.Error, &t.HasCover, &t.Plays, &t.Likes,
		&t.AddedAt, &t.CoverUpdatedAt, &t.Liked)
	if err != nil {
		return nil, fmt.Errorf("track not found: %w", err)
	}
	return t, nil
}

func collectTracks(rows pgx.Rows) ([]*entity.Track, error) {
	defer rows.Close()
	out := make([]*entity.Track, 0)
	for rows.Next() {
		if t, err := scanTrackDetail(rows); err == nil {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

func (r *TrackRepo) Create(t *entity.Track) error {
	_, err := r.pool.Exec(context.Background(),
		`INSERT INTO tracks (id, room_id, source, owner_id, title, artist, media_id, duration,
			size_bytes, status, error, has_cover, plays, added_at)
		 VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		t.ID, t.Source, t.OwnerID, t.Title, t.Artist, t.MediaID, t.Duration,
		t.SizeBytes, t.Status, t.Error, t.HasCover, t.Plays, t.AddedAt,
	)
	return err
}

// NOTE: one id may cover the library row plus its queue snapshots (they
// share the file). FindByID/Detail return an arbitrary matching row —
// equivalent for streaming, since all of them point at the same media.
func (r *TrackRepo) FindByID(id string) (*entity.Track, error) {
	row := r.pool.QueryRow(context.Background(),
		`SELECT `+trackColumns+` FROM tracks WHERE id = $1`, id)
	return scanTrack(row)
}

func (r *TrackRepo) Detail(id, viewerID string) (*entity.Track, error) {
	row := r.pool.QueryRow(context.Background(), trackDetailSelect+` WHERE t.id = $2`, viewerID, id)
	return scanTrackDetail(row)
}

func (r *TrackRepo) Delete(id string) error {
	ctx := context.Background()
	// track_likes has no FK since queue snapshots share ids (migration 005).
	if _, err := r.pool.Exec(ctx, `DELETE FROM track_likes WHERE track_id = $1`, id); err != nil {
		return err
	}
	// One id may cover the library row plus its queue snapshots; all of them
	// point at the same (now deleted) file, so they go together.
	_, err := r.pool.Exec(ctx, `DELETE FROM tracks WHERE id = $1`, id)
	return err
}

func (r *TrackRepo) UpdateStatus(id, status, errMsg string, duration int, sizeBytes int64, hasCover bool) error {
	_, err := r.pool.Exec(context.Background(),
		`UPDATE tracks SET status = $1, error = $2, duration = $3, size_bytes = $4, has_cover = $5 WHERE id = $6`,
		status, errMsg, duration, sizeBytes, hasCover, id,
	)
	return err
}

func (r *TrackRepo) SetCoverUploaded(id string) error {
	_, err := r.pool.Exec(context.Background(),
		`UPDATE tracks SET has_cover = TRUE, cover_updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *TrackRepo) List(query, sort string, limit, offset int, viewerID string) ([]*entity.Track, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	order := ` ORDER BY t.added_at DESC`
	if sort == "top" {
		order = ` ORDER BY t.likes DESC, t.added_at DESC`
	}
	ctx := context.Background()
	if query == "" {
		rows, err := r.pool.Query(ctx,
			trackDetailSelect+` WHERE `+libraryFilter+order+` LIMIT $2 OFFSET $3`,
			viewerID, limit, offset)
		if err != nil {
			return nil, err
		}
		return collectTracks(rows)
	}
	rows, err := r.pool.Query(ctx,
		trackDetailSelect+` WHERE `+libraryFilter+` AND (t.title ILIKE $2 OR t.artist ILIKE $2)`+order+` LIMIT $3 OFFSET $4`,
		viewerID, "%"+query+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	return collectTracks(rows)
}

func (r *TrackRepo) ListByOwner(ownerID string) ([]*entity.Track, error) {
	rows, err := r.pool.Query(context.Background(),
		trackDetailSelect+` WHERE `+libraryFilter+` AND t.owner_id = $1 ORDER BY t.added_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	return collectTracks(rows)
}

func (r *TrackRepo) ListLikedByUser(userID string, limit, offset int) ([]*entity.Track, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(context.Background(),
		trackDetailSelect+` JOIN track_likes tl ON tl.track_id = t.id AND tl.user_id = $1
		 WHERE `+libraryFilter+` ORDER BY tl.created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return collectTracks(rows)
}

func (r *TrackRepo) CountByOwner(ownerID string) (int, error) {
	var count int
	err := r.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM tracks WHERE `+libraryFilter+` AND owner_id = $1`, ownerID).Scan(&count)
	return count, err
}

func (r *TrackRepo) ListProcessing() ([]*entity.Track, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT `+trackColumns+` FROM tracks WHERE `+libraryFilter+` AND status = 'processing' ORDER BY added_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*entity.Track, 0)
	for rows.Next() {
		if t, err := scanTrack(rows); err == nil {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

// recountLikes recomputes the denormalised counter from the source-of-truth
// table, which keeps it correct under concurrent likes and repeated calls.
func (r *TrackRepo) recountLikes(ctx context.Context, trackID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`UPDATE tracks SET likes = (SELECT COUNT(*) FROM track_likes WHERE track_id = $1)
		 WHERE id = $1 RETURNING likes`, trackID).Scan(&n)
	return n, err
}

func (r *TrackRepo) Like(trackID, userID string) (int, error) {
	ctx := context.Background()
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO track_likes (track_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		trackID, userID); err != nil {
		return 0, err
	}
	return r.recountLikes(ctx, trackID)
}

func (r *TrackRepo) Unlike(trackID, userID string) (int, error) {
	ctx := context.Background()
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM track_likes WHERE track_id = $1 AND user_id = $2`,
		trackID, userID); err != nil {
		return 0, err
	}
	return r.recountLikes(ctx, trackID)
}

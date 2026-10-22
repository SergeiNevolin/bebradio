package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
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
	return r.ListFiltered(repository.TrackFilter{Query: query}, sort, limit, offset, viewerID)
}

func orderClause(sort string) string {
	if sort == "top" {
		return ` ORDER BY t.likes DESC, t.added_at DESC`
	}
	return ` ORDER BY t.added_at DESC`
}

// filterClauses builds the WHERE tail (after libraryFilter) and args for a
// TrackFilter. base is the next placeholder index ($1 is always viewerID).
// Query matches title/artist/owner username; Title/Artist/Owner narrow further.
func filterClauses(f repository.TrackFilter, base int) (string, []any) {
	var sb strings.Builder
	var args []any
	idx := base
	add := func(cond string, v any) {
		sb.WriteString(fmt.Sprintf(" AND "+cond, idx))
		args = append(args, v)
		idx++
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + q + "%"
		sb.WriteString(fmt.Sprintf(` AND (t.title ILIKE $%d OR t.artist ILIKE $%d OR COALESCE(u.username,'') ILIKE $%d)`, idx, idx+1, idx+2))
		args = append(args, like, like, like)
		idx += 3
	}
	if t := strings.TrimSpace(f.Title); t != "" {
		add(`t.title ILIKE $%d`, "%"+t+"%")
	}
	if a := strings.TrimSpace(f.Artist); a != "" {
		add(`t.artist ILIKE $%d`, "%"+a+"%")
	}
	if o := strings.TrimSpace(f.Owner); o != "" {
		like := "%" + o + "%"
		sb.WriteString(fmt.Sprintf(` AND (COALESCE(u.username,'') ILIKE $%d OR t.owner_id ILIKE $%d)`, idx, idx+1))
		args = append(args, like, like)
		idx += 2
	}
	switch f.Status {
	case entity.TrackStatusReady, entity.TrackStatusProcessing, entity.TrackStatusFailed:
		add(`t.status = $%d`, f.Status)
	}
	return sb.String(), args
}

func (r *TrackRepo) ListFiltered(f repository.TrackFilter, sort string, limit, offset int, viewerID string) ([]*entity.Track, error) {
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	where, args := filterClauses(f, 2)
	queryArgs := append([]any{viewerID}, args...)
	// LIMIT/OFFSET placeholders follow the filter args.
	limPos := len(queryArgs) + 1
	offPos := len(queryArgs) + 2
	queryArgs = append(queryArgs, limit, offset)
	rows, err := r.pool.Query(context.Background(),
		trackDetailSelect+` WHERE `+libraryFilter+where+orderClause(sort)+fmt.Sprintf(` LIMIT $%d OFFSET $%d`, limPos, offPos),
		queryArgs...)
	if err != nil {
		return nil, err
	}
	return collectTracks(rows)
}

// ListRandomReady feeds the 24/7 stream: only ready rows, random order,
// excluding already queued / recently played track IDs or media IDs.
func (r *TrackRepo) ListRandomReady(f repository.TrackFilter, limit int, excludeIDs []string) ([]*entity.Track, error) {
	if limit <= 0 {
		limit = 3
	}
	f.Status = entity.TrackStatusReady
	// $1 is viewerID ("" for stream picks), filter args start at $2.
	where, args := filterClauses(f, 2)
	queryArgs := append([]any{""}, args...)
	if len(excludeIDs) > 0 {
		placeholders := make([]string, len(excludeIDs))
		for i, id := range excludeIDs {
			placeholders[i] = fmt.Sprintf("$%d", len(queryArgs)+1)
			queryArgs = append(queryArgs, id)
		}
		excluded := strings.Join(placeholders, ",")
		where += ` AND t.id NOT IN (` + excluded + `) AND COALESCE(t.media_id, '') NOT IN (` + excluded + `)`
	}
	limPos := len(queryArgs) + 1
	queryArgs = append(queryArgs, limit)
	// queryArgs[0] is viewerID "" — liked flag always false for stream picks.
	rows, err := r.pool.Query(context.Background(),
		trackDetailSelect+` WHERE `+libraryFilter+where+` ORDER BY RANDOM()`+fmt.Sprintf(` LIMIT $%d`, limPos),
		queryArgs...)
	if err != nil {
		return nil, err
	}
	return collectTracks(rows)
}

func (r *TrackRepo) ListByOwner(ownerID string, limit, offset int) ([]*entity.Track, error) {
	rows, err := r.pool.Query(context.Background(),
		trackDetailSelect+` WHERE `+libraryFilter+` AND t.owner_id = $1 ORDER BY t.added_at DESC LIMIT $2 OFFSET $3`, ownerID, limit, offset)
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
		`SELECT COUNT(*) FROM tracks t WHERE `+libraryFilter+` AND t.owner_id = $1`, ownerID).Scan(&count)
	return count, err
}

func (r *TrackRepo) UpdateMetadata(id, title, artist string) error {
	_, err := r.pool.Exec(context.Background(),
		`UPDATE tracks SET title = $1, artist = $2 WHERE id = $3`,
		title, artist, id,
	)
	return err
}

func (r *TrackRepo) ListProcessing() ([]*entity.Track, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT `+trackColumns+` FROM tracks t WHERE `+libraryFilter+` AND t.status = 'processing' ORDER BY t.added_at`)
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

package repository

import (
	"io"

	"github.com/bebradio/backend-go/internal/domain/entity"
)

// TrackFilter narrows the library slice of the tracks table.
//
//	Query  — free text over title + artist + owner username (legacy ?q=).
//	Title  — substring of t.title.
//	Artist — substring of t.artist.
//	Owner  — substring of owner username OR owner_id (uploader filter).
//	Status — ready|processing|failed, empty = any.
type TrackFilter struct {
	Query  string
	Title  string
	Artist string
	Owner  string
	Status string
}

func (f TrackFilter) Empty() bool {
	return f.Query == "" && f.Title == "" && f.Artist == "" && f.Owner == "" && f.Status == ""
}

// TrackRepository persists the library slice of the tracks table:
// user uploads (room_id IS NULL). Room queue rows stay with RoomRepository.
type TrackRepository interface {
	Create(t *entity.Track) error
	FindByID(id string) (*entity.Track, error)
	// Detail is FindByID enriched with owner_name and the viewer's liked flag
	// (viewerID "" = anonymous, liked always false).
	Detail(id, viewerID string) (*entity.Track, error)
	Delete(id string) error
	UpdateStatus(id, status, errMsg string, duration int, sizeBytes int64, hasCover bool) error
	SetCoverUploaded(id string) error
	// List returns library tracks whose title/artist match query
	// (empty query = all), ordered by sort ("recent" = newest first,
	// "top" = most liked first), paginated. viewerID drives the per-row
	// liked flag.
	List(query, sort string, limit, offset int, viewerID string) ([]*entity.Track, error)
	// ListFiltered is List with field filters (title/artist/owner/status).
	// Empty filter behaves like List.
	ListFiltered(filter TrackFilter, sort string, limit, offset int, viewerID string) ([]*entity.Track, error)
	// ListRandomReady returns up to limit ready library tracks matching filter,
	// in random order, excluding excludeIDs. Used by the 24/7 stream refill.
	ListRandomReady(filter TrackFilter, limit int, excludeIDs []string) ([]*entity.Track, error)
	ListByOwner(ownerID string, limit, offset int) ([]*entity.Track, error)
	ListLikedByUser(userID string, limit, offset int) ([]*entity.Track, error)
	CountByOwner(ownerID string) (int, error)
	ListProcessing() ([]*entity.Track, error)
	UpdateMetadata(id, title, artist string) error
	// Like/Unlike are idempotent and return the fresh like count.
	Like(trackID, userID string) (int, error)
	Unlike(trackID, userID string) (int, error)
}

type MediaClient interface {
	Search(query string, limit int) ([]map[string]any, error)
	Resolve(url string) (map[string]any, error)
	Download(sourceURL, mediaID string) (map[string]any, error)
	Ensure(items []map[string]any) ([]string, error)
	Related(sourceURL string, limit int) ([]string, error)
	MediaCaptions(mediaID, lang string) (map[string]any, error)
	Content(mediaID, rangeHeader string) (int64, string, []byte, error)
	UpdateReferences(mediaIDs []string) error

	// Uploads: persistent user tracks, transcoded with ffmpeg on music-service.
	// Identity arrives with the data: returns the minted id and media_id.
	UploadTrack(filename string, body io.Reader) (map[string]any, error)
	UploadTrackCover(mediaID, filename string, body io.Reader) error
	TrackUploadStatus(mediaID string) (map[string]any, error)
	DeleteTrack(mediaID string) error
}

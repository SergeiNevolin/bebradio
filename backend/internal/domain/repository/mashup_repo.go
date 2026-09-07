package repository

import "github.com/bebradio/backend-go/internal/domain/entity"

type MashupRepository interface {
	Create(m *entity.Mashup) error
	FindByID(id string) (*entity.Mashup, error)
	// Detail is FindByID enriched with owner_name and the viewer's liked flag
	// (viewerID "" = anonymous, liked always false).
	Detail(id, viewerID string) (*entity.Mashup, error)
	Delete(id string) error
	UpdateStatus(id, status, errMsg string, duration int, sizeBytes int64, hasCover bool) error
	SetCoverUploaded(id string) error
	// List returns mashups whose title/artist match query (empty query = all),
	// ordered by sort ("recent" = newest first, "top" = most liked first),
	// paginated. viewerID drives the per-row liked flag.
	List(query, sort string, limit, offset int, viewerID string) ([]*entity.Mashup, error)
	ListByOwner(ownerID string) ([]*entity.Mashup, error)
	ListLikedByUser(userID string, limit, offset int) ([]*entity.Mashup, error)
	CountByOwner(ownerID string) (int, error)
	ListProcessing() ([]*entity.Mashup, error)
	// Like/Unlike are idempotent and return the fresh like count.
	Like(mashupID, userID string) (int, error)
	Unlike(mashupID, userID string) (int, error)
}

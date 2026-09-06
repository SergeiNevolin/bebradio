package repository

import "github.com/bebradio/backend-go/internal/domain/entity"

type MashupRepository interface {
	Create(m *entity.Mashup) error
	FindByID(id string) (*entity.Mashup, error)
	Delete(id string) error
	UpdateStatus(id, status, errMsg string, duration int, sizeBytes int64, hasCover bool) error
	// List returns mashups whose title/artist match query (empty query = all),
	// newest first, paginated.
	List(query string, limit, offset int) ([]*entity.Mashup, error)
	ListByOwner(ownerID string) ([]*entity.Mashup, error)
	CountByOwner(ownerID string) (int, error)
	ListProcessing() ([]*entity.Mashup, error)
}

package repository

import "github.com/bebradio/backend-go/internal/domain/entity"

type RoomRepository interface {
	Save(room *entity.Room) error
	FindByID(id string) (*entity.Room, error)
	Delete(id string) error
	ListPublic() ([]map[string]any, error)
	RecordVisit(userID, roomID string) error
	RecentRooms(userID string, limit int) ([]map[string]any, error)
}

// SessionRepository persists the queue and chat for any session kind. Room
// metadata and stream metadata live in their respective repositories.
type SessionRepository interface {
	SaveTracks(ref entity.SessionRef, tracks []*entity.Track) error
	LoadTracks(ref entity.SessionRef) ([]*entity.Track, error)
	SaveMessage(ref entity.SessionRef, msg *entity.ChatMessage) error
	LoadMessages(ref entity.SessionRef) ([]*entity.ChatMessage, error)
}

// StreamRepository persists stream metadata and its typed feature set.
type StreamRepository interface {
	Save(stream *entity.Stream) error
	FindByID(id string) (*entity.Stream, error)
	Delete(id string) error
	ListPublic() ([]map[string]any, error)
}

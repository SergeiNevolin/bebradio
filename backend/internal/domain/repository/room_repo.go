package repository

import "github.com/bebradio/backend-go/internal/domain/entity"

type RoomRepository interface {
	Save(room *entity.Room) error
	FindByID(id string) (*entity.Room, error)
	Delete(id string) error
	ListPublic() ([]map[string]any, error)
	SaveTracks(room *entity.Room) error
	SaveTracksFromSlice(roomID string, tracks []*entity.Track) error
	LoadTracks(roomID string) ([]*entity.Track, error)
	SaveMessage(roomID string, msg *entity.ChatMessage) error
	LoadMessages(roomID string) ([]*entity.ChatMessage, error)
	RecordVisit(userID, roomID string) error
	RecentRooms(userID string, limit int) ([]map[string]any, error)
}

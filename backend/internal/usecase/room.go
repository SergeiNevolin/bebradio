package usecase

import (
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/pkg/id"
)

type RoomUsecase struct {
	roomRepo    repository.RoomRepository
	userRepo    repository.UserRepository
	mediaClient repository.MediaClient
	auth        AuthBridge
	log         *slog.Logger

	rooms   map[string]*entity.Room
	roomsMu sync.RWMutex
}

func NewRoomUsecase(roomRepo repository.RoomRepository, userRepo repository.UserRepository, mediaClient repository.MediaClient, auth AuthBridge, log *slog.Logger) *RoomUsecase {
	return &RoomUsecase{
		roomRepo:    roomRepo,
		userRepo:    userRepo,
		mediaClient: mediaClient,
		auth:        auth,
		log:         log,
		rooms:       make(map[string]*entity.Room),
	}
}

func (uc *RoomUsecase) GetRoom(roomID string) *entity.Room {
	uc.roomsMu.RLock()
	defer uc.roomsMu.RUnlock()
	return uc.rooms[roomID]
}

func (uc *RoomUsecase) StoreRoom(room *entity.Room) {
	uc.roomsMu.Lock()
	defer uc.roomsMu.Unlock()
	uc.rooms[room.ID] = room
}

func (uc *RoomUsecase) RemoveRoom(roomID string) {
	uc.roomsMu.Lock()
	defer uc.roomsMu.Unlock()
	delete(uc.rooms, roomID)
}

func (uc *RoomUsecase) GetRooms() map[string]*entity.Room {
	return uc.rooms
}

func (uc *RoomUsecase) RoomsMu() *sync.RWMutex {
	return &uc.roomsMu
}

func (uc *RoomUsecase) GetOrLoadRoom(roomID string) (*entity.Room, error) {
	roomID = toUpper(roomID)
	uc.roomsMu.RLock()
	if rm, ok := uc.rooms[roomID]; ok {
		uc.roomsMu.RUnlock()
		return rm, nil
	}
	uc.roomsMu.RUnlock()

	rm, err := uc.roomRepo.FindByID(roomID)
	if err != nil {
		return nil, err
	}

	tracks, err := uc.roomRepo.LoadTracks(roomID)
	if err != nil {
		uc.log.Warn("failed to load tracks", "room_id", roomID, "error", err)
	} else if tracks != nil {
		rm.Queue = tracks
	}
	messages, err := uc.roomRepo.LoadMessages(roomID)
	if err != nil {
		uc.log.Warn("failed to load messages", "room_id", roomID, "error", err)
	} else if messages != nil {
		rm.Messages = messages
	}
	votes, err := uc.roomRepo.LoadVotes(roomID)
	if err != nil {
		uc.log.Warn("failed to load votes", "room_id", roomID, "error", err)
	} else if votes != nil {
		rm.Votes = votes
	}

	uc.roomsMu.Lock()
	uc.rooms[roomID] = rm
	uc.roomsMu.Unlock()
	return rm, nil
}

func (uc *RoomUsecase) CreateRoom(name, ownerID, password string) (*entity.Room, string, error) {
	rm := &entity.Room{
		ID:                id.New(6),
		Name:              name,
		OwnerID:           ownerID,
		Queue:             make([]*entity.Track, 0),
		Messages:          make([]*entity.ChatMessage, 0),
		Votes:             make([]*entity.TrackVote, 0),
		SkipVotes:         make(map[string]bool),
		Presence:          make(map[string]entity.PresenceInfo),
		Users:             make(map[string]string),
		RadioSeen:         make(map[string]bool),
		AllowAnonymousAdd: true,
		CreatedAt:         time.Now(),
	}
	if password != "" {
		hash, err := uc.auth.HashPassword(password)
		if err != nil {
			uc.log.Error("failed to hash room password", "error", err)
		} else {
			rm.PasswordHash = &hash
		}
	}

	uc.roomsMu.Lock()
	uc.rooms[rm.ID] = rm
	uc.roomsMu.Unlock()

	if err := uc.roomRepo.Save(rm); err != nil {
		return nil, "", err
	}

	access, err := uc.auth.CreateRoomToken(rm.ID)
	if err != nil {
		uc.log.Error("failed to create room token", "room_id", rm.ID, "error", err)
		return rm, "", nil
	}
	return rm, access, nil
}

func (uc *RoomUsecase) ListPublicRooms() ([]map[string]any, error) {
	dbRooms, err := uc.roomRepo.ListPublic()
	if err != nil {
		return nil, err
	}

	uc.roomsMu.RLock()
	defer uc.roomsMu.RUnlock()

	for _, r := range dbRooms {
		roomID, _ := r["id"].(string)
		if rm, ok := uc.rooms[roomID]; ok {
			rm.Mu.RLock()
			listeners := rm.Listeners()
			r["user_count"] = len(listeners)
			r["track_count"] = len(rm.Queue)
			r["is_playing"] = rm.IsPlaying
			r["has_password"] = rm.PasswordHash != nil
			rm.Mu.RUnlock()
		}
	}

	// Sort by user_count descending (popularity)
	sort.Slice(dbRooms, func(i, j int) bool {
		a, _ := dbRooms[i]["user_count"].(int)
		b, _ := dbRooms[j]["user_count"].(int)
		return a > b
	})

	return dbRooms, nil
}

func (uc *RoomUsecase) HasRoomAccess(rm *entity.Room, userID, access string) bool {
	if rm.PasswordHash == nil {
		return true
	}
	if userID != "" && userID == rm.OwnerID {
		return true
	}
	return uc.auth.VerifyRoomToken(access, rm.ID)
}

func (uc *RoomUsecase) JoinRoom(rm *entity.Room, password string) (string, error) {
	if rm.PasswordHash != nil {
		if !uc.auth.VerifyPassword(password, *rm.PasswordHash) {
			return "", ErrWrongPassword
		}
	}
	return uc.auth.CreateRoomToken(rm.ID)
}

func (uc *RoomUsecase) UpdateRoomSettings(rm *entity.Room, allowAnon, isPrivate, autoRadio *bool, password *string) error {
	if allowAnon != nil {
		rm.AllowAnonymousAdd = *allowAnon
	}
	if isPrivate != nil {
		rm.IsPrivate = *isPrivate
	}
	if autoRadio != nil {
		rm.AutoRadio = *autoRadio
	}
	if password != nil {
		if *password == "" {
			rm.PasswordHash = nil
		} else {
			hash, err := uc.auth.HashPassword(*password)
			if err != nil {
				return err
			}
			rm.PasswordHash = &hash
		}
	}
	return uc.roomRepo.Save(rm)
}

func (uc *RoomUsecase) DeleteRoom(rm *entity.Room) error {
	uc.RemoveRoom(rm.ID)
	return uc.roomRepo.Delete(rm.ID)
}

func (uc *RoomUsecase) SaveTracks(rm *entity.Room) error {
	return uc.roomRepo.SaveTracks(rm)
}

func (uc *RoomUsecase) SaveVotes(rm *entity.Room) error {
	return uc.roomRepo.SaveVotes(rm)
}

func (uc *RoomUsecase) CreateAccessToken(roomID string) (string, error) {
	return uc.auth.CreateRoomToken(roomID)
}

func (uc *RoomUsecase) RecordVisit(userID, roomID string) error {
	if userID == "" {
		return nil
	}
	return uc.roomRepo.RecordVisit(userID, roomID)
}

func (uc *RoomUsecase) RecentRooms(userID string, limit int) ([]map[string]any, error) {
	if userID == "" {
		return []map[string]any{}, nil
	}
	dbRooms, err := uc.roomRepo.RecentRooms(userID, limit)
	if err != nil {
		return nil, err
	}

	uc.roomsMu.RLock()
	defer uc.roomsMu.RUnlock()

	for _, r := range dbRooms {
		roomID, _ := r["id"].(string)
		if rm, ok := uc.rooms[roomID]; ok {
			rm.Mu.RLock()
			listeners := rm.Listeners()
			r["user_count"] = len(listeners)
			r["track_count"] = len(rm.Queue)
			r["is_playing"] = rm.IsPlaying
			r["has_password"] = rm.PasswordHash != nil
			rm.Mu.RUnlock()
		}
	}
	if dbRooms == nil {
		dbRooms = []map[string]any{}
	}
	return dbRooms, nil
}

var ErrWrongPassword = &BusinessError{Code: 403, Message: "Incorrect room password"}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		b[i] = c
	}
	return string(b)
}

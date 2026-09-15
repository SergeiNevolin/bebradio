package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/pkg/id"
	"github.com/redis/go-redis/v9"
)

type RoomUsecase struct {
	roomRepo    repository.RoomRepository
	userRepo    repository.UserRepository
	mediaClient repository.MediaClient
	auth        AuthBridge
	log         *slog.Logger
	rdb         *redis.Client
}

func NewRoomUsecase(roomRepo repository.RoomRepository, userRepo repository.UserRepository, mediaClient repository.MediaClient, auth AuthBridge, log *slog.Logger, rdb *redis.Client) *RoomUsecase {
	return &RoomUsecase{
		roomRepo:    roomRepo,
		userRepo:    userRepo,
		mediaClient: mediaClient,
		auth:        auth,
		log:         log,
		rdb:         rdb,
	}
}

func (uc *RoomUsecase) GetOrLoadRoom(ctx context.Context, roomID string) (*entity.Room, error) {
	roomID = toUpper(roomID)
	rm, err := uc.roomRepo.FindByID(roomID)
	if err != nil {
		return nil, err
	}

	// On first load, hydrate Redis from Postgres.
	tracks, _ := uc.roomRepo.LoadTracks(roomID)
	if len(tracks) > 0 {
		redisc.SetQueue(ctx, uc.rdb, roomID, tracks)
	}
	messages, _ := uc.roomRepo.LoadMessages(roomID)
	if len(messages) > 0 {
		for _, m := range messages {
			redisc.AppendMessage(ctx, uc.rdb, roomID, m)
		}
	}
	votes, _ := uc.roomRepo.LoadVotes(roomID)
	if len(votes) > 0 {
		// Convert from entity format to Redis format.
		voteMap := make(map[string]*redisc.VoteEntry)
		for _, v := range votes {
			ve, ok := voteMap[v.TrackID]
			if !ok {
				ve = &redisc.VoteEntry{}
				voteMap[v.TrackID] = ve
			}
			if v.Vote == 1 {
				ve.Likes++
			} else if v.Vote == -1 {
				ve.Disliked = append(ve.Disliked, v.UserID)
			}
		}
		for trackID, ve := range voteMap {
			redisc.SetVote(ctx, uc.rdb, roomID, trackID, ve)
		}
	}

	return rm, nil
}

func (uc *RoomUsecase) CreateRoom(ctx context.Context, name, ownerID, password string) (*entity.Room, string, error) {
	rm := entity.NewRoom(id.New(6), name, ownerID)

	if password != "" {
		hash, err := uc.auth.HashPassword(password)
		if err != nil {
			uc.log.Error("failed to hash room password", "error", err)
		} else {
			rm.PasswordHash = &hash
		}
	}

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

func (uc *RoomUsecase) ListPublicRooms(ctx context.Context) ([]map[string]any, error) {
	dbRooms, err := uc.roomRepo.ListPublic()
	if err != nil {
		return nil, err
	}

	for _, r := range dbRooms {
		roomID, _ := r["id"].(string)
		// Load presence count from Redis.
		count, _ := redisc.GetPresenceCount(ctx, uc.rdb, roomID)
		r["user_count"] = int(count)

		// Load queue length from Redis.
		tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
		r["track_count"] = len(tracks)

		// Load playback state from Redis.
		ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
		if ps != nil {
			r["is_playing"] = ps.IsPlaying
		}

		rm, err := uc.roomRepo.FindByID(roomID)
		if err == nil {
			r["has_password"] = rm.PasswordHash != nil
		}
	}

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

func (uc *RoomUsecase) UpdateRoomSettings(ctx context.Context, rm *entity.Room, allowAnon, isPrivate, autoRadio *bool, password *string) error {
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

func (uc *RoomUsecase) DeleteRoom(ctx context.Context, rm *entity.Room) error {
	redisc.DeleteRoom(ctx, uc.rdb, rm.ID)
	return uc.roomRepo.Delete(rm.ID)
}

func (uc *RoomUsecase) SaveTracks(ctx context.Context, rm *entity.Room) error {
	tracks, err := redisc.GetQueue(ctx, uc.rdb, rm.ID)
	if err != nil {
		return fmt.Errorf("get queue from redis: %w", err)
	}
	return uc.roomRepo.SaveTracksFromSlice(rm.ID, tracks)
}

func (uc *RoomUsecase) SaveVotes(ctx context.Context, rm *entity.Room) error {
	votes, err := redisc.GetVotes(ctx, uc.rdb, rm.ID)
	if err != nil {
		return fmt.Errorf("get votes from redis: %w", err)
	}
	// Convert Redis vote format to entity format for persistence.
	var entityVotes []*entity.TrackVote
	for trackID, ve := range votes {
		for _, uid := range ve.Disliked {
			entityVotes = append(entityVotes, &entity.TrackVote{
				UserID:  uid,
				TrackID: trackID,
				Vote:    -1,
			})
		}
		for i := 0; i < ve.Likes; i++ {
			entityVotes = append(entityVotes, &entity.TrackVote{
				TrackID: trackID,
				Vote:    1,
			})
		}
	}
	return uc.roomRepo.SaveVotesFromSlice(rm.ID, entityVotes)
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

func (uc *RoomUsecase) RecentRooms(ctx context.Context, userID string, limit int) ([]map[string]any, error) {
	if userID == "" {
		return []map[string]any{}, nil
	}
	dbRooms, err := uc.roomRepo.RecentRooms(userID, limit)
	if err != nil {
		return nil, err
	}

	for _, r := range dbRooms {
		roomID, _ := r["id"].(string)
		count, _ := redisc.GetPresenceCount(ctx, uc.rdb, roomID)
		r["user_count"] = int(count)

		tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
		r["track_count"] = len(tracks)

		ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
		if ps != nil {
			r["is_playing"] = ps.IsPlaying
		}

		rm, err := uc.roomRepo.FindByID(roomID)
		if err == nil {
			r["has_password"] = rm.PasswordHash != nil
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

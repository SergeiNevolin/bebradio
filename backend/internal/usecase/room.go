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
	sessionRepo repository.SessionRepository
	userRepo    repository.UserRepository
	mediaClient repository.MediaClient
	auth        AuthBridge
	log         *slog.Logger
	rdb         *redis.Client
}

func NewRoomUsecase(roomRepo repository.RoomRepository, userRepo repository.UserRepository, mediaClient repository.MediaClient, auth AuthBridge, log *slog.Logger, rdb *redis.Client, sessionRepos ...repository.SessionRepository) *RoomUsecase {
	var sessionRepo repository.SessionRepository
	if len(sessionRepos) > 0 {
		sessionRepo = sessionRepos[0]
	} else if sr, ok := roomRepo.(repository.SessionRepository); ok {
		sessionRepo = sr
	}
	return &RoomUsecase{
		roomRepo:    roomRepo,
		sessionRepo: sessionRepo,
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

	ref := rm.Ref()
	acquired, _ := uc.rdb.SetNX(ctx, redisc.SessionKey(ref, "hydrated"), "1", 0).Result()
	if acquired && uc.sessionRepo != nil {
		tracks, _ := uc.sessionRepo.LoadTracks(ref)
		if len(tracks) > 0 {
			redisc.SetQueue(ctx, uc.rdb, ref, tracks)
		}
		messages, _ := uc.sessionRepo.LoadMessages(ref)
		for _, m := range messages {
			redisc.AppendMessage(ctx, uc.rdb, ref, m)
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

func (uc *RoomUsecase) ListPublicRooms(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	dbRooms, err := uc.roomRepo.ListPublic()
	if err != nil {
		return nil, err
	}

	type roomCmds struct {
		presence *redis.IntCmd
		qlen     *redis.IntCmd
		playing  *redis.StringCmd
	}
	pipe := uc.rdb.Pipeline()
	cmds := make([]roomCmds, len(dbRooms))
	for i, r := range dbRooms {
		roomID, _ := r["id"].(string)
		ref := entity.NewRoomRef(roomID)
		cmds[i] = roomCmds{
			presence: pipe.HLen(ctx, redisc.SessionKey(ref, "presence")),
			qlen:     pipe.LLen(ctx, redisc.SessionKey(ref, "queue")),
			playing:  pipe.HGet(ctx, redisc.SessionKey(ref, "state"), "is_playing"),
		}
	}
	_, _ = pipe.Exec(ctx)

	for i, r := range dbRooms {
		if n, err := cmds[i].presence.Result(); err == nil {
			r["user_count"] = int(n)
		} else {
			r["user_count"] = 0
		}
		if n, err := cmds[i].qlen.Result(); err == nil {
			r["track_count"] = int(n)
		} else {
			r["track_count"] = 0
		}
		if v, err := cmds[i].playing.Result(); err == nil {
			r["is_playing"] = v == "1"
		} else {
			r["is_playing"] = false
		}
	}

	sort.Slice(dbRooms, func(i, j int) bool {
		a, _ := dbRooms[i]["user_count"].(int)
		b, _ := dbRooms[j]["user_count"].(int)
		return a > b
	})

	if offset < 0 {
		offset = 0
	}
	if offset >= len(dbRooms) {
		return []map[string]any{}, nil
	}
	dbRooms = dbRooms[offset:]
	if limit >= 0 && len(dbRooms) > limit {
		dbRooms = dbRooms[:limit]
	}
	return dbRooms, nil
}

func (uc *RoomUsecase) HasRoomAccess(rm *entity.Room, userID, access string) bool {
	if rm.PasswordHash == nil {
		return true
	}
	if userID != "" && userID == rm.OwnerID {
		return true
	}
	if userID != "" {
		user, err := uc.userRepo.FindByID(userID)
		if err == nil && user.IsAdmin() {
			return true
		}
	}
	return uc.auth.VerifyRoomToken(access, rm.ID)
}

// IsController reports whether userID may drive room playback (owner/admin).
func (uc *RoomUsecase) IsController(rm *entity.Room, userID string) bool {
	if userID == "" {
		return false
	}
	if userID == rm.OwnerID {
		return true
	}
	user, err := uc.userRepo.FindByID(userID)
	return err == nil && user.IsAdmin()
}

func (uc *RoomUsecase) JoinRoom(rm *entity.Room, password string, userID string) (string, error) {
	if rm.PasswordHash != nil {
		if userID != "" {
			user, err := uc.userRepo.FindByID(userID)
			if err == nil && user.IsAdmin() {
				return uc.auth.CreateRoomToken(rm.ID)
			}
		}
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
	if uc.sessionRepo == nil {
		return nil
	}
	tracks, err := redisc.GetQueue(ctx, uc.rdb, rm.Ref())
	if err != nil {
		return fmt.Errorf("get queue from redis: %w", err)
	}
	return uc.sessionRepo.SaveTracks(rm.Ref(), tracks)
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
		ref := entity.NewRoomRef(roomID)
		count, _ := redisc.GetPresenceCount(ctx, uc.rdb, ref)
		r["user_count"] = int(count)

		tracks, _ := redisc.GetQueue(ctx, uc.rdb, ref)
		r["track_count"] = len(tracks)

		ps, _ := redisc.GetPlayback(ctx, uc.rdb, ref)
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

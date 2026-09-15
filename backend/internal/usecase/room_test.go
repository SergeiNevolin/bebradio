package usecase

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

var testLog2 = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func testConfig() *config.Config {
	return &config.Config{
		RadioRefillAt:    1,
		RadioBatch:       3,
		MaxDuration:      3600,
		AutoAdvanceGrace: 2.5,
	}
}

func TestCreateRoom(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm, access, err := uc.CreateRoom(ctx, "Test Room", "owner1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rm.Name != "Test Room" {
		t.Errorf("expected name 'Test Room', got '%s'", rm.Name)
	}
	if rm.OwnerID != "owner1" {
		t.Errorf("expected owner_id 'owner1', got '%s'", rm.OwnerID)
	}
	if len(rm.ID) != 6 {
		t.Errorf("expected 6-char ID, got '%s'", rm.ID)
	}
	if access == "" {
		t.Error("expected non-empty access token")
	}
	if rm.PasswordHash != nil {
		t.Error("expected nil password hash when no password set")
	}
}

func TestCreateRoomWithPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm, _, err := uc.CreateRoom(ctx, "Private Room", "owner1", "secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rm.PasswordHash == nil {
		t.Error("expected non-nil password hash")
	}
}

func TestGetOrLoadRoom(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm, _, _ := uc.CreateRoom(ctx, "Test Room", "owner1", "")

	found, err := uc.GetOrLoadRoom(ctx, rm.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find room")
	}
	if found.ID != rm.ID {
		t.Errorf("expected room ID '%s', got '%s'", rm.ID, found.ID)
	}
}

func TestGetOrLoadRoomFromDB(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	room := entity.NewRoom("DB001", "DB Room", "owner1")
	roomRepo.Save(room)

	found, err := uc.GetOrLoadRoom(ctx, "DB001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.Name != "DB Room" {
		t.Errorf("expected name 'DB Room', got '%s'", found.Name)
	}
}

func TestGetOrLoadRoomNotFound(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	_, err := uc.GetOrLoadRoom(ctx, "NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestHasRoomAccessNoPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm := entity.NewRoom("X", "R", "O")
	if !uc.HasRoomAccess(rm, "", "") {
		t.Error("expected access for room without password")
	}
}

func TestHasRoomAccessOwner(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	hash := "hashed_password"
	rm := entity.NewRoom("X", "R", "owner1")
	rm.PasswordHash = &hash

	if !uc.HasRoomAccess(rm, "owner1", "") {
		t.Error("expected owner to have access")
	}
}

func TestHasRoomAccessWithToken(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	hash := "hashed_password"
	rm := entity.NewRoom("X", "R", "owner1")
	rm.PasswordHash = &hash

	if !uc.HasRoomAccess(rm, "other_user", "room_token_X") {
		t.Error("expected access with valid room token")
	}
}

func TestHasRoomAccessDenied(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	hash := "hashed_password"
	rm := entity.NewRoom("X", "R", "owner1")
	rm.PasswordHash = &hash

	if uc.HasRoomAccess(rm, "other_user", "") {
		t.Error("expected denied without token")
	}
}

func TestJoinRoomNoPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm := entity.NewRoom("X", "R", "O")

	token, err := uc.JoinRoom(rm, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestJoinRoomCorrectPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	hash := "hashed_secret123"
	rm := entity.NewRoom("X", "R", "O")
	rm.PasswordHash = &hash

	token, err := uc.JoinRoom(rm, "secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestJoinRoomWrongPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	hash := "hashed_secret123"
	rm := entity.NewRoom("X", "R", "O")
	rm.PasswordHash = &hash

	_, err := uc.JoinRoom(rm, "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	be, ok := err.(*BusinessError)
	if !ok {
		t.Fatalf("expected BusinessError, got %T", err)
	}
	if be.Code != 403 {
		t.Errorf("expected 403, got %d", be.Code)
	}
}

func TestUpdateRoomSettings(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm := entity.NewRoom("X", "R", "O")

	f := false
	tr := true
	if err := uc.UpdateRoomSettings(ctx, rm, &tr, &f, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !rm.AllowAnonymousAdd {
		t.Error("expected AllowAnonymousAdd true")
	}
	if rm.IsPrivate {
		t.Error("expected IsPrivate false")
	}
}

func TestUpdateRoomSettingsSetPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm := entity.NewRoom("X", "R", "O")

	pw := "newpassword"
	if err := uc.UpdateRoomSettings(ctx, rm, nil, nil, nil, &pw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rm.PasswordHash == nil {
		t.Error("expected password hash to be set")
	}
}

func TestUpdateRoomSettingsRemovePassword(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	hash := "old_hash"
	rm := entity.NewRoom("X", "R", "O")
	rm.PasswordHash = &hash

	pw := ""
	if err := uc.UpdateRoomSettings(ctx, rm, nil, nil, nil, &pw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rm.PasswordHash != nil {
		t.Error("expected password hash to be removed")
	}
}

func TestDeleteRoom(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm, _, _ := uc.CreateRoom(ctx, "To Delete", "owner1", "")

	err := uc.DeleteRoom(ctx, rm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found, _ := uc.GetOrLoadRoom(ctx, rm.ID)
	if found != nil {
		t.Error("expected room to be removed")
	}
}

func TestListPublicRooms(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	uc.CreateRoom(ctx, "Public Room", "owner1", "")

	rooms, err := uc.ListPublicRooms(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected 1 public room, got %d", len(rooms))
	}
}

func TestListPublicRoomsExcludesPrivate(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm, _, _ := uc.CreateRoom(ctx, "Private Room", "owner1", "")
	rm.IsPrivate = true
	roomRepo.Save(rm)

	rooms, _ := uc.ListPublicRooms(ctx)
	if len(rooms) != 0 {
		t.Errorf("expected 0 public rooms, got %d", len(rooms))
	}
}

func TestGetOrLoadRoomDoesNotResurrectSkippedTracks(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2, rdb)

	rm, _, _ := uc.CreateRoom(ctx, "Skip Room", "owner1", "")

	// Stale Postgres rows, as left behind by a skip that wasn't persisted yet.
	roomRepo.Tracks[rm.ID] = []*entity.Track{{ID: "stale1"}, {ID: "stale2"}}

	// First load hydrates Redis from Postgres.
	if _, err := uc.GetOrLoadRoom(ctx, rm.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q, _ := redisc.GetQueue(ctx, rdb, rm.ID)
	if len(q) != 2 {
		t.Fatalf("expected 2 hydrated tracks, got %d", len(q))
	}

	// Simulate skipping everything in Redis; Postgres stays stale.
	redisc.RemoveAt(ctx, rdb, rm.ID, 0)
	redisc.RemoveAt(ctx, rdb, rm.ID, 0)
	q, _ = redisc.GetQueue(ctx, rdb, rm.ID)
	if len(q) != 0 {
		t.Fatalf("expected empty queue after skips, got %d", len(q))
	}

	// Repeated loads (autoadvance ticks, new connections) must not resurrect.
	for i := 0; i < 3; i++ {
		if _, err := uc.GetOrLoadRoom(ctx, rm.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	q, _ = redisc.GetQueue(ctx, rdb, rm.ID)
	if len(q) != 0 {
		t.Errorf("skipped tracks resurrected: expected empty queue, got %d tracks", len(q))
	}
}

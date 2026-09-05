package usecase

import (
	"log/slog"
	"os"
	"testing"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

var testLog2 = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func testConfig() *config.Config {
	return &config.Config{
		RadioRefillAt:     1,
		RadioBatch:        3,
		MaxDuration:       3600,
		AutoAdvanceGrace:  2.5,
		AdvanceDedupWindow: 1.0,
	}
}

func TestCreateRoom(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm, access, err := uc.CreateRoom("Test Room", "owner1", "")
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
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm, _, err := uc.CreateRoom("Private Room", "owner1", "secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rm.PasswordHash == nil {
		t.Error("expected non-nil password hash")
	}
}

func TestGetOrLoadRoom(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm, _, _ := uc.CreateRoom("Test Room", "owner1", "")

	// Should be in memory
	found := uc.GetRoom(rm.ID)
	if found == nil {
		t.Fatal("expected to find room in memory")
	}
	if found.ID != rm.ID {
		t.Errorf("expected room ID '%s', got '%s'", rm.ID, found.ID)
	}
}

func TestGetOrLoadRoomFromDB(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	// Create a room in the repo directly
	room := entity.NewRoom("DB001", "DB Room", "owner1")
	roomRepo.Save(room)

	// Should load from DB
	found, err := uc.GetOrLoadRoom("DB001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.Name != "DB Room" {
		t.Errorf("expected name 'DB Room', got '%s'", found.Name)
	}
}

func TestGetOrLoadRoomNotFound(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	_, err := uc.GetOrLoadRoom("NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for nonexistent room")
	}
}

func TestHasRoomAccessNoPassword(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm := entity.NewRoom("X", "R", "O")
	if !uc.HasRoomAccess(rm, "", "") {
		t.Error("expected access for room without password")
	}
}

func TestHasRoomAccessOwner(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	hash := "hashed_password"
	rm := entity.NewRoom("X", "R", "owner1")
	rm.PasswordHash = &hash

	if !uc.HasRoomAccess(rm, "owner1", "") {
		t.Error("expected owner to have access")
	}
}

func TestHasRoomAccessWithToken(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	hash := "hashed_password"
	rm := entity.NewRoom("X", "R", "owner1")
	rm.PasswordHash = &hash

	if !uc.HasRoomAccess(rm, "other_user", "room_token_X") {
		t.Error("expected access with valid room token")
	}
}

func TestHasRoomAccessDenied(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	hash := "hashed_password"
	rm := entity.NewRoom("X", "R", "owner1")
	rm.PasswordHash = &hash

	if uc.HasRoomAccess(rm, "other_user", "") {
		t.Error("expected denied without token")
	}
}

func TestJoinRoomNoPassword(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

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
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

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
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

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
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm := entity.NewRoom("X", "R", "O")

	f := false
	tr := true
	uc.UpdateRoomSettings(rm, &tr, &f, nil, nil)

	if !rm.AllowAnonymousAdd {
		t.Error("expected AllowAnonymousAdd true")
	}
	if rm.IsPrivate {
		t.Error("expected IsPrivate false")
	}
}

func TestUpdateRoomSettingsSetPassword(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm := entity.NewRoom("X", "R", "O")

	pw := "newpassword"
	uc.UpdateRoomSettings(rm, nil, nil, nil, &pw)

	if rm.PasswordHash == nil {
		t.Error("expected password hash to be set")
	}
}

func TestUpdateRoomSettingsRemovePassword(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	hash := "old_hash"
	rm := entity.NewRoom("X", "R", "O")
	rm.PasswordHash = &hash

	pw := ""
	uc.UpdateRoomSettings(rm, nil, nil, nil, &pw)

	if rm.PasswordHash != nil {
		t.Error("expected password hash to be removed")
	}
}

func TestDeleteRoom(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm, _, _ := uc.CreateRoom("To Delete", "owner1", "")

	err := uc.DeleteRoom(rm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if uc.GetRoom(rm.ID) != nil {
		t.Error("expected room to be removed from memory")
	}
}

func TestListPublicRooms(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	uc.CreateRoom("Public Room", "owner1", "")

	rooms, err := uc.ListPublicRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 1 {
		t.Errorf("expected 1 public room, got %d", len(rooms))
	}
}

func TestListPublicRoomsExcludesPrivate(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm, _, _ := uc.CreateRoom("Private Room", "owner1", "")
	rm.IsPrivate = true
	roomRepo.Save(rm)

	rooms, _ := uc.ListPublicRooms()
	if len(rooms) != 0 {
		t.Errorf("expected 0 public rooms, got %d", len(rooms))
	}
}

func TestSaveVotesPersistsToRepo(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm, _, _ := uc.CreateRoom("Vote Room", "owner1", "")
	rm.Votes = []*entity.TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: 1},
		{UserID: "u2", TrackID: "t1", Vote: -1},
		{UserID: "u3", TrackID: "t2", Vote: 1},
	}

	if err := uc.SaveVotes(rm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	saved, err := roomRepo.LoadVotes(rm.ID)
	if err != nil {
		t.Fatalf("unexpected error loading votes: %v", err)
	}
	if len(saved) != 3 {
		t.Fatalf("expected 3 votes in repo, got %d", len(saved))
	}
}

func TestSaveVotesOverwritesPrevious(t *testing.T) {
	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	uc := NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, testLog2)

	rm, _, _ := uc.CreateRoom("Vote Room", "owner1", "")

	rm.Votes = []*entity.TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: 1},
	}
	uc.SaveVotes(rm)

	rm.Votes = []*entity.TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: -1},
		{UserID: "u2", TrackID: "t1", Vote: 1},
	}
	uc.SaveVotes(rm)

	saved, _ := roomRepo.LoadVotes(rm.ID)
	if len(saved) != 2 {
		t.Fatalf("expected 2 votes after overwrite, got %d", len(saved))
	}
	for _, v := range saved {
		if v.UserID == "u1" && v.Vote != -1 {
			t.Errorf("expected u1 vote to be -1, got %d", v.Vote)
		}
	}
}

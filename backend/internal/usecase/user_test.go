package usecase

import (
	"log/slog"
	"os"
	"testing"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

var userLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestGetProfile(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	uc := NewUserUsecase(userRepo, userLog)

	userRepo.Create(testUser("u1", "test@example.com", "testuser"))

	profile, err := uc.GetProfile("u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", profile.Username)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	uc := NewUserUsecase(userRepo, userLog)

	_, err := uc.GetProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestUpdateProfile(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	uc := NewUserUsecase(userRepo, userLog)

	userRepo.Create(testUser("u1", "test@example.com", "testuser"))

	bio := "New bio"
	avatar := "http://example.com/new.jpg"
	profile, err := uc.UpdateProfile("u1", &bio, &avatar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Bio != "New bio" {
		t.Errorf("expected bio 'New bio', got '%s'", profile.Bio)
	}
	if profile.AvatarURL != "http://example.com/new.jpg" {
		t.Errorf("expected avatar_url, got '%s'", profile.AvatarURL)
	}
}

func TestGetUser(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	uc := NewUserUsecase(userRepo, userLog)

	userRepo.Create(testUser("u1", "test@example.com", "testuser"))

	user, err := uc.GetUser("u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email, got '%s'", user.Email)
	}
}

func testUser(id, email, username string) *entity.User {
	return &entity.User{
		ID:       id,
		Email:    email,
		Username: username,
	}
}

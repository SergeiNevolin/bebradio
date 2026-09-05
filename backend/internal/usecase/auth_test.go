package usecase

import (
	"log/slog"
	"os"
	"testing"

	"github.com/bebradio/backend-go/internal/domain/repository"
)

var testLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestRegisterSuccess(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	user, token, err := uc.Register("test@example.com", "testuser", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", user.Email)
	}
	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", user.Username)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	uc.Register("test@example.com", "user1", "pass1")

	_, _, err := uc.Register("test@example.com", "user2", "pass2")
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
	be, ok := err.(*BusinessError)
	if !ok {
		t.Fatalf("expected BusinessError, got %T", err)
	}
	if be.Code != 409 {
		t.Errorf("expected status 409, got %d", be.Code)
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	uc.Register("user1@example.com", "sameuser", "pass1")

	_, _, err := uc.Register("user2@example.com", "sameuser", "pass2")
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
	be, ok := err.(*BusinessError)
	if !ok {
		t.Fatalf("expected BusinessError, got %T", err)
	}
	if be.Code != 409 {
		t.Errorf("expected status 409, got %d", be.Code)
	}
}

func TestLoginSuccess(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	uc.Register("test@example.com", "testuser", "password123")

	user, token, err := uc.Login("test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	uc.Register("test@example.com", "testuser", "password123")

	_, _, err := uc.Login("test@example.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	be, ok := err.(*BusinessError)
	if !ok {
		t.Fatalf("expected BusinessError, got %T", err)
	}
	if be.Code != 401 {
		t.Errorf("expected status 401, got %d", be.Code)
	}
}

func TestLoginNonexistentUser(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	_, _, err := uc.Login("nonexistent@example.com", "password")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	be, ok := err.(*BusinessError)
	if !ok {
		t.Fatalf("expected BusinessError, got %T", err)
	}
	if be.Code != 401 {
		t.Errorf("expected status 401, got %d", be.Code)
	}
}

func TestDecodeToken(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	// Mock decode returns the token itself as user ID
	userID, err := uc.DecodeToken("test_token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "test_token" {
		t.Errorf("expected 'test_token', got '%s'", userID)
	}
}

func TestGetUserByID(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	uc.Register("test@example.com", "testuser", "password123")

	// Find by the generated ID
	for _, u := range userRepo.Users {
		found, err := uc.GetUserByID(u.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found.Email != "test@example.com" {
			t.Errorf("expected email 'test@example.com', got '%s'", found.Email)
		}
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	userRepo := repository.NewMockUserRepo()
	auth := repository.NewMockAuthBridge()
	uc := NewAuthUsecase(userRepo, auth, testLog)

	_, err := uc.GetUserByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

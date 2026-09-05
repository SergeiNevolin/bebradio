package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	svc := New("secret", 72)

	hash, err := svc.HashPassword("mypassword")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if hash == "mypassword" {
		t.Error("hash should not equal plaintext")
	}
}

func TestVerifyPassword(t *testing.T) {
	svc := New("secret", 72)

	hash, _ := svc.HashPassword("correct_password")

	if !svc.VerifyPassword("correct_password", hash) {
		t.Error("expected password to verify")
	}
	if svc.VerifyPassword("wrong_password", hash) {
		t.Error("expected wrong password to fail")
	}
}

func TestVerifyPasswordDifferentHashes(t *testing.T) {
	svc := New("secret", 72)

	hash1, _ := svc.HashPassword("pass1")
	hash2, _ := svc.HashPassword("pass2")

	if svc.VerifyPassword("pass1", hash2) {
		t.Error("pass1 should not verify against hash2")
	}
	if svc.VerifyPassword("pass2", hash1) {
		t.Error("pass2 should not verify against hash1")
	}
}

func TestCreateToken(t *testing.T) {
	svc := New("secret", 72)

	token, err := svc.CreateToken("user123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	// JWT tokens have 3 parts separated by dots
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("expected 3-part JWT, got %d parts", len(parts))
	}
}

func TestDecodeToken(t *testing.T) {
	svc := New("secret", 72)

	token, _ := svc.CreateToken("user123")

	userID, err := svc.DecodeToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "user123" {
		t.Errorf("expected 'user123', got '%s'", userID)
	}
}

func TestDecodeTokenWrongKey(t *testing.T) {
	svc1 := New("secret1", 72)
	svc2 := New("secret2", 72)

	token, _ := svc1.CreateToken("user123")

	_, err := svc2.DecodeToken(token)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestDecodeTokenExpired(t *testing.T) {
	svc := New("secret", -1) // expired 1 hour ago

	token, _ := svc.CreateToken("user123")

	_, err := svc.DecodeToken(token)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestDecodeTokenInvalid(t *testing.T) {
	svc := New("secret", 72)

	_, err := svc.DecodeToken("not.a.valid.token")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestDecodeTokenEmpty(t *testing.T) {
	svc := New("secret", 72)

	_, err := svc.DecodeToken("")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for empty token, got %v", err)
	}
}

func TestCreateRoomToken(t *testing.T) {
	svc := New("secret", 72)

	token, err := svc.CreateRoomToken("ROOM1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestVerifyRoomToken(t *testing.T) {
	svc := New("secret", 72)

	token, _ := svc.CreateRoomToken("ROOM1")

	if !svc.VerifyRoomToken(token, "ROOM1") {
		t.Error("expected valid room token")
	}
}

func TestVerifyRoomTokenWrongRoom(t *testing.T) {
	svc := New("secret", 72)

	token, _ := svc.CreateRoomToken("ROOM1")

	if svc.VerifyRoomToken(token, "ROOM2") {
		t.Error("expected invalid for wrong room ID")
	}
}

func TestVerifyRoomTokenEmpty(t *testing.T) {
	svc := New("secret", 72)

	if svc.VerifyRoomToken("", "ROOM1") {
		t.Error("expected false for empty token")
	}
}

func TestVerifyRoomTokenInvalid(t *testing.T) {
	svc := New("secret", 72)

	if svc.VerifyRoomToken("garbage", "ROOM1") {
		t.Error("expected false for invalid token")
	}
}

func TestVerifyRoomTokenUserToken(t *testing.T) {
	svc := New("secret", 72)

	// A user token (not a room token) should not verify
	userToken, _ := svc.CreateToken("user1")

	if svc.VerifyRoomToken(userToken, "ROOM1") {
		t.Error("expected false for user token used as room token")
	}
}

func TestTokenExpiry(t *testing.T) {
	svc := New("secret", 1) // 1 hour

	token, _ := svc.CreateToken("user1")
	userID, err := svc.DecodeToken(token)
	if err != nil {
		t.Fatalf("token should be valid immediately: %v", err)
	}
	if userID != "user1" {
		t.Errorf("expected 'user1', got '%s'", userID)
	}
}

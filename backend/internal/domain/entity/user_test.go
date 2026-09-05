package entity

import "testing"

func TestUserPublicProfile(t *testing.T) {
	u := &User{
		ID:         "u1",
		Email:      "test@example.com",
		Username:   "testuser",
		Bio:        "Hello world",
		AvatarURL:  "http://example.com/avatar.jpg",
	}

	profile := u.PublicProfile()
	if profile["id"] != "u1" {
		t.Errorf("expected id 'u1', got '%v'", profile["id"])
	}
	if profile["username"] != "testuser" {
		t.Errorf("expected username 'testuser', got '%v'", profile["username"])
	}
	if profile["bio"] != "Hello world" {
		t.Errorf("expected bio 'Hello world', got '%v'", profile["bio"])
	}
	if profile["avatar_url"] != "http://example.com/avatar.jpg" {
		t.Errorf("expected avatar_url, got '%v'", profile["avatar_url"])
	}
	// Public profile should NOT contain email
	if _, exists := profile["email"]; exists {
		t.Error("public profile should not contain email")
	}
}

func TestUserProfileWithEmail(t *testing.T) {
	u := &User{
		ID:       "u1",
		Email:    "test@example.com",
		Username: "testuser",
	}

	profile := u.ProfileWithEmail()
	if profile["email"] != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%v'", profile["email"])
	}
	if profile["id"] != "u1" {
		t.Errorf("expected id 'u1', got '%v'", profile["id"])
	}
}

func TestUserPublicProfileEmpty(t *testing.T) {
	u := &User{}
	profile := u.PublicProfile()

	if profile["id"] != "" {
		t.Errorf("expected empty id, got '%v'", profile["id"])
	}
	if profile["username"] != "" {
		t.Errorf("expected empty username, got '%v'", profile["username"])
	}
}

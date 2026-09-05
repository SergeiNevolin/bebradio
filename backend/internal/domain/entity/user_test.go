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

func TestGetTrackVotes(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Votes = []*TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: 1},
		{UserID: "u2", TrackID: "t1", Vote: 1},
		{UserID: "u3", TrackID: "t1", Vote: -1},
		{UserID: "u4", TrackID: "t2", Vote: 1},
	}

	likes, dislikes := rm.GetTrackVotes("t1")
	if likes != 2 {
		t.Errorf("expected 2 likes for t1, got %d", likes)
	}
	if dislikes != 1 {
		t.Errorf("expected 1 dislike for t1, got %d", dislikes)
	}

	likes, dislikes = rm.GetTrackVotes("t2")
	if likes != 1 {
		t.Errorf("expected 1 like for t2, got %d", likes)
	}
	if dislikes != 0 {
		t.Errorf("expected 0 dislikes for t2, got %d", dislikes)
	}

	likes, dislikes = rm.GetTrackVotes("t3")
	if likes != 0 || dislikes != 0 {
		t.Errorf("expected 0 votes for t3, got %d/%d", likes, dislikes)
	}
}

func TestToDictIncludesVotes(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Queue = []*Track{
		{ID: "t1", Title: "Song 1", Duration: 200},
		{ID: "t2", Title: "Song 2", Duration: 300},
	}
	rm.Votes = []*TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: 1},
		{UserID: "u2", TrackID: "t1", Vote: 1},
		{UserID: "u3", TrackID: "t1", Vote: -1},
		{UserID: "u4", TrackID: "t2", Vote: 1},
	}

	dict := rm.ToDict()
	queue := dict["queue"].([]map[string]any)

	if len(queue) != 2 {
		t.Fatalf("expected 2 queue items, got %d", len(queue))
	}

	t1 := queue[0]
	if t1["likes"] != 2 {
		t.Errorf("expected 2 likes for t1, got %v", t1["likes"])
	}
	if t1["dislikes"] != 1 {
		t.Errorf("expected 1 dislike for t1, got %v", t1["dislikes"])
	}

	t2 := queue[1]
	if t2["likes"] != 1 {
		t.Errorf("expected 1 like for t2, got %v", t2["likes"])
	}
	if t2["dislikes"] != 0 {
		t.Errorf("expected 0 dislikes for t2, got %v", t2["dislikes"])
	}
}

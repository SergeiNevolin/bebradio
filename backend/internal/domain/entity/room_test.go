package entity

import (
	"testing"
	"time"
)

func TestNewRoom(t *testing.T) {
	rm := NewRoom("ABC123", "Test Room", "owner1")

	if rm.ID != "ABC123" {
		t.Errorf("expected ID 'ABC123', got '%s'", rm.ID)
	}
	if rm.Name != "Test Room" {
		t.Errorf("expected Name 'Test Room', got '%s'", rm.Name)
	}
	if rm.OwnerID != "owner1" {
		t.Errorf("expected OwnerID 'owner1', got '%s'", rm.OwnerID)
	}
	if len(rm.Queue) != 0 {
		t.Errorf("expected empty Queue, got %d tracks", len(rm.Queue))
	}
	if rm.IsPlaying {
		t.Error("expected IsPlaying false")
	}
	if !rm.AllowAnonymousAdd {
		t.Error("expected AllowAnonymousAdd true")
	}
	if rm.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestRoomCurrentTrackEmpty(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	if rm.CurrentTrack() != nil {
		t.Error("expected nil current track for empty queue")
	}
}

func TestRoomCurrentTrack(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	track := &Track{ID: "t1", Title: "Song 1"}
	rm.Queue = []*Track{track}
	rm.CurrentIndex = 0

	current := rm.CurrentTrack()
	if current == nil {
		t.Fatal("expected non-nil current track")
	}
	if current.ID != "t1" {
		t.Errorf("expected track ID 't1', got '%s'", current.ID)
	}
}

func TestRoomCurrentTrackOutOfBounds(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Queue = []*Track{{ID: "t1"}}

	rm.CurrentIndex = 5
	if rm.CurrentTrack() != nil {
		t.Error("expected nil for out of bounds index")
	}

	rm.CurrentIndex = -1
	if rm.CurrentTrack() != nil {
		t.Error("expected nil for negative index")
	}
}

func TestRoomGetCurrentPositionPaused(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Position = 42.5
	rm.IsPlaying = false

	pos := rm.GetCurrentPosition()
	if pos != 42.5 {
		t.Errorf("expected position 42.5, got %f", pos)
	}
}

func TestRoomGetCurrentPositionPlaying(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Position = 10.0
	rm.IsPlaying = true
	rm.LastSyncAt = time.Now().Add(-2 * time.Second)

	pos := rm.GetCurrentPosition()
	if pos < 11.5 || pos > 13.0 {
		t.Errorf("expected position around 12.0, got %f", pos)
	}
}

func TestRoomGetTrackVotes(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Votes = []*TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: 1},
		{UserID: "u2", TrackID: "t1", Vote: 1},
		{UserID: "u3", TrackID: "t1", Vote: -1},
		{UserID: "u4", TrackID: "t2", Vote: 1},
	}

	likes, dislikes := rm.GetTrackVotes("t1")
	if likes != 2 || dislikes != 1 {
		t.Errorf("expected 2 likes, 1 dislike; got %d likes, %d dislikes", likes, dislikes)
	}

	likes, dislikes = rm.GetTrackVotes("t2")
	if likes != 1 || dislikes != 0 {
		t.Errorf("expected 1 like, 0 dislikes; got %d likes, %d dislikes", likes, dislikes)
	}

	likes, dislikes = rm.GetTrackVotes("nonexistent")
	if likes != 0 || dislikes != 0 {
		t.Errorf("expected 0 likes, 0 dislikes for nonexistent track; got %d, %d", likes, dislikes)
	}
}

func TestRoomListeners(t *testing.T) {
	rm := NewRoom("X", "R", "O")

	listeners := rm.Listeners()
	if len(listeners) != 0 {
		t.Errorf("expected 0 listeners, got %d", len(listeners))
	}

	rm.Presence["conn1"] = PresenceInfo{ID: "u1", Name: "Alice"}
	rm.Presence["conn2"] = PresenceInfo{ID: "u2", Name: "Bob"}

	listeners = rm.Listeners()
	if len(listeners) != 2 {
		t.Errorf("expected 2 listeners, got %d", len(listeners))
	}
}

func TestRoomListenersDedup(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Presence["conn1"] = PresenceInfo{ID: "u1", Name: "Alice"}
	rm.Presence["conn2"] = PresenceInfo{ID: "u1", Name: "Alice"}

	listeners := rm.Listeners()
	if len(listeners) != 1 {
		t.Errorf("expected 1 listener (deduped), got %d", len(listeners))
	}
}

func TestRoomToDictEmpty(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	dict := rm.ToDict()

	if dict["id"] != "X" {
		t.Errorf("expected id 'X', got '%v'", dict["id"])
	}
	if dict["name"] != "R" {
		t.Errorf("expected name 'R', got '%v'", dict["name"])
	}
	if dict["owner_id"] != "O" {
		t.Errorf("expected owner_id 'O', got '%v'", dict["owner_id"])
	}
	if dict["current_track"] != nil {
		t.Error("expected nil current_track for empty queue")
	}
	if dict["is_playing"] != false {
		t.Error("expected is_playing false")
	}
	if dict["allow_anonymous_add"] != true {
		t.Error("expected allow_anonymous_add true")
	}
	if dict["auto_radio"] != false {
		t.Error("expected auto_radio false")
	}
	if dict["has_password"] != false {
		t.Error("expected has_password false")
	}
	if dict["radio_searching"] != false {
		t.Error("expected radio_searching false")
	}
}

func TestRoomToDictWithTracks(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Queue = []*Track{
		{ID: "t1", Title: "Song 1", Artist: "Artist 1"},
		{ID: "t2", Title: "Song 2", Artist: "Artist 2"},
		{ID: "t3", Title: "Song 3", Artist: "Artist 3"},
	}
	rm.CurrentIndex = 1

	dict := rm.ToDict()
	queue := dict["queue"].([]map[string]any)

	if len(queue) != 3 {
		t.Fatalf("expected 3 tracks in queue, got %d", len(queue))
	}

	currentTrack := dict["current_track"].(map[string]any)
	if currentTrack["id"] != "t2" {
		t.Errorf("expected current track id 't2', got '%v'", currentTrack["id"])
	}

	if dict["current_index"] != 1 {
		t.Errorf("expected current_index 1, got '%v'", dict["current_index"])
	}
}

func TestRoomToDictWithVotes(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Queue = []*Track{{ID: "t1"}, {ID: "t2"}}
	rm.Votes = []*TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: 1},
		{UserID: "u2", TrackID: "t1", Vote: 1},
		{UserID: "u3", TrackID: "t1", Vote: -1},
	}

	dict := rm.ToDict()
	queue := dict["queue"].([]map[string]any)

	if queue[0]["likes"] != 2 {
		t.Errorf("expected 2 likes for t1, got %v", queue[0]["likes"])
	}
	if queue[0]["dislikes"] != 1 {
		t.Errorf("expected 1 dislike for t1, got %v", queue[0]["dislikes"])
	}
}

func TestRoomToDictHasPassword(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	if rm.ToDict()["has_password"] != false {
		t.Error("expected has_password false without hash")
	}

	hash := "hashed_password"
	rm.PasswordHash = &hash
	if rm.ToDict()["has_password"] != true {
		t.Error("expected has_password true with hash")
	}

	// Ensure password_hash is never leaked
	dict := rm.ToDict()
	if _, exists := dict["password_hash"]; exists {
		t.Error("password_hash should not be in dict")
	}
}

func TestRoomToDictSkipVoters(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.SkipVotes = map[string]bool{"u1": true, "u2": true}

	dict := rm.ToDict()
	voters := dict["skip_voters"].([]string)
	if len(voters) != 2 {
		t.Errorf("expected 2 skip voters, got %d", len(voters))
	}
}

func TestRoomToDictListeners(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Presence["c1"] = PresenceInfo{ID: "u1", Name: "Alice"}

	dict := rm.ToDict()
	listeners := dict["listeners"].([]map[string]any)
	if len(listeners) != 1 {
		t.Errorf("expected 1 listener, got %d", len(listeners))
	}
}

func TestRoomToDictUserCount(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Presence["c1"] = PresenceInfo{ID: "u1", Name: "A"}
	rm.Presence["c2"] = PresenceInfo{ID: "u2", Name: "B"}

	dict := rm.ToDict()
	if dict["user_count"] != 2 {
		t.Errorf("expected user_count 2, got %v", dict["user_count"])
	}
}

func TestRoomToDictMessages(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	rm.Messages = []*ChatMessage{
		{ID: "m1", UserID: "u1", Username: "Alice", Text: "hello"},
	}

	dict := rm.ToDict()
	msgs := dict["messages"].([]map[string]any)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0]["text"] != "hello" {
		t.Errorf("expected message text 'hello', got '%v'", msgs[0]["text"])
	}
}

func TestRoomSkipVotesIsMap(t *testing.T) {
	rm := NewRoom("X", "R", "O")
	if rm.SkipVotes == nil {
		t.Error("expected SkipVotes to be initialized")
	}
	rm.SkipVotes["u1"] = true
	if !rm.SkipVotes["u1"] {
		t.Error("expected SkipVotes[u1] to be true")
	}
}

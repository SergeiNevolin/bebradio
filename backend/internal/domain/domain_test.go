package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewTrackFromInfoMapsEveryField(t *testing.T) {
	track := NewTrackFromInfo(TrackInfo{
		Title:     "Song",
		Artist:    "Band",
		StreamURL: "http://s",
		Thumbnail: "http://t",
		Duration:  123,
		SourceURL: "https://youtu.be/abc",
		ExpiresAt: 42,
	}, "Alice")

	if len(track.ID) != 8 {
		t.Errorf("track id = %q, want 8 characters", track.ID)
	}
	if track.Title != "Song" || track.Artist != "Band" || track.URL != "http://s" {
		t.Errorf("unexpected track: %+v", track)
	}
	if track.Thumbnail != "http://t" || track.Duration != 123 {
		t.Errorf("unexpected track: %+v", track)
	}
	if track.AddedBy != "Alice" {
		t.Errorf("added_by = %q, want Alice", track.AddedBy)
	}
	if track.SourceURL != "https://youtu.be/abc" || track.StreamExpiresAt != 42 {
		t.Errorf("unexpected source/expiry: %+v", track)
	}
}

func TestNewTrackFromInfoFillsMissingFields(t *testing.T) {
	track := NewTrackFromInfo(TrackInfo{StreamURL: "http://s"}, "")

	if track.URL != "http://s" {
		t.Errorf("url = %q", track.URL)
	}
	if track.Title != "Unknown" || track.Artist != "Unknown" {
		t.Errorf("title/artist = %q/%q, want Unknown", track.Title, track.Artist)
	}
	if track.AddedBy != "Anonymous" {
		t.Errorf("added_by = %q, want Anonymous", track.AddedBy)
	}
	if track.SourceURL != "" || track.StreamExpiresAt != 0 {
		t.Errorf("unexpected source/expiry: %+v", track)
	}
}

func TestNewRoomDefaults(t *testing.T) {
	r := NewRoom("", "", "owner")

	if len(r.ID) != 6 {
		t.Errorf("room id = %q, want 6 characters", r.ID)
	}
	if r.ID != upper(r.ID) {
		t.Errorf("room id = %q, want uppercase", r.ID)
	}
	if len(r.Queue) != 0 {
		t.Errorf("queue = %v, want empty", r.Queue)
	}
	if r.IsPlaying {
		t.Error("a new room should not be playing")
	}
	if !r.AllowAnonymousAdd {
		t.Error("a new room should accept anonymous additions")
	}
}

func TestCurrentTrack(t *testing.T) {
	t.Run("empty queue", func(t *testing.T) {
		if got := NewRoom("", "", "").CurrentTrack(); got != nil {
			t.Errorf("CurrentTrack() = %v, want nil", got)
		}
	})

	t.Run("first track", func(t *testing.T) {
		r := NewRoom("", "", "")
		r.Queue = append(r.Queue, &Track{ID: "abc"})
		if got := r.CurrentTrack(); got == nil || got.ID != "abc" {
			t.Errorf("CurrentTrack() = %v, want abc", got)
		}
	})

	t.Run("index out of range", func(t *testing.T) {
		r := NewRoom("", "", "")
		r.Queue = append(r.Queue, &Track{})
		r.CurrentIndex = 5
		if got := r.CurrentTrack(); got != nil {
			t.Errorf("CurrentTrack() = %v, want nil", got)
		}
	})
}

func TestCurrentPosition(t *testing.T) {
	t.Run("paused returns the stored position", func(t *testing.T) {
		r := NewRoom("", "", "")
		r.Position = 10
		r.IsPlaying = false
		if got := r.CurrentPosition(); got != 10 {
			t.Errorf("CurrentPosition() = %v, want 10", got)
		}
	})

	t.Run("playing extrapolates from the last sync", func(t *testing.T) {
		r := NewRoom("", "", "")
		r.Position = 10
		r.IsPlaying = true
		r.LastSyncAt = Now() - 5

		got := r.CurrentPosition()
		if got < 14.5 || got > 15.5 {
			t.Errorf("CurrentPosition() = %v, want about 15", got)
		}
	})
}

func TestTrackVotes(t *testing.T) {
	r := NewRoom("", "", "")
	r.Votes = []TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: 1},
		{UserID: "u2", TrackID: "t1", Vote: 1},
		{UserID: "u3", TrackID: "t1", Vote: -1},
		{UserID: "u4", TrackID: "t2", Vote: 1},
	}

	if got := r.TrackVotes("t1"); got.Likes != 2 || got.Dislikes != 1 {
		t.Errorf("TrackVotes(t1) = %+v, want 2 likes and 1 dislike", got)
	}
	if got := r.TrackVotes("nope"); got.Likes != 0 || got.Dislikes != 0 {
		t.Errorf("TrackVotes(nope) = %+v, want zero", got)
	}
}

func TestSetVoteReplacesAndClears(t *testing.T) {
	r := NewRoom("", "", "")

	r.SetVote("u1", "t1", 1)
	r.SetVote("u1", "t1", -1)
	if got := r.TrackVotes("t1"); got.Likes != 0 || got.Dislikes != 1 {
		t.Errorf("after changing a vote: %+v, want 0 likes and 1 dislike", got)
	}

	r.SetVote("u1", "t1", 0)
	if got := r.TrackVotes("t1"); got.Likes != 0 || got.Dislikes != 0 {
		t.Errorf("after clearing a vote: %+v, want zero", got)
	}
}

func TestToggleSkipVote(t *testing.T) {
	r := NewRoom("", "", "")

	if got := r.ToggleSkipVote("u1"); got != 1 {
		t.Errorf("first toggle = %d, want 1", got)
	}
	if got := r.ToggleSkipVote("u1"); got != 0 {
		t.Errorf("second toggle = %d, want 0", got)
	}
}

func TestListenersDeduplicateByIdentity(t *testing.T) {
	r := NewRoom("", "", "")
	// One person with two tabs, plus one guest.
	r.SetPresence(1, Listener{ID: "u1", Name: "Alice"})
	r.SetPresence(2, Listener{ID: "u1", Name: "Alice"})
	r.SetPresence(3, Listener{ID: "anon:3", Name: "Guest"})

	listeners := r.Listeners()
	if len(listeners) != 2 {
		t.Fatalf("Listeners() = %v, want 2 entries", listeners)
	}
	seen := map[string]bool{}
	for _, l := range listeners {
		seen[l.ID] = true
	}
	if !seen["u1"] || !seen["anon:3"] {
		t.Errorf("Listeners() = %v, want u1 and anon:3", listeners)
	}
	if got := r.Snapshot(100).UserCount; got != 2 {
		t.Errorf("user_count = %d, want 2", got)
	}
}

func TestAppendMessageTrimsBacklog(t *testing.T) {
	r := NewRoom("", "", "")
	for i := 0; i < 10; i++ {
		r.AppendMessage(NewChatMessage("u", "User", "hello"), 3)
	}
	if len(r.Messages) != 3 {
		t.Errorf("kept %d messages, want 3", len(r.Messages))
	}
}

func TestDropConnectionForgetsEverything(t *testing.T) {
	r := NewRoom("", "", "")
	r.SetPresence(7, Listener{ID: "u1", Name: "Alice"})
	r.SetUser(7, "u1")

	r.DropConnection(7)

	if len(r.Presence) != 0 || len(r.Users) != 0 {
		t.Errorf("presence=%v users=%v, want both empty", r.Presence, r.Users)
	}
}

func TestSnapshotShape(t *testing.T) {
	r := NewRoom("ABC123", "Test", "owner")
	r.Queue = append(r.Queue, &Track{ID: "t1", Title: "Song"})
	r.Votes = []TrackVote{{UserID: "u1", TrackID: "t1", Vote: 1}}
	r.SkipVotes["u1"] = struct{}{}

	snap := r.Snapshot(100)

	if snap.Name != "Test" || snap.ID != "ABC123" {
		t.Errorf("unexpected snapshot header: %+v", snap)
	}
	if len(snap.Queue) != 1 || snap.Queue[0].Likes != 1 {
		t.Errorf("queue = %+v, want one entry with one like", snap.Queue)
	}
	if snap.CurrentTrack == nil || snap.CurrentTrack.ID != "t1" {
		t.Errorf("current_track = %+v, want t1", snap.CurrentTrack)
	}
	if len(snap.SkipVoters) != 1 {
		t.Errorf("skip_voters = %v, want one", snap.SkipVoters)
	}
	if snap.UserCount != 0 {
		t.Errorf("user_count = %d, want 0", snap.UserCount)
	}
}

func TestSnapshotOfEmptyRoom(t *testing.T) {
	snap := NewRoom("", "", "").Snapshot(100)

	if snap.CurrentTrack != nil {
		t.Errorf("current_track = %+v, want nil", snap.CurrentTrack)
	}
	if len(snap.Queue) != 0 {
		t.Errorf("queue = %v, want empty", snap.Queue)
	}
	if snap.RadioSearching || snap.AutoRadio || snap.HasPassword {
		t.Errorf("unexpected flags: %+v", snap)
	}
	if snap.Listeners == nil {
		t.Error("listeners should serialise as an empty array, not null")
	}
}

// A room's password hash must never reach a client, whatever else changes about
// the payload.
func TestSnapshotNeverLeaksPasswordHash(t *testing.T) {
	r := NewRoom("", "", "")
	r.PasswordHash = "super-secret-hash"

	encoded, err := json.Marshal(r.Snapshot(100))
	if err != nil {
		t.Fatalf("marshalling snapshot: %v", err)
	}
	if strings.Contains(string(encoded), "super-secret-hash") || strings.Contains(string(encoded), "password_hash") {
		t.Errorf("snapshot leaked the password hash: %s", encoded)
	}
	if !r.Snapshot(100).HasPassword {
		t.Error("has_password should be true when a password is set")
	}
}

func TestSnapshotReportsRadioSearching(t *testing.T) {
	r := NewRoom("", "", "")
	if r.Snapshot(100).RadioSearching {
		t.Error("radio_searching should start false")
	}
	r.RadioFilling = true
	if !r.Snapshot(100).RadioSearching {
		t.Error("radio_searching should follow RadioFilling")
	}
}

func TestSnapshotPositionFollowsPlayback(t *testing.T) {
	r := NewRoom("", "", "")
	r.Position = 20
	r.IsPlaying = true
	r.LastSyncAt = Now() - 3

	if got := r.Snapshot(100).Position; got < 22.5 || got > 23.5 {
		t.Errorf("position = %v, want about 23", got)
	}

	r.IsPlaying = false
	if got := r.Snapshot(100).Position; got != 20 {
		t.Errorf("paused position = %v, want 20", got)
	}
}

func TestSnapshotCapsChatBacklog(t *testing.T) {
	r := NewRoom("", "", "")
	for i := 0; i < 20; i++ {
		r.Messages = append(r.Messages, NewChatMessage("u", "User", "hi"))
	}
	if got := len(r.Snapshot(5).Messages); got != 5 {
		t.Errorf("snapshot carried %d messages, want 5", got)
	}
}

func TestNowTracksWallClock(t *testing.T) {
	before := float64(time.Now().Unix())
	got := Now()
	if got < before-1 || got > before+2 {
		t.Errorf("Now() = %v, want about %v", got, before)
	}
}

func upper(s string) string { return strings.ToUpper(s) }

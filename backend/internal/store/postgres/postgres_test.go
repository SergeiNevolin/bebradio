package postgres_test

// These exercise the real SQL against a real PostgreSQL. The in-memory store
// used elsewhere cannot catch a mistyped column, a broken constraint or a
// migration that will not apply, so this suite is the only thing standing
// between a schema change and production.
//
// Set TEST_DATABASE_URL to run them; without it they are skipped, so the rest
// of the suite still runs on a machine with no database.

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/ids"
	"github.com/leenzstra/bebradio/backend/internal/store"
	"github.com/leenzstra/bebradio/backend/internal/store/postgres"
)

// open connects to the test database, skipping the test when none is
// configured.
func open(t *testing.T) *postgres.Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping the PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := postgres.Open(ctx, postgres.Options{DatabaseURL: url, MaxConns: 4})
	if err != nil {
		t.Fatalf("connecting to the test database: %v", err)
	}
	t.Cleanup(db.Close)

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("applying the schema: %v", err)
	}
	return db
}

// newUser stores a fresh account and returns it.
func newUser(t *testing.T, db *postgres.Store) domain.User {
	t.Helper()
	user := domain.User{
		ID:           ids.Short(),
		Email:        ids.Short() + "@example.com",
		Username:     "user-" + ids.Short(),
		PasswordHash: "$2b$04$abcdefghijklmnopqrstuv",
		CreatedAt:    domain.Now(),
	}
	if err := db.CreateUser(t.Context(), user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	return user
}

// newRoom stores a fresh room and returns its record.
func newRoom(t *testing.T, db *postgres.Store, ownerID string) store.RoomRecord {
	t.Helper()
	room := store.RoomRecord{
		ID:                strings.ToUpper(ids.New(6)),
		Name:              "Test Room",
		OwnerID:           ownerID,
		AllowAnonymousAdd: true,
		CreatedAt:         domain.Now(),
	}
	if err := db.SaveRoom(t.Context(), room); err != nil {
		t.Fatalf("SaveRoom() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.DeleteRoom(context.Background(), room.ID); err != nil {
			t.Logf("cleaning up room %s: %v", room.ID, err)
		}
	})
	return room
}

// Running the schema twice must be a no-op; the service applies it on every
// boot.
func TestMigrateIsIdempotent(t *testing.T) {
	db := open(t)
	if err := db.Migrate(t.Context()); err != nil {
		t.Errorf("re-applying the schema: %v", err)
	}
}

func TestUserRoundTrip(t *testing.T) {
	db := open(t)
	user := newUser(t, db)

	for name, lookup := range map[string]func() (domain.User, error){
		"by id":       func() (domain.User, error) { return db.UserByID(t.Context(), user.ID) },
		"by email":    func() (domain.User, error) { return db.UserByEmail(t.Context(), user.Email) },
		"by username": func() (domain.User, error) { return db.UserByUsername(t.Context(), user.Username) },
	} {
		t.Run(name, func(t *testing.T) {
			got, err := lookup()
			if err != nil {
				t.Fatalf("lookup error = %v", err)
			}
			if got.ID != user.ID || got.Email != user.Email || got.Username != user.Username {
				t.Errorf("got %+v, want %+v", got, user)
			}
			if got.PasswordHash != user.PasswordHash {
				t.Error("the stored password hash did not round-trip")
			}
		})
	}

	if _, err := db.UserByID(t.Context(), "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("UserByID(missing) error = %v, want ErrNotFound", err)
	}
}

// The unique constraints are what actually stop two people claiming the same
// address, since the pre-flight check in the service can be raced.
func TestCreateUserRejectsDuplicates(t *testing.T) {
	db := open(t)
	user := newUser(t, db)

	duplicateEmail := domain.User{
		ID: ids.Short(), Email: user.Email, Username: "other-" + ids.Short(),
		PasswordHash: "x", CreatedAt: domain.Now(),
	}
	if err := db.CreateUser(t.Context(), duplicateEmail); !errors.Is(err, store.ErrConflict) {
		t.Errorf("duplicate email error = %v, want ErrConflict", err)
	}

	duplicateName := domain.User{
		ID: ids.Short(), Email: ids.Short() + "@example.com", Username: user.Username,
		PasswordHash: "x", CreatedAt: domain.Now(),
	}
	if err := db.CreateUser(t.Context(), duplicateName); !errors.Is(err, store.ErrConflict) {
		t.Errorf("duplicate username error = %v, want ErrConflict", err)
	}
}

func TestUpdateProfileLeavesOmittedFieldsAlone(t *testing.T) {
	db := open(t)
	user := newUser(t, db)

	bio := "I like music"
	if _, err := db.UpdateProfile(t.Context(), user.ID, &bio, nil); err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}

	avatar := "https://img.example/a.png"
	updated, err := db.UpdateProfile(t.Context(), user.ID, nil, &avatar)
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if updated.Bio != bio {
		t.Errorf("bio = %q, want it preserved by an avatar-only update", updated.Bio)
	}
	if updated.AvatarURL != avatar {
		t.Errorf("avatar_url = %q, want %q", updated.AvatarURL, avatar)
	}

	if _, err := db.UpdateProfile(t.Context(), "missing", &bio, nil); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("UpdateProfile(missing) error = %v, want ErrNotFound", err)
	}
}

func TestRoomRoundTrip(t *testing.T) {
	db := open(t)
	user := newUser(t, db)
	room := newRoom(t, db, user.ID)

	loaded, err := db.LoadRoom(t.Context(), room.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if loaded.Room.Name != room.Name || loaded.Room.OwnerID != user.ID {
		t.Errorf("room = %+v, want %+v", loaded.Room, room)
	}
	if !loaded.Room.AllowAnonymousAdd || loaded.Room.IsPrivate || loaded.Room.AutoRadio {
		t.Errorf("default flags = %+v", loaded.Room)
	}
	if loaded.Room.PasswordHash != "" {
		t.Errorf("password_hash = %q, want empty for an open room", loaded.Room.PasswordHash)
	}

	if _, err := db.LoadRoom(t.Context(), "NOPE00"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("LoadRoom(missing) error = %v, want ErrNotFound", err)
	}
}

// Saving a room again updates its settings without disturbing its ownership.
func TestSaveRoomUpdatesSettings(t *testing.T) {
	db := open(t)
	user := newUser(t, db)
	room := newRoom(t, db, user.ID)

	room.Name = "Renamed"
	room.IsPrivate = true
	room.AutoRadio = true
	room.AllowAnonymousAdd = false
	room.PasswordHash = "$2b$04$hash"
	if err := db.SaveRoom(t.Context(), room); err != nil {
		t.Fatalf("SaveRoom() error = %v", err)
	}

	loaded, err := db.LoadRoom(t.Context(), room.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if loaded.Room.Name != "Renamed" || !loaded.Room.IsPrivate || !loaded.Room.AutoRadio {
		t.Errorf("room = %+v", loaded.Room)
	}
	if loaded.Room.AllowAnonymousAdd {
		t.Error("allow_anonymous_add was not updated")
	}
	if loaded.Room.PasswordHash != "$2b$04$hash" {
		t.Errorf("password_hash = %q", loaded.Room.PasswordHash)
	}
	if loaded.Room.OwnerID != user.ID {
		t.Errorf("owner_id = %q, want it unchanged", loaded.Room.OwnerID)
	}
}

// Queue order is the whole point of a queue, so it has to survive the round
// trip exactly.
func TestReplaceTracksPreservesOrder(t *testing.T) {
	db := open(t)
	user := newUser(t, db)
	room := newRoom(t, db, user.ID)

	tracks := []*domain.Track{
		{ID: ids.Short(), Title: "First", Artist: "A", URL: "http://1", Duration: 100,
			AddedBy: "Alice", SourceURL: "https://youtu.be/aaaaaaaaaaa", StreamExpiresAt: 111, AddedAt: 1},
		{ID: ids.Short(), Title: "Second", Artist: "B", URL: "http://2", Duration: 200,
			AddedBy: "Bob", SourceURL: "https://youtu.be/bbbbbbbbbbb", StreamExpiresAt: 222, AddedAt: 2},
		{ID: ids.Short(), Title: "Third", Artist: "C", URL: "http://3", Duration: 300,
			AddedBy: "\U0001F4FB Radio", SourceURL: "https://youtu.be/ccccccccccc", StreamExpiresAt: 333, AddedAt: 3},
	}
	if err := db.ReplaceTracks(t.Context(), room.ID, tracks); err != nil {
		t.Fatalf("ReplaceTracks() error = %v", err)
	}

	loaded, err := db.LoadRoom(t.Context(), room.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(loaded.Tracks) != len(tracks) {
		t.Fatalf("loaded %d tracks, want %d", len(loaded.Tracks), len(tracks))
	}
	for i, want := range tracks {
		got := loaded.Tracks[i]
		if got.ID != want.ID || got.Title != want.Title || got.Duration != want.Duration {
			t.Errorf("track %d = %+v, want %+v", i, got, want)
		}
		if got.SourceURL != want.SourceURL || got.StreamExpiresAt != want.StreamExpiresAt {
			t.Errorf("track %d lost its stream metadata: %+v", i, got)
		}
		if got.AddedBy != want.AddedBy {
			t.Errorf("track %d added_by = %q, want %q", i, got.AddedBy, want.AddedBy)
		}
	}

	// Replacing with a shorter queue must remove what is no longer there.
	if err := db.ReplaceTracks(t.Context(), room.ID, tracks[:1]); err != nil {
		t.Fatalf("ReplaceTracks() error = %v", err)
	}
	loaded, err = db.LoadRoom(t.Context(), room.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(loaded.Tracks) != 1 {
		t.Errorf("loaded %d tracks, want 1", len(loaded.Tracks))
	}

	if err := db.ReplaceTracks(t.Context(), room.ID, nil); err != nil {
		t.Fatalf("ReplaceTracks(nil) error = %v", err)
	}
	loaded, err = db.LoadRoom(t.Context(), room.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(loaded.Tracks) != 0 {
		t.Errorf("loaded %d tracks, want none", len(loaded.Tracks))
	}
}

func TestMessagesAreStoredInOrder(t *testing.T) {
	db := open(t)
	user := newUser(t, db)
	room := newRoom(t, db, user.ID)

	for i, text := range []string{"first", "second", "third"} {
		msg := domain.ChatMessage{
			ID: ids.Short(), UserID: user.ID, Username: "Alice",
			Text: text, CreatedAt: float64(i + 1),
		}
		if err := db.AppendMessage(t.Context(), room.ID, msg); err != nil {
			t.Fatalf("AppendMessage() error = %v", err)
		}
	}

	loaded, err := db.LoadRoom(t.Context(), room.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(loaded.Messages) != 3 {
		t.Fatalf("loaded %d messages, want 3", len(loaded.Messages))
	}
	for i, want := range []string{"first", "second", "third"} {
		if loaded.Messages[i].Text != want {
			t.Errorf("message %d = %q, want %q", i, loaded.Messages[i].Text, want)
		}
	}
}

func TestReplaceVotes(t *testing.T) {
	db := open(t)
	user := newUser(t, db)
	room := newRoom(t, db, user.ID)

	votes := []domain.TrackVote{
		{UserID: "u1", TrackID: "t1", Vote: 1},
		{UserID: "u2", TrackID: "t1", Vote: -1},
	}
	if err := db.ReplaceVotes(t.Context(), room.ID, votes); err != nil {
		t.Fatalf("ReplaceVotes() error = %v", err)
	}

	loaded, err := db.LoadRoom(t.Context(), room.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(loaded.Votes) != 2 {
		t.Fatalf("loaded %d votes, want 2", len(loaded.Votes))
	}

	if err := db.ReplaceVotes(t.Context(), room.ID, nil); err != nil {
		t.Fatalf("ReplaceVotes(nil) error = %v", err)
	}
	loaded, err = db.LoadRoom(t.Context(), room.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(loaded.Votes) != 0 {
		t.Errorf("loaded %d votes, want none", len(loaded.Votes))
	}
}

// Deleting a room must take its queue, chat and votes with it; leaving orphans
// behind would make the tables grow without bound.
func TestDeleteRoomRemovesEverything(t *testing.T) {
	db := open(t)
	user := newUser(t, db)
	room := newRoom(t, db, user.ID)

	if err := db.ReplaceTracks(t.Context(), room.ID, []*domain.Track{{ID: ids.Short(), Title: "T"}}); err != nil {
		t.Fatalf("ReplaceTracks() error = %v", err)
	}
	if err := db.AppendMessage(t.Context(), room.ID, domain.ChatMessage{ID: ids.Short(), Text: "hi"}); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}
	if err := db.ReplaceVotes(t.Context(), room.ID, []domain.TrackVote{{UserID: "u", TrackID: "t", Vote: 1}}); err != nil {
		t.Fatalf("ReplaceVotes() error = %v", err)
	}

	if err := db.DeleteRoom(t.Context(), room.ID); err != nil {
		t.Fatalf("DeleteRoom() error = %v", err)
	}
	if _, err := db.LoadRoom(t.Context(), room.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("LoadRoom() after delete = %v, want ErrNotFound", err)
	}

	// Deleting a room that is already gone must not be an error; the service
	// can race two deletes.
	if err := db.DeleteRoom(t.Context(), room.ID); err != nil {
		t.Errorf("DeleteRoom() on a missing room = %v, want nil", err)
	}
}

func TestListPublicRooms(t *testing.T) {
	db := open(t)
	user := newUser(t, db)

	open1 := newRoom(t, db, user.ID)
	if err := db.ReplaceTracks(t.Context(), open1.ID, []*domain.Track{
		{ID: ids.Short(), Title: "One"}, {ID: ids.Short(), Title: "Two"},
	}); err != nil {
		t.Fatalf("ReplaceTracks() error = %v", err)
	}

	locked := newRoom(t, db, user.ID)
	locked.PasswordHash = "$2b$04$hash"
	if err := db.SaveRoom(t.Context(), locked); err != nil {
		t.Fatalf("SaveRoom() error = %v", err)
	}

	private := newRoom(t, db, user.ID)
	private.IsPrivate = true
	if err := db.SaveRoom(t.Context(), private); err != nil {
		t.Fatalf("SaveRoom() error = %v", err)
	}

	rooms, err := db.ListPublicRooms(t.Context())
	if err != nil {
		t.Fatalf("ListPublicRooms() error = %v", err)
	}

	byID := make(map[string]store.PublicRoom, len(rooms))
	for _, r := range rooms {
		byID[r.ID] = r
	}
	if got, ok := byID[open1.ID]; !ok || got.TrackCount != 2 {
		t.Errorf("open room = %+v, want a track count of 2", got)
	}
	if got, ok := byID[locked.ID]; !ok || !got.HasPassword {
		t.Errorf("locked room = %+v, want has_password", got)
	}
	if _, listed := byID[private.ID]; listed {
		t.Error("a private room appeared in the public listing")
	}
}

func TestPing(t *testing.T) {
	if err := open(t).Ping(t.Context()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

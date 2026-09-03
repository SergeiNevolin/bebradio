package room

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/leenzstra/bebradio/backend/internal/auth"
	"github.com/leenzstra/bebradio/backend/internal/config"
	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/hub"
	"github.com/leenzstra/bebradio/backend/internal/store"
	"github.com/leenzstra/bebradio/backend/internal/store/memory"
	"github.com/leenzstra/bebradio/backend/internal/youtube"
	"github.com/leenzstra/bebradio/backend/internal/youtube/youtubetest"
)

// fixture is a service wired to in-memory collaborators.
type fixture struct {
	svc   *Service
	store *memory.Store
	yt    *youtubetest.Fake
	cfg   config.Config
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	cfg := testConfig()
	st := memory.New()
	yt := youtubetest.New()

	svc := New(Deps{
		Store:     st,
		Hub:       hub.New(discardLogger()),
		YouTube:   yt,
		Tokens:    auth.NewTokens("test-secret", time.Hour),
		Passwords: auth.NewPasswords(4),
		Config:    cfg,
		Logger:    discardLogger(),
	})
	t.Cleanup(svc.Shutdown)

	return &fixture{svc: svc, store: st, yt: yt, cfg: cfg}
}

func testConfig() config.Config {
	return config.Config{
		MaxChatMessages:     100,
		MaxChatTextLen:      2000,
		StreamRefreshMargin: 600 * time.Second,
		AutoAdvanceInterval: 20 * time.Millisecond,
		AutoAdvanceGrace:    2500 * time.Millisecond,
		AdvanceDedupWindow:  time.Second,
		ReactionEmojis:      config.DefaultReactionEmojis,
		RadioRefillAt:       1,
		RadioBatch:          3,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// createRoom makes a room and returns its live entry, so a test can inspect and
// mutate state the public API does not expose.
func (f *fixture) createRoom(t *testing.T, ownerID string) *entry {
	t.Helper()
	created, err := f.svc.Create(t.Context(), "Test Room", ownerID, "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	e, ok := f.svc.peek(created.ID)
	if !ok {
		t.Fatalf("room %s is not in the registry after creation", created.ID)
	}
	return e
}

// --- Access control ---

func TestOpenRoomIsAccessibleToAnyone(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	snap, locked, err := f.svc.View(t.Context(), e.state.ID, Caller{})
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
	if locked != nil {
		t.Fatalf("an open room reported as locked: %+v", locked)
	}
	if snap.ID != e.state.ID {
		t.Errorf("View() returned room %q, want %q", snap.ID, e.state.ID)
	}
}

func TestLockedRoomHidesEverythingButItsName(t *testing.T) {
	f := newFixture(t)
	created, err := f.svc.Create(t.Context(), "Secret", "owner", "hunter2")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, locked, err := f.svc.View(t.Context(), created.ID, Caller{})
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
	if locked == nil {
		t.Fatal("a password-protected room was returned in full to a stranger")
	}
	if locked.Name != "Secret" || !locked.Locked || !locked.HasPassword {
		t.Errorf("locked payload = %+v", locked)
	}
}

func TestOwnerBypassesTheRoomPassword(t *testing.T) {
	f := newFixture(t)
	created, err := f.svc.Create(t.Context(), "Secret", "owner", "hunter2")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	snap, locked, err := f.svc.View(t.Context(), created.ID, Caller{UserID: "owner"})
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
	if locked != nil {
		t.Fatal("the owner was locked out of their own room")
	}
	// The owner is handed an access token so a fresh browser can open the room
	// without retyping the password.
	if snap.Access == "" {
		t.Error("the owner was not issued an access token")
	}
}

func TestJoinIssuesAccessOnlyForTheCorrectPassword(t *testing.T) {
	f := newFixture(t)
	created, err := f.svc.Create(t.Context(), "Secret", "owner", "hunter2")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, _, err := f.svc.Join(t.Context(), created.ID, "wrong"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("Join(wrong) error = %v, want ErrWrongPassword", err)
	}

	_, access, err := f.svc.Join(t.Context(), created.ID, "hunter2")
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}

	_, locked, err := f.svc.View(t.Context(), created.ID, Caller{AccessToken: access})
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
	if locked != nil {
		t.Error("a valid access token did not unlock the room")
	}
}

// An access token names one room; it must not open any other.
func TestAccessTokenIsScopedToOneRoom(t *testing.T) {
	f := newFixture(t)
	first, err := f.svc.Create(t.Context(), "A", "owner", "p")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := f.svc.Create(t.Context(), "B", "owner", "p")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, locked, err := f.svc.View(t.Context(), second.ID, Caller{AccessToken: first.Access})
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
	if locked == nil {
		t.Error("one room's access token opened another room")
	}
}

func TestRoomIDLookupIsCaseInsensitive(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	lower := lowercase(e.state.ID)
	snap, _, err := f.svc.View(t.Context(), lower, Caller{})
	if err != nil {
		t.Fatalf("View(%q) error = %v", lower, err)
	}
	if snap.ID != e.state.ID {
		t.Errorf("View(%q) = %q, want %q", lower, snap.ID, e.state.ID)
	}
}

func TestUnknownRoomIsNotFound(t *testing.T) {
	f := newFixture(t)
	if _, _, err := f.svc.View(t.Context(), "XXXXXX", Caller{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("View(unknown) error = %v, want ErrNotFound", err)
	}
}

// --- Settings ---

func TestOnlyTheOwnerMayChangeSettings(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	private := true
	if _, err := f.svc.UpdateSettings(t.Context(), e.state.ID, Caller{UserID: "someone-else"},
		SettingsUpdate{IsPrivate: &private}); !errors.Is(err, ErrNotOwner) {
		t.Errorf("UpdateSettings(stranger) error = %v, want ErrNotOwner", err)
	}
	if _, err := f.svc.UpdateSettings(t.Context(), e.state.ID, Caller{},
		SettingsUpdate{IsPrivate: &private}); !errors.Is(err, ErrNotOwner) {
		t.Errorf("UpdateSettings(anonymous) error = %v, want ErrNotOwner", err)
	}
}

// Changing one setting must not disturb the others -- in particular, a room's
// password must survive an unrelated update.
func TestSettingsUpdateIsPartial(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	owner := Caller{UserID: "owner"}

	password := "secret"
	if _, err := f.svc.UpdateSettings(t.Context(), e.state.ID, owner, SettingsUpdate{Password: &password}); err != nil {
		t.Fatalf("setting a password: %v", err)
	}

	private := true
	updated, err := f.svc.UpdateSettings(t.Context(), e.state.ID, owner, SettingsUpdate{IsPrivate: &private})
	if err != nil {
		t.Fatalf("making the room private: %v", err)
	}
	if !updated.HasPassword {
		t.Error("an unrelated settings change dropped the room password")
	}
	if !updated.IsPrivate {
		t.Error("is_private was not applied")
	}

	empty := ""
	updated, err = f.svc.UpdateSettings(t.Context(), e.state.ID, owner, SettingsUpdate{Password: &empty})
	if err != nil {
		t.Fatalf("removing the password: %v", err)
	}
	if updated.HasPassword {
		t.Error("an empty password should remove the room password")
	}
}

func TestDeleteRequiresOwnership(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	roomID := e.state.ID

	if err := f.svc.Delete(t.Context(), roomID, Caller{UserID: "stranger"}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("Delete(stranger) error = %v, want ErrNotOwner", err)
	}
	if err := f.svc.Delete(t.Context(), roomID, Caller{UserID: "owner"}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := f.svc.View(t.Context(), roomID, Caller{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("the room survived deletion: %v", err)
	}
}

// --- Queue ---

func TestAddTrackStartsPlaybackAndPersists(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	track, err := f.svc.AddTrack(t.Context(), e.state.ID,
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ", Caller{UserID: "u1", Username: "Alice"}, "")
	if err != nil {
		t.Fatalf("AddTrack() error = %v", err)
	}
	if track.AddedBy != "Alice" {
		t.Errorf("added_by = %q, want Alice", track.AddedBy)
	}

	e.mu.Lock()
	playing, queued := e.state.IsPlaying, len(e.state.Queue)
	e.mu.Unlock()
	if !playing || queued != 1 {
		t.Errorf("after the first track: playing=%v queued=%d", playing, queued)
	}

	contents, err := f.store.LoadRoom(t.Context(), e.state.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(contents.Tracks) != 1 {
		t.Errorf("stored %d tracks, want 1", len(contents.Tracks))
	}
}

func TestAddTrackCreditsAnonymousCallers(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	track, err := f.svc.AddTrack(t.Context(), e.state.ID,
		"https://youtu.be/dQw4w9WgXcQ", Caller{}, "Guest")
	if err != nil {
		t.Fatalf("AddTrack() error = %v", err)
	}
	if track.AddedBy != "Guest" {
		t.Errorf("added_by = %q, want Guest", track.AddedBy)
	}
}

func TestAddTrackRespectsTheAnonymousSetting(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	deny := false
	if _, err := f.svc.UpdateSettings(t.Context(), e.state.ID, Caller{UserID: "owner"},
		SettingsUpdate{AllowAnonymousAdd: &deny}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	_, err := f.svc.AddTrack(t.Context(), e.state.ID, "https://youtu.be/x", Caller{}, "Guest")
	if !errors.Is(err, ErrAnonymousAddDenied) {
		t.Errorf("AddTrack(anonymous) error = %v, want ErrAnonymousAddDenied", err)
	}
}

func TestAddTrackReportsAFailedLookup(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	f.yt.FetchTrackFn = func(context.Context, string) (youtube.TrackInfo, error) {
		return youtube.TrackInfo{}, errors.New("yt-dlp exploded")
	}

	if _, err := f.svc.AddTrack(t.Context(), e.state.ID, "https://youtu.be/x", Caller{}, ""); !errors.Is(err, ErrLookupFailed) {
		t.Errorf("AddTrack() error = %v, want ErrLookupFailed", err)
	}
}

// A track whose watch URL yt-dlp did not echo back still needs one, or its
// stream could never be re-resolved once it expires.
func TestAddTrackFallsBackToTheSubmittedURL(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	f.yt.FetchTrackFn = func(context.Context, string) (youtube.TrackInfo, error) {
		return youtube.TrackInfo{StreamURL: "https://stream", ExpiresAt: youtubetest.FarFuture}, nil
	}

	track, err := f.svc.AddTrack(t.Context(), e.state.ID, "https://youtu.be/abc", Caller{}, "")
	if err != nil {
		t.Fatalf("AddTrack() error = %v", err)
	}
	if track.SourceURL != "https://youtu.be/abc" {
		t.Errorf("source_url = %q, want the submitted URL", track.SourceURL)
	}
}

// --- Playback ---

func TestPlaybackPersistsTheQueue(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	e.mu.Lock()
	e.state.Queue = []*domain.Track{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	e.mu.Unlock()
	f.svc.persistTracks(t.Context(), e)

	if _, err := f.svc.Playback(t.Context(), e.state.ID, Caller{}, PlaybackCommand{Action: "next"}); err != nil {
		t.Fatalf("Playback() error = %v", err)
	}

	contents, err := f.store.LoadRoom(t.Context(), e.state.ID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(contents.Tracks) != 2 {
		t.Errorf("stored %d tracks after next, want 2", len(contents.Tracks))
	}
}

func TestPlaybackSeek(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	position := 42.5
	if _, err := f.svc.Playback(t.Context(), e.state.ID, Caller{},
		PlaybackCommand{Action: "seek", Position: &position}); err != nil {
		t.Fatalf("Playback() error = %v", err)
	}

	e.mu.Lock()
	got := e.state.Position
	e.mu.Unlock()
	if got != 42.5 {
		t.Errorf("position = %v, want 42.5", got)
	}
}

func TestPlaybackIsRefusedOnALockedRoom(t *testing.T) {
	f := newFixture(t)
	created, err := f.svc.Create(t.Context(), "Secret", "owner", "hunter2")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = f.svc.Playback(t.Context(), created.ID, Caller{}, PlaybackCommand{Action: "next"})
	if !errors.Is(err, ErrLocked) {
		t.Errorf("Playback(locked) error = %v, want ErrLocked", err)
	}
}

// --- Stream refresh ---

func TestRefreshReplacesAnExpiredStreamURL(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	e.mu.Lock()
	e.state.Queue = []*domain.Track{
		{ID: "a", URL: "stale", SourceURL: "https://youtu.be/aaa", StreamExpiresAt: 1},
	}
	e.mu.Unlock()

	changed, err := f.svc.refreshTracks(t.Context(), e, currentOnly)
	if err != nil {
		t.Fatalf("refreshTracks() error = %v", err)
	}
	if !changed {
		t.Fatal("refreshTracks() = false, want true for an expired URL")
	}

	e.mu.Lock()
	got := e.state.Queue[0]
	e.mu.Unlock()
	if got.URL != "fresh:https://youtu.be/aaa" {
		t.Errorf("url = %q, want the refreshed stream", got.URL)
	}
	if got.StreamExpiresAt != youtubetest.FarFuture {
		t.Errorf("stream_expires_at = %v, want the new expiry", got.StreamExpiresAt)
	}
}

func TestRefreshLeavesLiveStreamsAlone(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	e.mu.Lock()
	e.state.Queue = []*domain.Track{
		{ID: "a", URL: "ok", SourceURL: "https://youtu.be/aaa", StreamExpiresAt: domain.Now() + 3600},
	}
	e.mu.Unlock()

	changed, err := f.svc.refreshTracks(t.Context(), e, currentOnly)
	if err != nil {
		t.Fatalf("refreshTracks() error = %v", err)
	}
	if changed {
		t.Error("a stream that is still valid was re-resolved")
	}
	if f.yt.CallCount("ResolveStream") != 0 {
		t.Error("refreshTracks() reached for the network unnecessarily")
	}
}

// A track added before source URLs were recorded cannot be re-resolved; it must
// be left as it is rather than blanked.
func TestRefreshIgnoresTracksWithoutASourceURL(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	e.mu.Lock()
	e.state.Queue = []*domain.Track{{ID: "a", URL: "legacy", SourceURL: "", StreamExpiresAt: 1}}
	e.mu.Unlock()

	changed, err := f.svc.refreshTracks(t.Context(), e, currentOnly)
	if err != nil {
		t.Fatalf("refreshTracks() error = %v", err)
	}
	if changed {
		t.Error("a track with no source URL was reported as refreshed")
	}
}

// Refreshing the next track as well is what makes the hand-off at the end of a
// song instant; the track after that can wait.
func TestRefreshAheadCoversCurrentAndNextOnly(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	e.mu.Lock()
	e.state.Queue = []*domain.Track{
		{ID: "a", URL: "old", SourceURL: "https://youtu.be/aaa", StreamExpiresAt: 1},
		{ID: "b", URL: "old", SourceURL: "https://youtu.be/bbb", StreamExpiresAt: 1},
		{ID: "c", URL: "old", SourceURL: "https://youtu.be/ccc", StreamExpiresAt: 1},
	}
	e.mu.Unlock()

	changed, err := f.svc.refreshTracks(t.Context(), e, currentAndNext)
	if err != nil {
		t.Fatalf("refreshTracks() error = %v", err)
	}
	if !changed {
		t.Fatal("refreshTracks() = false, want true")
	}

	e.mu.Lock()
	urls := []string{e.state.Queue[0].URL, e.state.Queue[1].URL, e.state.Queue[2].URL}
	e.mu.Unlock()

	if urls[0] != "fresh:https://youtu.be/aaa" || urls[1] != "fresh:https://youtu.be/bbb" {
		t.Errorf("current and next = %v, want both refreshed", urls[:2])
	}
	if urls[2] != "old" {
		t.Errorf("the track after next = %q, want it left alone", urls[2])
	}
}

// A failure on one track must not stop the others being refreshed.
func TestRefreshContinuesPastAFailure(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	f.yt.ResolveStreamFn = func(_ context.Context, source string) (youtube.StreamInfo, error) {
		if source == "https://youtu.be/aaa" {
			return youtube.StreamInfo{}, errors.New("unavailable")
		}
		return youtube.StreamInfo{StreamURL: "fresh", ExpiresAt: youtubetest.FarFuture}, nil
	}

	e.mu.Lock()
	e.state.Queue = []*domain.Track{
		{ID: "a", URL: "old", SourceURL: "https://youtu.be/aaa", StreamExpiresAt: 1},
		{ID: "b", URL: "old", SourceURL: "https://youtu.be/bbb", StreamExpiresAt: 1},
	}
	e.mu.Unlock()

	changed, err := f.svc.refreshTracks(t.Context(), e, currentAndNext)
	if err == nil {
		t.Error("refreshTracks() error = nil, want the failure reported")
	}
	if !changed {
		t.Error("refreshTracks() = false, want the successful track reported as changed")
	}

	e.mu.Lock()
	second := e.state.Queue[1].URL
	e.mu.Unlock()
	if second != "fresh" {
		t.Errorf("second track url = %q, want it refreshed despite the first failing", second)
	}
}

// --- Auto-radio ---

func TestNeedsRefillRequiresSettingSeedAndLowQueue(t *testing.T) {
	f := newFixture(t)
	r := domain.NewRoom("ABC123", "R", "owner")
	r.RadioSeedURL = "https://youtu.be/abc"

	if f.svc.needsRefill(r) {
		t.Error("a room with auto-radio off should not refill")
	}
	r.AutoRadio = true
	if !f.svc.needsRefill(r) {
		t.Error("an empty auto-radio room with a seed should refill")
	}
	r.Queue = []*domain.Track{{ID: "1"}, {ID: "2"}, {ID: "3"}}
	if f.svc.needsRefill(r) {
		t.Error("a room with a healthy queue should not refill")
	}
	r.Queue = nil
	r.RadioSeedURL = ""
	if f.svc.needsRefill(r) {
		t.Error("a room with nothing to seed from should not refill")
	}
}

func TestRefillAppendsRelatedTracks(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	f.yt.RelatedFn = func(context.Context, string, int) ([]string, error) {
		return []string{
			"https://www.youtube.com/watch?v=rel0000000a",
			"https://www.youtube.com/watch?v=rel0000000b",
		}, nil
	}

	e.mu.Lock()
	e.state.AutoRadio = true
	e.state.RadioSeedURL = "https://www.youtube.com/watch?v=seed000000a"
	e.state.RadioFilling = true // as maybeRefill would have claimed it
	e.mu.Unlock()

	f.svc.refill(t.Context(), e)

	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.state.Queue) != 2 {
		t.Fatalf("queue has %d tracks, want 2", len(e.state.Queue))
	}
	for _, track := range e.state.Queue {
		if track.AddedBy != RadioTag {
			t.Errorf("added_by = %q, want %q", track.AddedBy, RadioTag)
		}
	}
	if !e.state.IsPlaying {
		t.Error("a refilled room should start playing")
	}
	if e.state.RadioFilling {
		t.Error("the searching flag should be cleared once the refill ends")
	}
}

// A refill must never offer a track the room has already been given.
func TestRefillSkipsAlreadySeenVideos(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	f.yt.RelatedFn = func(context.Context, string, int) ([]string, error) {
		return []string{
			"https://www.youtube.com/watch?v=rel0000000a",
			"https://www.youtube.com/watch?v=rel0000000b",
		}, nil
	}

	e.mu.Lock()
	e.state.AutoRadio = true
	e.state.RadioSeedURL = "https://www.youtube.com/watch?v=seed000000a"
	e.state.RadioSeen["rel0000000a"] = struct{}{}
	e.state.RadioFilling = true
	e.mu.Unlock()

	f.svc.refill(t.Context(), e)

	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.state.Queue) != 1 {
		t.Fatalf("queue has %d tracks, want 1", len(e.state.Queue))
	}
	if got := e.state.Queue[0].SourceURL; got != "https://www.youtube.com/watch?v=rel0000000b" {
		t.Errorf("queued %q, want the unseen candidate", got)
	}
}

func TestMaybeRefillDoesNothingWhenNotDue(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	f.svc.maybeRefill(e)
	f.svc.Shutdown() // waits for any background work

	if f.yt.CallCount("FetchRelated") != 0 {
		t.Error("maybeRefill() searched for music in a room that did not need it")
	}
}

// Two callers arriving together must not both start a refill.
func TestMaybeRefillClaimsTheRoomExactlyOnce(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	// Buffered, so the refill goroutine can announce that it started without
	// waiting for the test to be ready to hear it.
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	f.yt.RelatedFn = func(context.Context, string, int) ([]string, error) {
		started <- struct{}{}
		<-release
		return nil, nil
	}

	e.mu.Lock()
	e.state.AutoRadio = true
	e.state.RadioSeedURL = "https://www.youtube.com/watch?v=seed000000a"
	e.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.svc.maybeRefill(e)
		}()
	}
	wg.Wait()

	<-started
	close(release)
	f.svc.Shutdown()

	if got := f.yt.CallCount("FetchRelated"); got != 1 {
		t.Errorf("FetchRelated called %d times, want exactly 1", got)
	}
}

// --- Registry ---

// Two callers asking for the same unloaded room must end up sharing one live
// state, or their changes would silently diverge.
func TestConcurrentLoadsShareOneRoom(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	roomID := e.state.ID
	f.svc.forget(roomID)

	const callers = 16
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		entries []*entry
	)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loaded, err := f.svc.getOrLoad(context.Background(), roomID)
			if err != nil {
				t.Errorf("getOrLoad() error = %v", err)
				return
			}
			mu.Lock()
			entries = append(entries, loaded)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(entries) != callers {
		t.Fatalf("got %d entries, want %d", len(entries), callers)
	}
	for _, got := range entries {
		if got != entries[0] {
			t.Fatal("concurrent loads produced more than one live room")
		}
	}
}

func TestForgottenRoomIsReloadedFromTheStore(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	roomID := e.state.ID

	if _, err := f.svc.AddTrack(t.Context(), roomID, "https://youtu.be/dQw4w9WgXcQ", Caller{}, "Guest"); err != nil {
		t.Fatalf("AddTrack() error = %v", err)
	}
	f.svc.forget(roomID)

	snap, _, err := f.svc.View(t.Context(), roomID, Caller{})
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
	if len(snap.Queue) != 1 {
		t.Errorf("reloaded queue has %d tracks, want 1", len(snap.Queue))
	}
}

func TestListPublicHidesPrivateRooms(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	rooms, err := f.svc.ListPublic(t.Context())
	if err != nil {
		t.Fatalf("ListPublic() error = %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("ListPublic() returned %d rooms, want 1", len(rooms))
	}

	private := true
	if _, err := f.svc.UpdateSettings(t.Context(), e.state.ID, Caller{UserID: "owner"},
		SettingsUpdate{IsPrivate: &private}); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	rooms, err = f.svc.ListPublic(t.Context())
	if err != nil {
		t.Fatalf("ListPublic() error = %v", err)
	}
	if len(rooms) != 0 {
		t.Errorf("ListPublic() returned %d rooms, want none", len(rooms))
	}
}

// --- Lyrics ---

func TestLyricsForAnEmptyRoom(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	lyrics, err := f.svc.LyricsFor(t.Context(), e.state.ID, "", Caller{})
	if err != nil {
		t.Fatalf("LyricsFor() error = %v", err)
	}
	if lyrics.Available || lyrics.TrackID != nil || len(lyrics.Cues) != 0 {
		t.Errorf("lyrics = %+v, want nothing available", lyrics)
	}
}

func TestLyricsForATrackWithoutASourceURL(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")

	e.mu.Lock()
	e.state.Queue = []*domain.Track{{ID: "x", SourceURL: ""}}
	e.mu.Unlock()

	lyrics, err := f.svc.LyricsFor(t.Context(), e.state.ID, "", Caller{})
	if err != nil {
		t.Fatalf("LyricsFor() error = %v", err)
	}
	if lyrics.Available || lyrics.TrackID == nil || *lyrics.TrackID != "x" {
		t.Errorf("lyrics = %+v, want the track named but no captions", lyrics)
	}
	if f.yt.CallCount("FetchSubtitles") != 0 {
		t.Error("a track with no source URL should not trigger a caption lookup")
	}
}

func TestLyricsReturnsCues(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	f.yt.SubtitlesFn = func(context.Context, string, string) (youtube.Subtitles, error) {
		return youtube.Subtitles{
			Lang: "en", Auto: true,
			Cues: []youtube.Cue{{Start: 1, Dur: 2, Text: "sing along"}},
		}, nil
	}

	e.mu.Lock()
	e.state.Queue = []*domain.Track{{ID: "a", SourceURL: "https://youtu.be/dQw4w9WgXcQ"}}
	e.mu.Unlock()

	lyrics, err := f.svc.LyricsFor(t.Context(), e.state.ID, "", Caller{})
	if err != nil {
		t.Fatalf("LyricsFor() error = %v", err)
	}
	if !lyrics.Available || !lyrics.Auto || lyrics.Lang != "en" {
		t.Errorf("lyrics = %+v", lyrics)
	}
	if len(lyrics.Cues) != 1 || lyrics.Cues[0].Text != "sing along" {
		t.Errorf("cues = %+v", lyrics.Cues)
	}
}

// A video with no captions is an ordinary outcome, not an error the caller
// should have to handle.
func TestLyricsSwallowsALookupFailure(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	f.yt.SubtitlesFn = func(context.Context, string, string) (youtube.Subtitles, error) {
		return youtube.Subtitles{}, errors.New("no captions")
	}

	e.mu.Lock()
	e.state.Queue = []*domain.Track{{ID: "a", SourceURL: "https://youtu.be/dQw4w9WgXcQ"}}
	e.mu.Unlock()

	lyrics, err := f.svc.LyricsFor(t.Context(), e.state.ID, "", Caller{})
	if err != nil {
		t.Fatalf("LyricsFor() error = %v", err)
	}
	if lyrics.Available || lyrics.Cues == nil {
		t.Errorf("lyrics = %+v, want an empty cue list", lyrics)
	}
}

// --- Store failures ---

// The queue in memory is what listeners are hearing; a failed write must be
// logged, not turned into a failed request.
func TestPlaybackSurvivesAStoreFailure(t *testing.T) {
	f := newFixture(t)
	e := f.createRoom(t, "owner")
	f.svc.store = failingStore{Store: f.store}

	e.mu.Lock()
	e.state.Queue = []*domain.Track{{ID: "a"}, {ID: "b"}}
	e.mu.Unlock()

	if _, err := f.svc.Playback(t.Context(), e.state.ID, Caller{}, PlaybackCommand{Action: "next"}); err != nil {
		t.Fatalf("Playback() error = %v, want the failure absorbed", err)
	}

	e.mu.Lock()
	remaining := len(e.state.Queue)
	e.mu.Unlock()
	if remaining != 1 {
		t.Errorf("queue has %d tracks, want the advance to have happened anyway", remaining)
	}
}

type failingStore struct {
	store.Store
}

func (failingStore) ReplaceTracks(context.Context, string, []*domain.Track) error {
	return errors.New("database is down")
}

func lowercase(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

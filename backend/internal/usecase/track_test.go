package usecase

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

func newTrackUC(t *testing.T) (*TrackUsecase, *repository.MockTrackRepo, *repository.MockMediaClient) {
	t.Helper()
	repo := repository.NewMockTrackRepo()
	media := repository.NewMockMediaClient()
	cfg := &config.Config{MashupUserQuota: 3}
	return NewTrackUsecase(repo, media, cfg, testLog2), repo, media
}

func TestTrackCreateSuccess(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	uploaded := ""
	media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		io.Copy(io.Discard, body)
		uploaded = filename
		return map[string]any{"id": "t1", "media_id": "m1", "status": "processing"}, nil
	}

	m, err := uc.Create("owner1", "My Bootleg", "DJ Earworm", "mix.mp3", strings.NewReader("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status != "processing" {
		t.Errorf("expected processing, got %q", m.Status)
	}
	// Identity arrives with the data: row id and media id come from music-service.
	if m.ID != "t1" || m.MediaID != "m1" {
		t.Errorf("expected ids from upload response, got id=%q media=%q", m.ID, m.MediaID)
	}
	if uploaded != "mix.mp3" {
		t.Errorf("expected filename forwarded, got %q", uploaded)
	}
	if _, err := repo.FindByID(m.ID); err != nil {
		t.Errorf("track row not persisted: %v", err)
	}
}

func TestTrackCreateQuotaExceeded(t *testing.T) {
	uc, repo, _ := newTrackUC(t)
	for i := 0; i < 3; i++ {
		repo.Create(&entity.Track{ID: string(rune('a' + i)), OwnerID: "owner1"})
	}

	_, err := uc.Create("owner1", "x", "", "x.mp3", strings.NewReader("d"))
	if !errors.Is(err, ErrTrackQuota) {
		t.Fatalf("expected ErrTrackQuota, got %v", err)
	}
}

func TestTrackCreateUploadFailureLeavesNoRow(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		return nil, errors.New("media down")
	}

	m, err := uc.Create("owner1", "x", "", "x.mp3", strings.NewReader("d"))
	if !errors.Is(err, ErrTrackUploadFail) {
		t.Fatalf("expected ErrTrackUploadFail, got %v", err)
	}
	if m != nil {
		t.Errorf("expected no track on upload failure, got %v", m.ID)
	}
	if n, _ := repo.CountByOwner("owner1"); n != 0 {
		t.Errorf("expected no row persisted, got %d", n)
	}
}

func TestTrackCreateMissingIdentity(t *testing.T) {
	uc, _, media := newTrackUC(t)
	media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		return map[string]any{"status": "processing"}, nil
	}

	_, err := uc.Create("owner1", "x", "", "x.mp3", strings.NewReader("d"))
	if !errors.Is(err, ErrTrackUploadFail) {
		t.Fatalf("expected ErrTrackUploadFail, got %v", err)
	}
}

func TestTrackCreateTitleFallsBackToFilename(t *testing.T) {
	uc, _, _ := newTrackUC(t)
	m, err := uc.Create("owner1", "   ", "", "Awesome Mix.mp3", strings.NewReader("d"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Awesome Mix" {
		t.Errorf("expected title from filename, got %q", m.Title)
	}
}

func TestTrackDeleteRequiresOwner(t *testing.T) {
	uc, repo, _ := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "owner1", MediaID: "abc", Status: "ready"})

	if err := uc.Delete("m1", "someone-else"); !errors.Is(err, ErrTrackNotOwner) {
		t.Fatalf("expected ErrTrackNotOwner, got %v", err)
	}
	if _, err := repo.FindByID("m1"); err != nil {
		t.Error("row should survive a rejected delete")
	}
}

func TestTrackDeleteNotFound(t *testing.T) {
	uc, _, _ := newTrackUC(t)
	if err := uc.Delete("nope", "owner1"); !errors.Is(err, ErrTrackNotFound) {
		t.Fatalf("expected ErrTrackNotFound, got %v", err)
	}
}

func TestTrackDeleteKeepsRowWhenMediaServiceFails(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "owner1", MediaID: "abc"})
	media.DeleteTrackFn = func(mediaID string) error { return errors.New("boom") }

	if err := uc.Delete("m1", "owner1"); !errors.Is(err, ErrTrackMediaDown) {
		t.Fatalf("expected ErrTrackMediaDown, got %v", err)
	}
	if _, err := repo.FindByID("m1"); err != nil {
		t.Error("row must not be deleted when the file could not be removed")
	}
}

func TestTrackDeleteSuccess(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "owner1", MediaID: "abc"})
	called := false
	media.DeleteTrackFn = func(mediaID string) error { called = true; return nil }

	if err := uc.Delete("m1", "owner1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected media client DeleteTrack to be called")
	}
	if _, err := repo.FindByID("m1"); err == nil {
		t.Error("row should be gone")
	}
}

func TestTrackPollTransitionsToReady(t *testing.T) {
	oldInterval, oldTimeout := trackPollInterval, trackPollTimeout
	trackPollInterval, trackPollTimeout = 2*time.Millisecond, 500*time.Millisecond
	defer func() { trackPollInterval, trackPollTimeout = oldInterval, oldTimeout }()

	uc, repo, media := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "processing"})
	media.TrackUploadStatusFn = func(mediaID string) (map[string]any, error) {
		return map[string]any{"status": "ready", "duration": float64(217), "has_cover": true}, nil
	}

	uc.poll("m1", "abc")

	m, _ := repo.FindByID("m1")
	if m.Status != "ready" || m.Duration != 217 || !m.HasCover {
		t.Errorf("unexpected row after poll: %+v", m)
	}
}

func TestTrackPollTransitionsToFailed(t *testing.T) {
	oldInterval, oldTimeout := trackPollInterval, trackPollTimeout
	trackPollInterval, trackPollTimeout = 2*time.Millisecond, 500*time.Millisecond
	defer func() { trackPollInterval, trackPollTimeout = oldInterval, oldTimeout }()

	uc, repo, media := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "processing"})
	media.TrackUploadStatusFn = func(mediaID string) (map[string]any, error) {
		return map[string]any{"status": "failed", "error": "no audio stream"}, nil
	}

	uc.poll("m1", "abc")

	m, _ := repo.FindByID("m1")
	if m.Status != "failed" || m.Error != "no audio stream" {
		t.Errorf("unexpected row after poll: %+v", m)
	}
}

func TestTrackLikeAndUnlike(t *testing.T) {
	uc, repo, _ := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "ready"})

	res, err := uc.Like("m1", "user-a")
	if err != nil {
		t.Fatalf("like: %v", err)
	}
	if res["likes"] != 1 || res["liked"] != true {
		t.Fatalf("unexpected like result: %v", res)
	}

	// Idempotent: a repeat like does not double-count.
	res, _ = uc.Like("m1", "user-a")
	if res["likes"] != 1 {
		t.Errorf("repeat like should stay at 1, got %v", res["likes"])
	}

	res, _ = uc.Like("m1", "user-b")
	if res["likes"] != 2 {
		t.Errorf("expected 2 likes, got %v", res["likes"])
	}

	res, err = uc.Unlike("m1", "user-a")
	if err != nil {
		t.Fatalf("unlike: %v", err)
	}
	if res["likes"] != 1 || res["liked"] != false {
		t.Fatalf("unexpected unlike result: %v", res)
	}

	liked, _ := uc.ListLiked("user-b", 0, 0)
	if len(liked) != 1 || liked[0]["id"] != "m1" {
		t.Errorf("user-b should have one liked track, got %v", liked)
	}
	if none, _ := uc.ListLiked("user-a", 0, 0); len(none) != 0 {
		t.Errorf("user-a should have no liked tracks, got %v", none)
	}
}

func TestTrackLikeNotFound(t *testing.T) {
	uc, _, _ := newTrackUC(t)
	if _, err := uc.Like("ghost", "user-a"); !errors.Is(err, ErrTrackNotFound) {
		t.Fatalf("expected ErrTrackNotFound, got %v", err)
	}
}

func TestTrackGetCarriesLikedFlag(t *testing.T) {
	uc, repo, _ := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "ready"})
	_, _ = uc.Like("m1", "user-a")

	m, err := uc.Get("m1", "user-a")
	if err != nil || !m.Liked {
		t.Fatalf("expected liked=true for user-a, got %+v (err %v)", m, err)
	}
	other, _ := uc.Get("m1", "user-b")
	if other.Liked {
		t.Errorf("expected liked=false for user-b")
	}
}

func TestTrackSetCoverRequiresOwner(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "owner1", MediaID: "abc", Status: "ready"})
	called := false
	media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		called = true
		return nil
	}

	if err := uc.SetCover("m1", "intruder", "c.jpg", strings.NewReader("img")); !errors.Is(err, ErrTrackNotOwner) {
		t.Fatalf("expected ErrTrackNotOwner, got %v", err)
	}
	if called {
		t.Error("media client must not be called for a non-owner")
	}
}

func TestTrackSetCoverSuccess(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "owner1", MediaID: "abc", Status: "ready"})
	var gotMedia string
	media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		io.Copy(io.Discard, body)
		gotMedia = mediaID
		return nil
	}

	if err := uc.SetCover("m1", "owner1", "c.jpg", strings.NewReader("img")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMedia != "abc" {
		t.Errorf("media client got media id %q, want abc", gotMedia)
	}
	if !repo.CoverSet["m1"] {
		t.Error("expected SetCoverUploaded to be recorded")
	}
}

func TestTrackSetCoverMediaFailure(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "owner1", MediaID: "abc", Status: "ready"})
	media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		return errors.New("media down")
	}

	if err := uc.SetCover("m1", "owner1", "c.jpg", strings.NewReader("img")); !errors.Is(err, ErrTrackCoverFail) {
		t.Fatalf("expected ErrTrackCoverFail, got %v", err)
	}
	if repo.CoverSet["m1"] {
		t.Error("cover flag must not be set when media service fails")
	}
}

func TestTrackPollTimesOut(t *testing.T) {
	oldInterval, oldTimeout := trackPollInterval, trackPollTimeout
	trackPollInterval, trackPollTimeout = 2*time.Millisecond, 20*time.Millisecond
	defer func() { trackPollInterval, trackPollTimeout = oldInterval, oldTimeout }()

	uc, repo, media := newTrackUC(t)
	repo.Create(&entity.Track{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "processing"})
	media.TrackUploadStatusFn = func(mediaID string) (map[string]any, error) {
		return map[string]any{"status": "processing"}, nil
	}

	uc.poll("m1", "abc")

	m, _ := repo.FindByID("m1")
	if m.Status != "failed" {
		t.Errorf("expected failed after timeout, got %q", m.Status)
	}
}



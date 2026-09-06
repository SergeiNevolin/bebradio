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

func newMashupUC(t *testing.T) (*MashupUsecase, *repository.MockMashupRepo, *repository.MockMediaClient) {
	t.Helper()
	repo := repository.NewMockMashupRepo()
	media := repository.NewMockMediaClient()
	cfg := &config.Config{MashupUserQuota: 3}
	return NewMashupUsecase(repo, media, cfg, testLog2), repo, media
}

func TestMashupCreateSuccess(t *testing.T) {
	uc, repo, media := newMashupUC(t)
	uploaded := ""
	media.UploadMashupFn = func(mediaID, filename string, body io.Reader) error {
		io.Copy(io.Discard, body)
		uploaded = mediaID
		return nil
	}

	m, err := uc.Create("owner1", "My Bootleg", "DJ Earworm", "mix.mp3", strings.NewReader("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status != "processing" {
		t.Errorf("expected processing, got %q", m.Status)
	}
	if uploaded != m.MediaID || m.MediaID == "" {
		t.Errorf("media client not called with mashup media id (%q vs %q)", uploaded, m.MediaID)
	}
	if _, err := repo.FindByID(m.ID); err != nil {
		t.Errorf("mashup row not persisted: %v", err)
	}
}

func TestMashupCreateQuotaExceeded(t *testing.T) {
	uc, repo, _ := newMashupUC(t)
	for i := 0; i < 3; i++ {
		repo.Create(&entity.Mashup{ID: string(rune('a' + i)), OwnerID: "owner1"})
	}

	_, err := uc.Create("owner1", "x", "", "x.mp3", strings.NewReader("d"))
	if !errors.Is(err, ErrMashupQuota) {
		t.Fatalf("expected ErrMashupQuota, got %v", err)
	}
}

func TestMashupCreateUploadFailureMarksFailed(t *testing.T) {
	uc, repo, media := newMashupUC(t)
	media.UploadMashupFn = func(mediaID, filename string, body io.Reader) error {
		return errors.New("media down")
	}

	m, err := uc.Create("owner1", "x", "", "x.mp3", strings.NewReader("d"))
	if !errors.Is(err, ErrMashupUploadFail) {
		t.Fatalf("expected ErrMashupUploadFail, got %v", err)
	}
	stored, _ := repo.FindByID(m.ID)
	if stored.Status != "failed" {
		t.Errorf("expected row marked failed, got %q", stored.Status)
	}
}

func TestMashupCreateTitleFallsBackToFilename(t *testing.T) {
	uc, _, _ := newMashupUC(t)
	m, err := uc.Create("owner1", "   ", "", "Awesome Mix.mp3", strings.NewReader("d"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Awesome Mix" {
		t.Errorf("expected title from filename, got %q", m.Title)
	}
}

func TestMashupDeleteRequiresOwner(t *testing.T) {
	uc, repo, _ := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "owner1", MediaID: "abc", Status: "ready"})

	if err := uc.Delete("m1", "someone-else"); !errors.Is(err, ErrMashupNotOwner) {
		t.Fatalf("expected ErrMashupNotOwner, got %v", err)
	}
	if _, err := repo.FindByID("m1"); err != nil {
		t.Error("row should survive a rejected delete")
	}
}

func TestMashupDeleteNotFound(t *testing.T) {
	uc, _, _ := newMashupUC(t)
	if err := uc.Delete("nope", "owner1"); !errors.Is(err, ErrMashupNotFound) {
		t.Fatalf("expected ErrMashupNotFound, got %v", err)
	}
}

func TestMashupDeleteKeepsRowWhenMediaServiceFails(t *testing.T) {
	uc, repo, media := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "owner1", MediaID: "abc"})
	media.DeleteMashupFn = func(mediaID string) error { return errors.New("boom") }

	if err := uc.Delete("m1", "owner1"); !errors.Is(err, ErrMashupMediaDown) {
		t.Fatalf("expected ErrMashupMediaDown, got %v", err)
	}
	if _, err := repo.FindByID("m1"); err != nil {
		t.Error("row must not be deleted when the file could not be removed")
	}
}

func TestMashupDeleteSuccess(t *testing.T) {
	uc, repo, media := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "owner1", MediaID: "abc"})
	called := false
	media.DeleteMashupFn = func(mediaID string) error { called = true; return nil }

	if err := uc.Delete("m1", "owner1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected media client DeleteMashup to be called")
	}
	if _, err := repo.FindByID("m1"); err == nil {
		t.Error("row should be gone")
	}
}

func TestMashupPollTransitionsToReady(t *testing.T) {
	oldInterval, oldTimeout := mashupPollInterval, mashupPollTimeout
	mashupPollInterval, mashupPollTimeout = 2*time.Millisecond, 500*time.Millisecond
	defer func() { mashupPollInterval, mashupPollTimeout = oldInterval, oldTimeout }()

	uc, repo, media := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "processing"})
	media.MashupStatusFn = func(mediaID string) (map[string]any, error) {
		return map[string]any{"status": "ready", "duration": float64(217), "has_cover": true}, nil
	}

	uc.poll("m1", "abc")

	m, _ := repo.FindByID("m1")
	if m.Status != "ready" || m.Duration != 217 || !m.HasCover {
		t.Errorf("unexpected row after poll: %+v", m)
	}
}

func TestMashupPollTransitionsToFailed(t *testing.T) {
	oldInterval, oldTimeout := mashupPollInterval, mashupPollTimeout
	mashupPollInterval, mashupPollTimeout = 2*time.Millisecond, 500*time.Millisecond
	defer func() { mashupPollInterval, mashupPollTimeout = oldInterval, oldTimeout }()

	uc, repo, media := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "processing"})
	media.MashupStatusFn = func(mediaID string) (map[string]any, error) {
		return map[string]any{"status": "failed", "error": "no audio stream"}, nil
	}

	uc.poll("m1", "abc")

	m, _ := repo.FindByID("m1")
	if m.Status != "failed" || m.Error != "no audio stream" {
		t.Errorf("unexpected row after poll: %+v", m)
	}
}

func TestMashupLikeAndUnlike(t *testing.T) {
	uc, repo, _ := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "ready"})

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
		t.Errorf("user-b should have one liked mashup, got %v", liked)
	}
	if none, _ := uc.ListLiked("user-a", 0, 0); len(none) != 0 {
		t.Errorf("user-a should have no liked mashups, got %v", none)
	}
}

func TestMashupLikeNotFound(t *testing.T) {
	uc, _, _ := newMashupUC(t)
	if _, err := uc.Like("ghost", "user-a"); !errors.Is(err, ErrMashupNotFound) {
		t.Fatalf("expected ErrMashupNotFound, got %v", err)
	}
}

func TestMashupGetCarriesLikedFlag(t *testing.T) {
	uc, repo, _ := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "ready"})
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

func TestMashupSetCoverRequiresOwner(t *testing.T) {
	uc, repo, media := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "owner1", MediaID: "abc", Status: "ready"})
	called := false
	media.UploadMashupCoverFn = func(mediaID, filename string, body io.Reader) error {
		called = true
		return nil
	}

	if err := uc.SetCover("m1", "intruder", "c.jpg", strings.NewReader("img")); !errors.Is(err, ErrMashupNotOwner) {
		t.Fatalf("expected ErrMashupNotOwner, got %v", err)
	}
	if called {
		t.Error("media client must not be called for a non-owner")
	}
}

func TestMashupSetCoverSuccess(t *testing.T) {
	uc, repo, media := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "owner1", MediaID: "abc", Status: "ready"})
	var gotMedia string
	media.UploadMashupCoverFn = func(mediaID, filename string, body io.Reader) error {
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

func TestMashupSetCoverMediaFailure(t *testing.T) {
	uc, repo, media := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "owner1", MediaID: "abc", Status: "ready"})
	media.UploadMashupCoverFn = func(mediaID, filename string, body io.Reader) error {
		return errors.New("media down")
	}

	if err := uc.SetCover("m1", "owner1", "c.jpg", strings.NewReader("img")); !errors.Is(err, ErrMashupCoverFail) {
		t.Fatalf("expected ErrMashupCoverFail, got %v", err)
	}
	if repo.CoverSet["m1"] {
		t.Error("cover flag must not be set when media service fails")
	}
}

func TestMashupPollTimesOut(t *testing.T) {
	oldInterval, oldTimeout := mashupPollInterval, mashupPollTimeout
	mashupPollInterval, mashupPollTimeout = 2*time.Millisecond, 20*time.Millisecond
	defer func() { mashupPollInterval, mashupPollTimeout = oldInterval, oldTimeout }()

	uc, repo, media := newMashupUC(t)
	repo.Create(&entity.Mashup{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "processing"})
	media.MashupStatusFn = func(mediaID string) (map[string]any, error) {
		return map[string]any{"status": "processing"}, nil
	}

	uc.poll("m1", "abc")

	m, _ := repo.FindByID("m1")
	if m.Status != "failed" {
		t.Errorf("expected failed after timeout, got %q", m.Status)
	}
}

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

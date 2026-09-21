package usecase

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bebradio/backend-go/internal/domain/entity"
)

func TestTrackImportSuccess(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		data, _ := io.ReadAll(body)
		if filename != "queue-yt1.m4a" {
			t.Errorf("expected queue-based filename, got %q", filename)
		}
		if string(data) != "cached-audio" {
			t.Errorf("expected cached audio forwarded, got %q", data)
		}
		return map[string]any{"id": "lib9", "media_id": "mm9", "status": "processing"}, nil
	}
	media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		if mediaID != "mm9" {
			t.Errorf("expected cover on new media id, got %q", mediaID)
		}
		return nil
	}

	m, err := uc.Import("admin1", "Clip", "Channel", "queue-yt1.m4a",
		strings.NewReader("cached-audio"), "cover.jpg", strings.NewReader("img"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Source != entity.TrackSourceUpload || m.Status != entity.TrackStatusProcessing {
		t.Errorf("expected processing upload row, got %+v", m)
	}
	if m.OwnerID != "admin1" || m.Title != "Clip" {
		t.Errorf("expected ownership/metadata carried over, got %+v", m)
	}
	if _, err := repo.FindByID("lib9"); err != nil {
		t.Error("imported row not persisted")
	}
	if !repo.CoverSet["lib9"] {
		t.Error("expected cover flag persisted")
	}
}

func TestTrackImportCoverFailureStillSucceeds(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		io.Copy(io.Discard, body)
		return map[string]any{"id": "lib9", "media_id": "mm9", "status": "processing"}, nil
	}
	media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		return errors.New("cover down")
	}

	m, err := uc.Import("admin1", "Clip", "Channel", "queue-yt1.m4a",
		strings.NewReader("cached-audio"), "cover.jpg", strings.NewReader("img"))
	if err != nil {
		t.Fatalf("cover failure must not fail the import, got %v", err)
	}
	if _, err := repo.FindByID(m.ID); err != nil {
		t.Error("imported row not persisted")
	}
	if repo.CoverSet["lib9"] {
		t.Error("failed cover must not flip the cover flag")
	}
}

func TestTrackImportWithoutCover(t *testing.T) {
	uc, _, media := newTrackUC(t)
	coverCalled := false
	media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		io.Copy(io.Discard, body)
		return map[string]any{"id": "lib9", "media_id": "mm9", "status": "processing"}, nil
	}
	media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		coverCalled = true
		return nil
	}

	if _, err := uc.Import("admin1", "Clip", "Channel", "queue-yt1.m4a",
		strings.NewReader("cached-audio"), "", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if coverCalled {
		t.Error("nil cover must skip the cover upload")
	}
}

func TestTrackImportQuotaExceeded(t *testing.T) {
	uc, _, _ := newTrackUC(t)
	for i := 0; i < 3; i++ {
		_ = uc.repo.Create(&entity.Track{ID: string(rune('a' + i)), OwnerID: "admin1"})
	}

	_, err := uc.Import("admin1", "x", "", "x.m4a", strings.NewReader("d"), "", nil)
	if !errors.Is(err, ErrTrackQuota) {
		t.Fatalf("expected ErrTrackQuota, got %v", err)
	}
}

func TestTrackImportUploadFailureLeavesNoRow(t *testing.T) {
	uc, repo, media := newTrackUC(t)
	media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		return nil, errors.New("media down")
	}

	_, err := uc.Import("admin1", "x", "", "x.m4a", strings.NewReader("d"), "", nil)
	if !errors.Is(err, ErrTrackUploadFail) {
		t.Fatalf("expected ErrTrackUploadFail, got %v", err)
	}
	if n, _ := repo.CountByOwner("admin1"); n != 0 {
		t.Errorf("expected no row persisted, got %d", n)
	}
}

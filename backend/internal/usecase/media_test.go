package usecase

import (
	"testing"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

func TestEnsureTrackReadyNil(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	if uc.EnsureTrackReady(nil) {
		t.Error("expected false for nil track")
	}
}

func TestEnsureTrackReadyNoMediaID(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	track := &entity.Track{ID: "t1"}
	if uc.EnsureTrackReady(track) {
		t.Error("expected false for track without media_id")
	}
}

func TestEnsureTrackReadySuccess(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.EnsureFn = func(items []map[string]any) ([]string, error) {
		return []string{"media123"}, nil
	}
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	track := &entity.Track{ID: "t1", MediaID: "media123", SourceURL: "https://youtube.com/watch?v=abc"}
	result := uc.EnsureTrackReady(track)

	if !result {
		t.Error("expected true when track becomes ready")
	}
	if track.URL != "/api/media/media123" {
		t.Errorf("expected URL '/api/media/media123', got '%s'", track.URL)
	}
	if track.LocalPath != "media123.m4a" {
		t.Errorf("expected local_path 'media123.m4a', got '%s'", track.LocalPath)
	}
}

func TestEnsureTrackReadyAlreadyHasURL(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.EnsureFn = func(items []map[string]any) ([]string, error) {
		return []string{"media123"}, nil
	}
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	track := &entity.Track{ID: "t1", MediaID: "media123", URL: "/api/media/media123"}
	result := uc.EnsureTrackReady(track)

	if result {
		t.Error("expected false when track already has URL")
	}
}

func TestEnsureTrackReadyNotReady(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.EnsureFn = func(items []map[string]any) ([]string, error) {
		return []string{"other_media"}, nil
	}
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	track := &entity.Track{ID: "t1", MediaID: "media123", SourceURL: "url"}
	result := uc.EnsureTrackReady(track)

	if result {
		t.Error("expected false when media not ready")
	}
}

func TestEnsureRoomMediaNoTracks(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	rm := entity.NewRoom("X", "R", "O")
	if uc.EnsureRoomMedia(rm) {
		t.Error("expected false for empty queue")
	}
}

func TestEnsureRoomMediaAllReady(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{
		{ID: "t1", MediaID: "m1", URL: "/api/media/m1"},
	}
	rm.CurrentIndex = 0

	if uc.EnsureRoomMedia(rm) {
		t.Error("expected false when all tracks already have URLs")
	}
}

func TestEnsureRoomMediaDownloadsPending(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.EnsureFn = func(items []map[string]any) ([]string, error) {
		return []string{"m1", "m2"}, nil
	}
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{
		{ID: "t1", MediaID: "m1", SourceURL: "url1"},
		{ID: "t2", MediaID: "m2", SourceURL: "url2"},
	}
	rm.CurrentIndex = 0

	changed := uc.EnsureRoomMedia(rm)
	if !changed {
		t.Error("expected true when tracks become ready")
	}
	if rm.Queue[0].URL != "/api/media/m1" {
		t.Errorf("expected URL for track 1, got '%s'", rm.Queue[0].URL)
	}
}

func TestEnsureRoomMediaPartialReady(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.EnsureFn = func(items []map[string]any) ([]string, error) {
		return []string{"m1"}, nil
	}
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{
		{ID: "t1", MediaID: "m1", SourceURL: "url1"},
		{ID: "t2", MediaID: "m2", SourceURL: "url2"},
	}
	rm.CurrentIndex = 0

	changed := uc.EnsureRoomMedia(rm)
	if !changed {
		t.Error("expected true when at least one track becomes ready")
	}
	if rm.Queue[0].URL != "/api/media/m1" {
		t.Error("expected track 1 to have URL")
	}
	if rm.Queue[1].URL != "" {
		t.Error("expected track 2 to still have no URL")
	}
}

func TestFetchTrack(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		return map[string]any{"media_id": "m1", "title": "Song"}, nil
	}
	cfg := &config.Config{}
	uc := NewMediaUsecase(mediaClient, cfg, testLog2)

	result, err := uc.FetchTrack("https://youtube.com/watch?v=abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["media_id"] != "m1" {
		t.Errorf("expected media_id 'm1', got '%v'", result["media_id"])
	}
}

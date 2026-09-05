package usecase

import (
	"log/slog"
	"os"
	"testing"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

var radioLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func testRadioConfig() *config.Config {
	return &config.Config{
		RadioRefillAt: 1,
		RadioBatch:    3,
		MaxDuration:   3600,
	}
}

func TestNeedsRefillDisabled(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = false

	if uc.NeedsRefill(rm) {
		t.Error("expected false when auto_radio is disabled")
	}
}

func TestNeedsRefillWithSeed(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=abc"

	if !uc.NeedsRefill(rm) {
		t.Error("expected true when auto_radio on, seed set, queue short")
	}
}

func TestNeedsRefillNoSeed(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true

	if uc.NeedsRefill(rm) {
		t.Error("expected false when no seed URL")
	}
}

func TestNeedsRefillQueueFull(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=abc"
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}}

	if uc.NeedsRefill(rm) {
		t.Error("expected false when queue has tracks above refill threshold")
	}
}

func TestNeedsRefillFilling(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=abc"
	rm.RadioFilling = true

	if uc.NeedsRefill(rm) {
		t.Error("expected false when already filling")
	}
}

func TestRefillAppendsTracks(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://youtube.com/watch?v=1", "https://youtube.com/watch?v=2"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		return map[string]any{
			"title":      "Related Song",
			"artist":     "Radio Artist",
			"duration":   200,
			"source_url": url,
			"media_id":   "media_" + url,
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"

	tracks, err := uc.Refill(rm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 2 {
		t.Errorf("expected 2 tracks, got %d", len(tracks))
	}
	if tracks[0].AddedBy != RadioTag {
		t.Errorf("expected added_by '%s', got '%s'", RadioTag, tracks[0].AddedBy)
	}
	if !rm.IsPlaying {
		t.Error("expected is_playing true after refill")
	}
}

func TestRefillSkipsTooLong(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://youtube.com/watch?v=short", "https://youtube.com/watch?v=long"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		duration := float64(200)
		if url == "https://youtube.com/watch?v=long" {
			duration = float64(7200)
		}
		return map[string]any{
			"title":      "Song",
			"duration":   duration,
			"source_url": url,
			"media_id":   "media_" + url,
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"

	tracks, _ := uc.Refill(rm)
	if len(tracks) != 1 {
		t.Errorf("expected 1 track (long skipped), got %d", len(tracks))
	}
}

func TestRefillNoopWhenDisabled(t *testing.T) {
	called := false
	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		called = true
		return nil, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = false

	tracks, err := uc.Refill(rm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracks != nil {
		t.Error("expected nil tracks")
	}
	if called {
		t.Error("expected Related to not be called")
	}
}

func TestRefillResetsFillingOnComplete(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://youtube.com/watch?v=1"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		return map[string]any{
			"title":      "Song",
			"duration":   200,
			"source_url": url,
			"media_id":   "media_1",
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"

	uc.Refill(rm)

	if rm.RadioFilling {
		t.Error("expected radio_filling false after refill completes")
	}
}

func TestRefillResetsFillingOnError(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return nil, &testError{"network error"}
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"

	uc.Refill(rm)

	if rm.RadioFilling {
		t.Error("expected radio_filling false after error")
	}
}

func TestRefillDeduplicates(t *testing.T) {
	mediaClient := repository.NewMockMediaClient()
	callCount := 0
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://youtube.com/watch?v=1"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		callCount++
		return map[string]any{
			"title":      "Song",
			"duration":   200,
			"source_url": url,
			"media_id":   "media_1",
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog)

	rm := entity.NewRoom("X", "R", "O")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"
	rm.RadioSeen["media_1"] = true

	tracks, _ := uc.Refill(rm)
	if len(tracks) != 0 {
		t.Errorf("expected 0 tracks (already seen), got %d", len(tracks))
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

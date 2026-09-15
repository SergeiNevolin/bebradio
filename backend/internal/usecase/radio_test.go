package usecase

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

var radioLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func testRadioConfig() *config.Config {
	return &config.Config{
		RadioRefillAt: 1,
		RadioBatch:    3,
		MaxDuration:   3600,
	}
}

func setupRadioRoom(t *testing.T, rdb *redis.Client, roomID string, autoRadio bool, seedURL string, tracks []*entity.Track, ps *redisc.PlaybackState) {
	t.Helper()
	ctx := context.Background()
	rm := entity.NewRoom(roomID, "R", "O")
	rm.AutoRadio = autoRadio
	roomRepo := repository.NewMockRoomRepo()
	roomRepo.Save(rm)
	if ps == nil {
		ps = &redisc.PlaybackState{}
	}
	ps.RadioSeedURL = seedURL
	redisc.SetPlayback(ctx, rdb, roomID, ps)
	if len(tracks) > 0 {
		redisc.SetQueue(ctx, rdb, roomID, tracks)
	}
}

func TestNeedsRefillDisabled(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, false, "", nil, nil)

	if uc.NeedsRefill(ctx, roomID, false) {
		t.Error("expected false when auto_radio is disabled")
	}
}

func TestNeedsRefillWithSeed(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=abc", nil, nil)

	if !uc.NeedsRefill(ctx, roomID, true) {
		t.Error("expected true when auto_radio on, seed set, queue short")
	}
}

func TestNeedsRefillNoSeed(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "", nil, nil)

	if uc.NeedsRefill(ctx, roomID, true) {
		t.Error("expected false when no seed URL")
	}
}

func TestNeedsRefillQueueFull(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=abc", tracks, nil)

	if uc.NeedsRefill(ctx, roomID, true) {
		t.Error("expected false when queue has tracks above refill threshold")
	}
}

func TestNeedsRefillFilling(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	mediaClient := repository.NewMockMediaClient()
	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	ps := &redisc.PlaybackState{RadioFilling: true}
	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=abc", nil, ps)

	if uc.NeedsRefill(ctx, roomID, true) {
		t.Error("expected false when already filling")
	}
}

func TestRefillAppendsTracks(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

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
			"id":         "row_" + url,
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=seed", nil, nil)

	tracks, err := uc.Refill(ctx, roomID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 2 {
		t.Errorf("expected 2 tracks, got %d", len(tracks))
	}
	if tracks[0].AddedBy != RadioTag {
		t.Errorf("expected added_by '%s', got '%s'", RadioTag, tracks[0].AddedBy)
	}
	// NOTE: Refill only picks; appending (with live re-validation) and
	// playback resume are the caller's job (backgroundRefill via
	// AppendFreshTrack). Refill must not touch IsPlaying.
	ps, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if ps.IsPlaying {
		t.Error("expected is_playing untouched by Refill itself")
	}
}

func TestRefillSkipsTooLong(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

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
			"id":         "row_" + url,
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=seed", nil, nil)

	tracks, _ := uc.Refill(ctx, roomID)
	if len(tracks) != 1 {
		t.Errorf("expected 1 track (long skipped), got %d", len(tracks))
	}
}

func TestRefillNoopWhenDisabled(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	called := false
	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		called = true
		return nil, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, false, "", nil, nil)

	tracks, err := uc.Refill(ctx, roomID)
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
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

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
			"id":         "row_1",
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=seed", nil, nil)

	uc.Refill(ctx, roomID)

	ps, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if ps.RadioFilling {
		t.Error("expected radio_filling false after refill completes")
	}
}

func TestRefillResetsFillingOnError(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return nil, &testError{"network error"}
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=seed", nil, nil)

	uc.Refill(ctx, roomID)

	ps, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if ps.RadioFilling {
		t.Error("expected radio_filling false after error")
	}
}

func TestRefillDeduplicates(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

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
			"id":         "row_1",
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=seed", []*entity.Track{
		{ID: "existing", MediaID: "media_1"},
	}, nil)

	tracks, _ := uc.Refill(ctx, roomID)
	if len(tracks) != 0 {
		t.Errorf("expected 0 tracks (already seen), got %d", len(tracks))
	}
}

func TestRefillSetsAndClearsFilling(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://youtube.com/watch?v=1"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		return map[string]any{
			"title":    "Song",
			"duration": 200,
			"source_url": url,
			"media_id": "m1",
			"id":       "row_1",
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=seed", nil, nil)

	ps, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if ps.RadioFilling {
		t.Fatal("RadioFilling should start as false")
	}

	tracks, err := uc.Refill(ctx, roomID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(tracks))
	}
	ps, _ = redisc.GetPlayback(ctx, rdb, roomID)
	if ps.RadioFilling {
		t.Error("RadioFilling should be false after Refill completes")
	}
}

func TestRefillNoopWhenFillingAlreadyTrue(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	called := false
	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		called = true
		return []string{"https://youtube.com/watch?v=1"}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	ps := &redisc.PlaybackState{RadioFilling: true}
	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=seed", nil, ps)

	tracks, err := uc.Refill(ctx, roomID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracks != nil {
		t.Error("expected nil tracks when already filling")
	}
	if called {
		t.Error("Related should not be called when already filling")
	}
}

func TestRefillCalledAfterNeedsRefill(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://youtube.com/watch?v=1"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		return map[string]any{
			"title":    "Radio Song",
			"duration": 180,
			"source_url": url,
			"media_id": "radio_m1",
			"id":       "row_r1",
		}, nil
	}

	cfg := testRadioConfig()
	uc := NewRadioUsecase(mediaClient, cfg, radioLog, rdb)

	setupRadioRoom(t, rdb, roomID, true, "https://youtube.com/watch?v=seed", nil, nil)

	if !uc.NeedsRefill(ctx, roomID, true) {
		t.Fatal("expected NeedsRefill to return true")
	}

	tracks, err := uc.Refill(ctx, roomID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track from Refill, got %d", len(tracks))
	}

	existing, _ := redisc.GetQueue(ctx, rdb, roomID)
	redisc.SetQueue(ctx, rdb, roomID, append(existing, tracks...))

	existing, _ = redisc.GetQueue(ctx, rdb, roomID)
	redisc.SetQueue(ctx, rdb, roomID, append(existing, &entity.Track{ID: "extra1"}, &entity.Track{ID: "extra2"}))

	if uc.NeedsRefill(ctx, roomID, true) {
		t.Error("expected NeedsRefill to return false after adding tracks")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

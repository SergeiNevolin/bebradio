package ws

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/redis/go-redis/v9"
)

var handlerLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

type handlerDeps struct {
	manager  *ConnectionManager
	room     *usecase.RoomUsecase
	roomRepo *repository.MockRoomRepo
	radio    *usecase.RadioUsecase
	media    *usecase.MediaUsecase
	handler  *Handler
	rdb      *redis.Client
}

func setupHandler(t *testing.T) *handlerDeps {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()

	cfg := &config.Config{
		RadioRefillAt:    3,
		RadioBatch:       3,
		MaxDuration:      3600,
		AutoAdvanceGrace: 2.5,
	}

	roomUC := usecase.NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, handlerLog, rdb)
	playbackUC := usecase.NewPlaybackUsecase(rdb)
	chatUC := usecase.NewChatUsecase(roomRepo, handlerLog, rdb)
	radioUC := usecase.NewRadioUsecase(mediaClient, cfg, handlerLog, rdb)
	mediaUC := usecase.NewMediaUsecase(mediaClient, cfg, handlerLog, rdb)

	manager := NewConnectionManager(handlerLog)
	handler := NewHandler(manager, roomUC, playbackUC, chatUC, radioUC, mediaUC, cfg, rdb, handlerLog)

	return &handlerDeps{
		manager:  manager,
		room:     roomUC,
		roomRepo: roomRepo,
		radio:    radioUC,
		media:    mediaUC,
		handler:  handler,
		rdb:      rdb,
	}
}

func storeRoomRedis(t *testing.T, d *handlerDeps, rm *entity.Room, tracks []*entity.Track, ps *redisc.PlaybackState) {
	t.Helper()
	ctx := context.Background()
	d.roomRepo.Save(rm)
	if ps == nil {
		ps = &redisc.PlaybackState{}
	}
	redisc.SetPlayback(ctx, d.rdb, rm.ID, ps)
	if len(tracks) > 0 {
		redisc.SetQueue(ctx, d.rdb, rm.ID, tracks)
	}
}

func TestBackgroundRefillAddsTracks(t *testing.T) {
	d := setupHandler(t)
	ctx := context.Background()

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{"https://youtube.com/watch?v=1", "https://youtube.com/watch?v=2"}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		return map[string]any{
			"title":      "Radio Song",
			"artist":     "Radio Artist",
			"duration":   200,
			"source_url": url,
			"media_id":   "media_" + url,
			"id":         "row_" + url,
		}, nil
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog, d.rdb)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	tracks := []*entity.Track{{ID: "existing", Title: "Existing Track", Duration: 300}}
	ps := &redisc.PlaybackState{
		IsPlaying:    true,
		RadioSeedURL: "https://youtube.com/watch?v=seed",
		LastSyncAt:   time.Now(),
	}
	storeRoomRedis(t, d, rm, tracks, ps)

	d.handler.backgroundRefill(ctx, rm, rm.ID)

	finalTracks, _ := redisc.GetQueue(ctx, d.rdb, rm.ID)
	if len(finalTracks) != 3 {
		t.Fatalf("expected 3 tracks after refill (1 existing + 2 radio), got %d", len(finalTracks))
	}

	finalPs, _ := redisc.GetPlayback(ctx, d.rdb, rm.ID)
	if finalPs.RadioFilling {
		t.Error("RadioFilling should be false after backgroundRefill completes")
	}

	for _, track := range finalTracks[1:] {
		if track.AddedBy != "Radio" {
			t.Errorf("expected added_by 'Radio', got '%s'", track.AddedBy)
		}
		if track.MediaID == "" {
			t.Error("expected non-empty media_id for radio track")
		}
	}
}

func TestBackgroundRefillDoesNotPreSetRadioFilling(t *testing.T) {
	d := setupHandler(t)
	ctx := context.Background()

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

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog, d.rdb)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	tracks := []*entity.Track{{ID: "t1", Title: "Track 1", Duration: 300}}
	ps := &redisc.PlaybackState{
		IsPlaying:    true,
		RadioSeedURL: "https://youtube.com/watch?v=seed",
		LastSyncAt:   time.Now(),
	}
	storeRoomRedis(t, d, rm, tracks, ps)

	initialPs, _ := redisc.GetPlayback(ctx, d.rdb, rm.ID)
	if initialPs.RadioFilling {
		t.Fatal("RadioFilling should start as false")
	}

	d.handler.backgroundRefill(ctx, rm, rm.ID)

	finalPs, _ := redisc.GetPlayback(ctx, d.rdb, rm.ID)
	if finalPs.RadioFilling {
		t.Error("RadioFilling should be false after backgroundRefill completes")
	}

	finalTracks, _ := redisc.GetQueue(ctx, d.rdb, rm.ID)
	if len(finalTracks) != 2 {
		t.Fatalf("expected 2 tracks (1 existing + 1 radio), got %d", len(finalTracks))
	}
}

func TestBackgroundRefillHandlesRefillError(t *testing.T) {
	d := setupHandler(t)
	ctx := context.Background()

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return nil, &testRadioError{"media service unavailable"}
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog, d.rdb)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	tracks := []*entity.Track{{ID: "t1", Title: "Track", Duration: 300}}
	ps := &redisc.PlaybackState{
		IsPlaying:    true,
		RadioSeedURL: "https://youtube.com/watch?v=seed",
		LastSyncAt:   time.Now(),
	}
	storeRoomRedis(t, d, rm, tracks, ps)

	d.handler.backgroundRefill(ctx, rm, rm.ID)

	finalTracks, _ := redisc.GetQueue(ctx, d.rdb, rm.ID)
	if len(finalTracks) != 1 {
		t.Errorf("expected queue unchanged on error, got %d tracks", len(finalTracks))
	}
	finalPs, _ := redisc.GetPlayback(ctx, d.rdb, rm.ID)
	if finalPs.RadioFilling {
		t.Error("RadioFilling should be false after error")
	}
}

func TestBackgroundRefillEmptyCandidates(t *testing.T) {
	d := setupHandler(t)
	ctx := context.Background()

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{}, nil
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog, d.rdb)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	tracks := []*entity.Track{{ID: "t1", Title: "Track", Duration: 300}}
	ps := &redisc.PlaybackState{
		IsPlaying:    true,
		RadioSeedURL: "https://youtube.com/watch?v=seed",
		LastSyncAt:   time.Now(),
	}
	storeRoomRedis(t, d, rm, tracks, ps)

	d.handler.backgroundRefill(ctx, rm, rm.ID)

	finalTracks, _ := redisc.GetQueue(ctx, d.rdb, rm.ID)
	if len(finalTracks) != 1 {
		t.Errorf("expected queue unchanged when no candidates, got %d tracks", len(finalTracks))
	}
	finalPs, _ := redisc.GetPlayback(ctx, d.rdb, rm.ID)
	if finalPs.RadioFilling {
		t.Error("RadioFilling should be false after empty result")
	}
}

func TestBackgroundRefillRespectsMaxDuration(t *testing.T) {
	d := setupHandler(t)
	ctx := context.Background()

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{
			"https://youtube.com/watch?v=short",
			"https://youtube.com/watch?v=long",
			"https://youtube.com/watch?v=ok",
		}, nil
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

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog, d.rdb)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	tracks := []*entity.Track{{ID: "t1", Title: "Track", Duration: 300}}
	ps := &redisc.PlaybackState{
		IsPlaying:    true,
		RadioSeedURL: "https://youtube.com/watch?v=seed",
		LastSyncAt:   time.Now(),
	}
	storeRoomRedis(t, d, rm, tracks, ps)

	d.handler.backgroundRefill(ctx, rm, rm.ID)

	finalTracks, _ := redisc.GetQueue(ctx, d.rdb, rm.ID)
	radioTracks := 0
	for _, tr := range finalTracks[1:] {
		if tr.AddedBy == "Radio" {
			radioTracks++
			if tr.Duration > 3600 {
				t.Errorf("radio track too long: %d seconds", tr.Duration)
			}
		}
	}
	if radioTracks != 2 {
		t.Errorf("expected 2 radio tracks (long skipped), got %d", radioTracks)
	}
}

func TestBackgroundRefillSetsIsPlaying(t *testing.T) {
	d := setupHandler(t)
	ctx := context.Background()

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

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog, d.rdb)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	ps := &redisc.PlaybackState{
		IsPlaying:    false,
		RadioSeedURL: "https://youtube.com/watch?v=seed",
	}
	storeRoomRedis(t, d, rm, nil, ps)

	d.handler.backgroundRefill(ctx, rm, rm.ID)

	finalPs, _ := redisc.GetPlayback(ctx, d.rdb, rm.ID)
	if !finalPs.IsPlaying {
		t.Error("expected IsPlaying true after refill adds tracks to empty queue")
	}
}

type testRadioError struct {
	msg string
}

func (e *testRadioError) Error() string { return e.msg }

package ws

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/usecase"
)

var handlerLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

type handlerDeps struct {
	manager *ConnectionManager
	room    *usecase.RoomUsecase
	radio   *usecase.RadioUsecase
	media   *usecase.MediaUsecase
	handler *Handler
}

func setupHandler(t *testing.T) *handlerDeps {
	t.Helper()

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()

	cfg := &config.Config{
		RadioRefillAt:  3,
		RadioBatch:     3,
		MaxDuration:    3600,
		AutoAdvanceGrace: 2.5,
	}

	roomUC := usecase.NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, handlerLog)
	playbackUC := usecase.NewPlaybackUsecase()
	chatUC := usecase.NewChatUsecase(roomRepo, handlerLog)
	radioUC := usecase.NewRadioUsecase(mediaClient, cfg, handlerLog)
	mediaUC := usecase.NewMediaUsecase(mediaClient, cfg, handlerLog)

	manager := NewConnectionManager(handlerLog)
	handler := NewHandler(manager, roomUC, playbackUC, chatUC, radioUC, mediaUC, cfg, handlerLog)

	return &handlerDeps{
		manager: manager,
		room:    roomUC,
		radio:   radioUC,
		media:   mediaUC,
		handler: handler,
	}
}

func TestBackgroundRefillAddsTracks(t *testing.T) {
	d := setupHandler(t)

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
		}, nil
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"
	rm.Queue = []*entity.Track{{ID: "existing", Title: "Existing Track", Duration: 300}}
	rm.IsPlaying = true
	rm.LastSyncAt = time.Now()

	d.room.StoreRoom(rm)

	d.handler.backgroundRefill(rm, rm.ID)

	if len(rm.Queue) != 3 {
		t.Fatalf("expected 3 tracks after refill (1 existing + 2 radio), got %d", len(rm.Queue))
	}

	if rm.RadioFilling {
		t.Error("RadioFilling should be false after backgroundRefill completes")
	}

	for _, track := range rm.Queue[1:] {
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
		}, nil
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"
	rm.Queue = []*entity.Track{{ID: "t1", Title: "Track 1", Duration: 300}}
	rm.IsPlaying = true
	rm.LastSyncAt = time.Now()
	d.room.StoreRoom(rm)

	if rm.RadioFilling {
		t.Fatal("RadioFilling should start as false")
	}

	d.handler.backgroundRefill(rm, rm.ID)

	if rm.RadioFilling {
		t.Error("RadioFilling should be false after backgroundRefill completes")
	}

	if len(rm.Queue) != 2 {
		t.Fatalf("expected 2 tracks (1 existing + 1 radio), got %d", len(rm.Queue))
	}
}

func TestBackgroundRefillHandlesRefillError(t *testing.T) {
	d := setupHandler(t)

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return nil, &testRadioError{"media service unavailable"}
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"
	rm.Queue = []*entity.Track{{ID: "t1", Title: "Track", Duration: 300}}
	rm.IsPlaying = true
	rm.LastSyncAt = time.Now()
	d.room.StoreRoom(rm)

	d.handler.backgroundRefill(rm, rm.ID)

	if len(rm.Queue) != 1 {
		t.Errorf("expected queue unchanged on error, got %d tracks", len(rm.Queue))
	}
	if rm.RadioFilling {
		t.Error("RadioFilling should be false after error")
	}
}

func TestBackgroundRefillEmptyCandidates(t *testing.T) {
	d := setupHandler(t)

	mediaClient := repository.NewMockMediaClient()
	mediaClient.RelatedFn = func(sourceURL string, limit int) ([]string, error) {
		return []string{}, nil
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"
	rm.Queue = []*entity.Track{{ID: "t1", Title: "Track", Duration: 300}}
	rm.IsPlaying = true
	rm.LastSyncAt = time.Now()
	d.room.StoreRoom(rm)

	d.handler.backgroundRefill(rm, rm.ID)

	if len(rm.Queue) != 1 {
		t.Errorf("expected queue unchanged when no candidates, got %d tracks", len(rm.Queue))
	}
	if rm.RadioFilling {
		t.Error("RadioFilling should be false after empty result")
	}
}

func TestBackgroundRefillRespectsMaxDuration(t *testing.T) {
	d := setupHandler(t)

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
		}, nil
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"
	rm.Queue = []*entity.Track{{ID: "t1", Title: "Track", Duration: 300}}
	rm.IsPlaying = true
	rm.LastSyncAt = time.Now()
	d.room.StoreRoom(rm)

	d.handler.backgroundRefill(rm, rm.ID)

	radioTracks := 0
	for _, tr := range rm.Queue[1:] {
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
		}, nil
	}

	d.radio = usecase.NewRadioUsecase(mediaClient, d.handler.config, handlerLog)
	d.handler.radio = d.radio

	rm := entity.NewRoom("R1", "Test Room", "owner1")
	rm.AutoRadio = true
	rm.RadioSeedURL = "https://youtube.com/watch?v=seed"
	rm.IsPlaying = false

	d.room.StoreRoom(rm)

	d.handler.backgroundRefill(rm, rm.ID)

	if !rm.IsPlaying {
		t.Error("expected IsPlaying true after refill adds tracks to empty queue")
	}
}

type testRadioError struct {
	msg string
}

func (e *testRadioError) Error() string { return e.msg }

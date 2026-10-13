package worker

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/delivery/ws"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/redis/go-redis/v9"
)

var workerLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

type workerDeps struct {
	w         *AutoAdvance
	tracks    *repository.MockTrackRepo
	roomRepo  *repository.MockRoomRepo
	rdb       *redis.Client
	radio     *usecase.RadioUsecase
	room      *usecase.RoomUsecase
	ctx       context.Context
}

func setupWorker(t *testing.T) *workerDeps {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	userRepo := repository.NewMockUserRepo()
	mediaClient := repository.NewMockMediaClient()
	auth := repository.NewMockAuthBridge()
	tracks := repository.NewMockTrackRepo()

	cfg := &config.Config{
		RadioRefillAt:       1,
		RadioBatch:          3,
		MaxDuration:         3600,
		AutoAdvanceInterval: 2.0,
		AutoAdvanceGrace:    2.5,
	}

	roomUC := usecase.NewRoomUsecase(roomRepo, userRepo, mediaClient, auth, workerLog, rdb)
	playbackUC := usecase.NewPlaybackUsecase(rdb)
	mediaUC := usecase.NewMediaUsecase(mediaClient, cfg, workerLog, rdb)
	radioUC := usecase.NewRadioUsecase(mediaClient, cfg, workerLog, rdb)
	radioUC.SetTrackRepo(tracks)
	manager := ws.NewConnectionManager(workerLog)

	w := NewAutoAdvance(roomUC, playbackUC, mediaUC, radioUC, manager, cfg, rdb, workerLog)

	return &workerDeps{w: w, tracks: tracks, roomRepo: roomRepo, rdb: rdb, radio: radioUC, room: roomUC, ctx: context.Background()}
}

func saveStreamRoom(t *testing.T, d *workerDeps, roomID string, mashupRadio bool) *entity.Room {
	t.Helper()
	rm := entity.NewRoom(roomID, "Stream", "owner1")
	rm.IsStream = true
	rm.MashupRadio = mashupRadio
	if err := d.roomRepo.Save(rm); err != nil {
		t.Fatal(err)
	}
	return rm
}

func readyTrack(id, title, artist string) *entity.Track {
	return &entity.Track{ID: id, Source: entity.TrackSourceUpload, Title: title, Artist: artist, Duration: 180, Status: entity.TrackStatusReady}
}

// waitFor polls cond until true or the deadline hits.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// A brand-new stream room has no Redis keys at all. Opening the page
// (GetOrLoadRoom) creates the playback hash; the next tick must then fill
// the queue from the library without any manual track add.
func TestTickRefillsColdStream(t *testing.T) {
	d := setupWorker(t)
	roomID := "COLD"
	saveStreamRoom(t, d, roomID, true)
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))
	d.tracks.Create(readyTrack("m2", "Bootleg B", "DJ Y"))

	if _, err := d.room.GetOrLoadRoom(d.ctx, roomID); err != nil {
		t.Fatal(err)
	}
	d.w.tick(d.ctx)

	waitFor(t, "queue refill", func() bool {
		n, _ := redisc.GetQueueLen(d.ctx, d.rdb, roomID)
		return n == 2
	})
	ps, _ := redisc.GetPlayback(d.ctx, d.rdb, roomID)
	if ps == nil || !ps.IsPlaying {
		t.Error("expected the stream to be playing after refill")
	}
}

// With the kill switch off the stream stays empty and silent: no auto-adds.
func TestTickLeavesFlagOffStreamEmpty(t *testing.T) {
	d := setupWorker(t)
	roomID := "OFF"
	saveStreamRoom(t, d, roomID, false)
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))

	if _, err := d.room.GetOrLoadRoom(d.ctx, roomID); err != nil {
		t.Fatal(err)
	}
	d.w.tick(d.ctx)
	// Give any stray goroutine a chance to (incorrectly) refill.
	time.Sleep(200 * time.Millisecond)

	if n, _ := redisc.GetQueueLen(d.ctx, d.rdb, roomID); n != 0 {
		t.Errorf("expected empty queue with mashup_radio off, got %d", n)
	}
}

// A finished track advances even with zero listeners; the drained queue is
// then refilled on the following ticks.
func TestStreamAdvancesFinishedTrack(t *testing.T) {
	d := setupWorker(t)
	roomID := "ADV"
	saveStreamRoom(t, d, roomID, true)
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))
	d.tracks.Create(readyTrack("m2", "Bootleg B", "DJ Y"))

	past := time.Now().Add(-10 * time.Minute)
	redisc.SetQueue(d.ctx, d.rdb, roomID, []*entity.Track{readyTrack("q1", "Queued", "DJ Z")})
	redisc.SetPlayback(d.ctx, d.rdb, roomID, &redisc.PlaybackState{
		CurrentIndex: 0, IsPlaying: true, Position: 0,
		LastSyncAt: past, CurrentStartedAt: past,
	})

	d.w.processRoom(d.ctx, roomID)

	if tracks, _ := redisc.GetQueue(d.ctx, d.rdb, roomID); len(tracks) != 0 {
		t.Fatalf("expected the finished track to be popped, queue=%d", len(tracks))
	}
	// The refill runs in the background: the stream must come back by itself.
	waitFor(t, "post-advance refill", func() bool {
		n, _ := redisc.GetQueueLen(d.ctx, d.rdb, roomID)
		return n == 2
	})
}

// Queue survived but the playback hash is gone (e.g. Redis flush):
// the stream restarts instead of stalling.
func TestLostStateHashRestarts(t *testing.T) {
	d := setupWorker(t)
	roomID := "LOST"
	saveStreamRoom(t, d, roomID, true)
	redisc.SetQueue(d.ctx, d.rdb, roomID, []*entity.Track{readyTrack("q1", "Queued", "DJ Z")})

	d.w.processRoom(d.ctx, roomID)

	ps, _ := redisc.GetPlayback(d.ctx, d.rdb, roomID)
	if ps == nil || !ps.IsPlaying {
		t.Error("expected playback to be recreated and playing")
	}
}

// A crash between "mark filling" and "clear filling" used to stall the
// stream forever. An empty queue proves no refill is usefully in flight,
// so the flag is busted and the refill proceeds.
func TestStuckFillingFlagBusted(t *testing.T) {
	d := setupWorker(t)
	roomID := "STUCK"
	saveStreamRoom(t, d, roomID, true)
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))

	redisc.SetPlayback(d.ctx, d.rdb, roomID, &redisc.PlaybackState{RadioFilling: true})

	d.w.processRoom(d.ctx, roomID)

	waitFor(t, "refill past stuck flag", func() bool {
		n, _ := redisc.GetQueueLen(d.ctx, d.rdb, roomID)
		return n == 1
	})
	ps, _ := redisc.GetPlayback(d.ctx, d.rdb, roomID)
	if ps == nil || !ps.IsPlaying {
		t.Error("expected the stream to be playing after the forced refill")
	}
	if ps != nil && ps.RadioFilling {
		t.Error("expected the stuck filling flag to be cleared")
	}
}

// Nothing playable anywhere (empty library): the stream stays paused instead
// of erroring, and keeps retrying on later ticks.
func TestStarvedStreamStaysPaused(t *testing.T) {
	d := setupWorker(t)
	roomID := "STARVED"
	saveStreamRoom(t, d, roomID, true)

	if _, err := d.room.GetOrLoadRoom(d.ctx, roomID); err != nil {
		t.Fatal(err)
	}
	d.w.tick(d.ctx)
	time.Sleep(200 * time.Millisecond)

	if n, _ := redisc.GetQueueLen(d.ctx, d.rdb, roomID); n != 0 {
		t.Errorf("expected empty queue with no library tracks, got %d", n)
	}
}

// Plain (non-stream) rooms with no listeners are untouched by the tick.
func TestTickIgnoresPlainRoom(t *testing.T) {
	d := setupWorker(t)
	roomID := "PLAIN"
	rm := entity.NewRoom(roomID, "Plain", "owner1")
	if err := d.roomRepo.Save(rm); err != nil {
		t.Fatal(err)
	}
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))

	if _, err := d.room.GetOrLoadRoom(d.ctx, roomID); err != nil {
		t.Fatal(err)
	}
	d.w.tick(d.ctx)
	time.Sleep(200 * time.Millisecond)

	if n, _ := redisc.GetQueueLen(d.ctx, d.rdb, roomID); n != 0 {
		t.Errorf("expected plain room to stay empty, got %d", n)
	}
}

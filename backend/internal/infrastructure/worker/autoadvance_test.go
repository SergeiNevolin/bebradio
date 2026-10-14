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
	radioUC := usecase.NewRadioUsecase(mediaClient, tracks, cfg, workerLog, rdb)
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

	// The pop and the background refill race (miniredis + mocks are
	// in-process, so the refill routinely lands first): asserting an exact
	// empty queue here is flaky. Instead wait until q1 is gone — an empty
	// queue satisfies this instantly, [m1 m2] after the refill; if the pop
	// never happened q1 stays and this times out.
	waitFor(t, "finished track popped", func() bool {
		tracks, _ := redisc.GetQueue(d.ctx, d.rdb, roomID)
		for _, tr := range tracks {
			if tr.ID == "q1" {
				return false
			}
		}
		return true
	})
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

func liveRoom(t *testing.T, d *workerDeps, roomID string, tracks []*entity.Track, startedAt time.Time) {
	t.Helper()
	rm := entity.NewRoom(roomID, "Live", "owner1")
	if err := d.roomRepo.Save(rm); err != nil {
		t.Fatal(err)
	}
	redisc.SetQueue(d.ctx, d.rdb, roomID, tracks)
	redisc.SetPlayback(d.ctx, d.rdb, roomID, &redisc.PlaybackState{
		CurrentIndex: 0, IsPlaying: true, Position: 0,
		LastSyncAt: startedAt, CurrentStartedAt: startedAt,
	})
	redisc.SetPresence(d.ctx, d.rdb, roomID, "addr1", &entity.PresenceInfo{ID: "u1", Name: "Listener"})
}

// A finished track advances while listeners are online (regression cover for
// the fast-path reorder: the DB-hit shortcut must not skip transitions).
func TestProcessRoomWithListenersAdvancesFinishedTrack(t *testing.T) {
	d := setupWorker(t)
	roomID := "LIVEADV"
	past := time.Now().Add(-10 * time.Minute)
	liveRoom(t, d, roomID,
		[]*entity.Track{
			{ID: "t1", Title: "Old", Duration: 100},
			{ID: "t2", Title: "Next", Duration: 100},
		}, past)

	d.w.processRoom(d.ctx, roomID)

	tracks, _ := redisc.GetQueue(d.ctx, d.rdb, roomID)
	if len(tracks) != 1 || tracks[0].ID != "t2" {
		t.Fatalf("expected advance to t2, queue=%v", tracks)
	}
}

// Healthy queue, track mid-play: the tick refreshes media URLs at most and
// leaves playback alone (and still works after the reorder).
func TestProcessRoomHealthyQueueUntouched(t *testing.T) {
	d := setupWorker(t)
	roomID := "LIVEOK"
	now := time.Now()
	liveRoom(t, d, roomID,
		[]*entity.Track{
			{ID: "t1", Title: "Fresh", Duration: 600},
			{ID: "t2", Title: "Next", Duration: 100},
			{ID: "t3", Title: "Later", Duration: 100},
		}, now)

	d.w.processRoom(d.ctx, roomID)

	tracks, _ := redisc.GetQueue(d.ctx, d.rdb, roomID)
	if len(tracks) != 3 || tracks[0].ID != "t1" {
		t.Fatalf("expected untouched queue, got %v", tracks)
	}
	if ps, _ := redisc.GetPlayback(d.ctx, d.rdb, roomID); ps == nil || !ps.IsPlaying || ps.CurrentIndex != 0 {
		t.Fatalf("expected playback intact, got %+v", ps)
	}
}

func idleRoom(t *testing.T, d *workerDeps, roomID string, isStream bool) {
	t.Helper()
	rm := entity.NewRoom(roomID, "Idle", "owner1")
	rm.IsStream = isStream
	rm.MashupRadio = false
	if err := d.roomRepo.Save(rm); err != nil {
		t.Fatal(err)
	}
	// Paused, empty queue: nothing of value in Redis.
	redisc.SetPlayback(d.ctx, d.rdb, roomID, &redisc.PlaybackState{})
}

// A room idle past the TTL loses its Redis keys (its queue persists in
// Postgres and rehydrates on the next visit).
func TestReapIdleRoom(t *testing.T) {
	d := setupWorker(t)
	roomID := "REAP"
	idleRoom(t, d, roomID, false)

	if reaped := d.w.reapIfIdle(d.ctx, roomID); reaped {
		t.Fatal("first sighting must only start the idle clock")
	}
	old := time.Now().Add(-25 * time.Hour)
	if err := redisc.SetIdleSince(d.ctx, d.rdb, roomID, old); err != nil {
		t.Fatal(err)
	}

	if reaped := d.w.reapIfIdle(d.ctx, roomID); !reaped {
		t.Fatal("expected long-idle room to be reaped")
	}
	if ps, _ := redisc.GetPlayback(d.ctx, d.rdb, roomID); ps != nil {
		t.Errorf("expected playback hash gone, got %+v", ps)
	}
	if _, ok := redisc.GetIdleSince(d.ctx, d.rdb, roomID); ok {
		t.Error("expected the idle marker gone with the room")
	}
}

// Fresh idle rooms are only clocked, never reaped.
func TestNoReapFreshIdleRoom(t *testing.T) {
	d := setupWorker(t)
	roomID := "FRESH"
	idleRoom(t, d, roomID, false)

	if reaped := d.w.reapIfIdle(d.ctx, roomID); reaped {
		t.Fatal("fresh idle room must not be reaped")
	}
	if _, ok := redisc.GetIdleSince(d.ctx, d.rdb, roomID); !ok {
		t.Error("expected the idle clock to be started")
	}
}

// Queued tracks and active playback veto eviction, however old the marker.
func TestNoReapActiveRoom(t *testing.T) {
	d := setupWorker(t)
	roomID := "BUSY"
	idleRoom(t, d, roomID, false)
	redisc.SetQueue(d.ctx, d.rdb, roomID, []*entity.Track{readyTrack("q1", "Queued", "DJ Z")})
	old := time.Now().Add(-72 * time.Hour)
	if err := redisc.SetIdleSince(d.ctx, d.rdb, roomID, old); err != nil {
		t.Fatal(err)
	}

	if reaped := d.w.reapIfIdle(d.ctx, roomID); reaped {
		t.Fatal("room with queued tracks must not be reaped")
	}
	if n, _ := redisc.GetQueueLen(d.ctx, d.rdb, roomID); n != 1 {
		t.Errorf("expected queue intact, len=%d", n)
	}
}

// Streams are exempt: their Redis-only filter/seen state must survive
// listener droughts. The marker refreshes so we don't pay a DB load on
// every tick.
func TestNoReapStreamRoom(t *testing.T) {
	d := setupWorker(t)
	roomID := "STREAMIDLE"
	idleRoom(t, d, roomID, true)
	old := time.Now().Add(-72 * time.Hour)
	if err := redisc.SetIdleSince(d.ctx, d.rdb, roomID, old); err != nil {
		t.Fatal(err)
	}

	if reaped := d.w.reapIfIdle(d.ctx, roomID); reaped {
		t.Fatal("stream room must be exempt from reaping")
	}
	if since, ok := redisc.GetIdleSince(d.ctx, d.rdb, roomID); !ok || time.Since(since) > time.Hour {
		t.Error("expected the idle marker refreshed for the exempt stream")
	}
}

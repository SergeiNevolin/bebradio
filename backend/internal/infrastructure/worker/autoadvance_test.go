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
	w          *AutoAdvance
	tracks     *repository.MockTrackRepo
	roomRepo   *repository.MockRoomRepo
	streamRepo *repository.MockStreamRepo
	stream     *usecase.StreamUsecase
	rdb        *redis.Client
	radio      *usecase.RadioUsecase
	room       *usecase.RoomUsecase
	ctx        context.Context
}

func setupWorker(t *testing.T) *workerDeps {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	roomRepo := repository.NewMockRoomRepo()
	streamRepo := repository.NewMockStreamRepo()
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
	streamUC := usecase.NewStreamUsecase(streamRepo, roomRepo, userRepo, workerLog, rdb)
	playbackUC := usecase.NewPlaybackUsecase(rdb)
	mediaUC := usecase.NewMediaUsecase(mediaClient, cfg, workerLog, rdb)
	radioUC := usecase.NewRadioUsecase(mediaClient, tracks, cfg, workerLog, rdb)
	refiller := usecase.NewMashupRadioRefiller(tracks, cfg, workerLog, rdb)
	manager := ws.NewConnectionManager(workerLog)

	w := NewAutoAdvance(roomUC, playbackUC, mediaUC, radioUC, manager, cfg, rdb, workerLog, streamUC, refiller)
	return &workerDeps{w: w, tracks: tracks, roomRepo: roomRepo, streamRepo: streamRepo, stream: streamUC, rdb: rdb, radio: radioUC, room: roomUC, ctx: context.Background()}
}

func saveStream(t *testing.T, d *workerDeps, streamID string, enabled bool) *entity.Stream {
	t.Helper()
	st := entity.NewStream(streamID, "Stream", "owner1")
	st.Features.MashupRadio.Enabled = enabled
	if err := d.streamRepo.Save(st); err != nil {
		t.Fatal(err)
	}
	return st
}

func readyTrack(id, title, artist string) *entity.Track {
	return &entity.Track{ID: id, Source: entity.TrackSourceUpload, Title: title, Artist: artist, Duration: 180, Status: entity.TrackStatusReady}
}

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

func TestTickRefillsColdStream(t *testing.T) {
	d := setupWorker(t)
	st := saveStream(t, d, "COLD", true)
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))
	d.tracks.Create(readyTrack("m2", "Bootleg B", "DJ Y"))

	if _, err := d.stream.GetOrLoadStream(d.ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	d.w.tick(d.ctx)

	waitFor(t, "queue refill", func() bool {
		n, _ := redisc.GetQueueLen(d.ctx, d.rdb, st.Ref())
		return n == 2
	})
	ps, _ := redisc.GetPlayback(d.ctx, d.rdb, st.Ref())
	if ps == nil || !ps.IsPlaying {
		t.Error("expected the stream to be playing after refill")
	}
}

func TestTickLoopsExhaustedStreamLibrary(t *testing.T) {
	d := setupWorker(t)
	st := saveStream(t, d, "LOOP", true)
	tr := readyTrack("m1", "Bootleg A", "DJ X")
	tr.MediaID = "media-m1"
	d.tracks.Create(tr)

	if _, err := d.stream.GetOrLoadStream(d.ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	if err := redisc.AddRadioSeen(d.ctx, d.rdb, st.Ref(), tr.MediaID); err != nil {
		t.Fatal(err)
	}
	d.w.tick(d.ctx)

	waitFor(t, "library loop refill", func() bool {
		n, _ := redisc.GetQueueLen(d.ctx, d.rdb, st.Ref())
		return n == 1
	})
	ps, _ := redisc.GetPlayback(d.ctx, d.rdb, st.Ref())
	if ps == nil || !ps.IsPlaying {
		t.Error("expected the stream to restart after the library was exhausted")
	}
}

func TestTickLeavesDisabledMashupStreamEmpty(t *testing.T) {
	d := setupWorker(t)
	st := saveStream(t, d, "OFF", false)
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))
	if _, err := d.stream.GetOrLoadStream(d.ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	d.w.tick(d.ctx)
	time.Sleep(200 * time.Millisecond)
	if n, _ := redisc.GetQueueLen(d.ctx, d.rdb, st.Ref()); n != 0 {
		t.Errorf("expected empty queue with mashup radio off, got %d", n)
	}
}

func TestStreamAdvancesFinishedTrack(t *testing.T) {
	d := setupWorker(t)
	st := saveStream(t, d, "ADV", true)
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))
	d.tracks.Create(readyTrack("m2", "Bootleg B", "DJ Y"))
	if _, err := d.stream.GetOrLoadStream(d.ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-10 * time.Minute)
	redisc.SetQueue(d.ctx, d.rdb, st.Ref(), []*entity.Track{readyTrack("q1", "Queued", "DJ Z")})
	redisc.SetPlayback(d.ctx, d.rdb, st.Ref(), &redisc.PlaybackState{
		CurrentIndex: 0, IsPlaying: true, Position: 0,
		LastSyncAt: past, CurrentStartedAt: past,
	})

	d.w.processStream(d.ctx, st.Ref())

	waitFor(t, "finished track popped", func() bool {
		tracks, _ := redisc.GetQueue(d.ctx, d.rdb, st.Ref())
		for _, tr := range tracks {
			if tr.ID == "q1" {
				return false
			}
		}
		return true
	})
	waitFor(t, "post-advance refill", func() bool {
		n, _ := redisc.GetQueueLen(d.ctx, d.rdb, st.Ref())
		return n == 2
	})
}

func TestStuckFillingFlagBusted(t *testing.T) {
	d := setupWorker(t)
	st := saveStream(t, d, "STUCK", true)
	d.tracks.Create(readyTrack("m1", "Bootleg A", "DJ X"))
	if _, err := d.stream.GetOrLoadStream(d.ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	redisc.SetPlayback(d.ctx, d.rdb, st.Ref(), &redisc.PlaybackState{RadioFilling: true})

	d.w.processStream(d.ctx, st.Ref())

	waitFor(t, "refill past stuck flag", func() bool {
		n, _ := redisc.GetQueueLen(d.ctx, d.rdb, st.Ref())
		return n == 1
	})
	ps, _ := redisc.GetPlayback(d.ctx, d.rdb, st.Ref())
	if ps == nil || !ps.IsPlaying || ps.RadioFilling {
		t.Errorf("expected playing with filling cleared, got %+v", ps)
	}
}

func TestStarvedStreamStaysPaused(t *testing.T) {
	d := setupWorker(t)
	st := saveStream(t, d, "STARVED", true)
	if _, err := d.stream.GetOrLoadStream(d.ctx, st.ID); err != nil {
		t.Fatal(err)
	}
	d.w.tick(d.ctx)
	time.Sleep(200 * time.Millisecond)
	if n, _ := redisc.GetQueueLen(d.ctx, d.rdb, st.Ref()); n != 0 {
		t.Errorf("expected empty queue with no library tracks, got %d", n)
	}
}

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
	if n, _ := redisc.GetQueueLen(d.ctx, d.rdb, rm.Ref()); n != 0 {
		t.Errorf("expected plain room to stay empty, got %d", n)
	}
}

func liveRoom(t *testing.T, d *workerDeps, roomID string, tracks []*entity.Track, startedAt time.Time) {
	t.Helper()
	rm := entity.NewRoom(roomID, "Live", "owner1")
	if err := d.roomRepo.Save(rm); err != nil {
		t.Fatal(err)
	}
	redisc.SetQueue(d.ctx, d.rdb, rm.Ref(), tracks)
	redisc.SetPlayback(d.ctx, d.rdb, rm.Ref(), &redisc.PlaybackState{
		CurrentIndex: 0, IsPlaying: true, Position: 0,
		LastSyncAt: startedAt, CurrentStartedAt: startedAt,
	})
	redisc.SetPresence(d.ctx, d.rdb, rm.Ref(), "addr1", &entity.PresenceInfo{ID: "u1", Name: "Listener"})
}

func TestProcessRoomWithListenersAdvancesFinishedTrack(t *testing.T) {
	d := setupWorker(t)
	roomID := "LIVEADV"
	past := time.Now().Add(-10 * time.Minute)
	liveRoom(t, d, roomID, []*entity.Track{
		{ID: "t1", Title: "Old", Duration: 100},
		{ID: "t2", Title: "Next", Duration: 100},
	}, past)

	d.w.processRoom(d.ctx, entity.NewRoomRef(roomID))

	tracks, _ := redisc.GetQueue(d.ctx, d.rdb, roomID)
	if len(tracks) != 1 || tracks[0].ID != "t2" {
		t.Fatalf("expected advance to t2, queue=%v", tracks)
	}
}

func TestProcessRoomHealthyQueueUntouched(t *testing.T) {
	d := setupWorker(t)
	roomID := "LIVEOK"
	liveRoom(t, d, roomID, []*entity.Track{
		{ID: "t1", Title: "Fresh", Duration: 600},
		{ID: "t2", Title: "Next", Duration: 100},
		{ID: "t3", Title: "Later", Duration: 100},
	}, time.Now())

	d.w.processRoom(d.ctx, entity.NewRoomRef(roomID))

	tracks, _ := redisc.GetQueue(d.ctx, d.rdb, roomID)
	if len(tracks) != 3 || tracks[0].ID != "t1" {
		t.Fatalf("expected untouched queue, got %v", tracks)
	}
	if ps, _ := redisc.GetPlayback(d.ctx, d.rdb, roomID); ps == nil || !ps.IsPlaying || ps.CurrentIndex != 0 {
		t.Fatalf("expected playback intact, got %+v", ps)
	}
}

func idleRoom(t *testing.T, d *workerDeps, roomID string) *entity.Room {
	t.Helper()
	rm := entity.NewRoom(roomID, "Idle", "owner1")
	if err := d.roomRepo.Save(rm); err != nil {
		t.Fatal(err)
	}
	redisc.SetPlayback(d.ctx, d.rdb, rm.Ref(), &redisc.PlaybackState{})
	return rm
}

func TestReapIdleRoom(t *testing.T) {
	d := setupWorker(t)
	rm := idleRoom(t, d, "REAP")
	ref := rm.Ref()

	if reaped := d.w.reapIfIdle(d.ctx, ref); reaped {
		t.Fatal("first sighting must only start the idle clock")
	}
	old := time.Now().Add(-25 * time.Hour)
	if err := redisc.SetIdleSince(d.ctx, d.rdb, rm.ID, old); err != nil {
		t.Fatal(err)
	}
	if reaped := d.w.reapIfIdle(d.ctx, ref); !reaped {
		t.Fatal("expected long-idle room to be reaped")
	}
	if ps, _ := redisc.GetPlayback(d.ctx, d.rdb, ref); ps != nil {
		t.Errorf("expected playback hash gone, got %+v", ps)
	}
	if _, ok := redisc.GetIdleSince(d.ctx, d.rdb, rm.ID); ok {
		t.Error("expected the idle marker gone with the room")
	}
}

func TestNoReapFreshIdleRoom(t *testing.T) {
	d := setupWorker(t)
	rm := idleRoom(t, d, "FRESH")
	if reaped := d.w.reapIfIdle(d.ctx, rm.Ref()); reaped {
		t.Fatal("fresh idle room must not be reaped")
	}
	if _, ok := redisc.GetIdleSince(d.ctx, d.rdb, rm.ID); !ok {
		t.Error("expected the idle clock to be started")
	}
}

func TestNoReapActiveRoom(t *testing.T) {
	d := setupWorker(t)
	rm := idleRoom(t, d, "BUSY")
	redisc.SetQueue(d.ctx, d.rdb, rm.Ref(), []*entity.Track{readyTrack("q1", "Queued", "DJ Z")})
	old := time.Now().Add(-72 * time.Hour)
	if err := redisc.SetIdleSince(d.ctx, d.rdb, rm.ID, old); err != nil {
		t.Fatal(err)
	}
	if reaped := d.w.reapIfIdle(d.ctx, rm.Ref()); reaped {
		t.Fatal("room with queued tracks must not be reaped")
	}
	if n, _ := redisc.GetQueueLen(d.ctx, d.rdb, rm.Ref()); n != 1 {
		t.Errorf("expected queue intact, len=%d", n)
	}
}

package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/delivery/ws"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/redis/go-redis/v9"
)

type AutoAdvance struct {
	room      *usecase.RoomUsecase
	playback  *usecase.PlaybackUsecase
	media     *usecase.MediaUsecase
	radio     *usecase.RadioUsecase
	manager   *ws.ConnectionManager
	config    *config.Config
	rdb       *redis.Client
	log       *slog.Logger
	// lastStarvedLog throttles the "stream has nothing to play" warning per
	// room. Only the single tick goroutine touches it, so no mutex needed.
	lastStarvedLog map[string]time.Time
}

// How often a starved stream (empty queue, library/filter yield nothing)
// re-logs its warning. Without throttling it would spam every tick (2s).
const starvedLogInterval = 5 * time.Minute

// If a track has Duration==0 (live stream, unresolved), skip after this long.
const durationZeroSkipAfter = 5 * time.Minute

func NewAutoAdvance(
	room *usecase.RoomUsecase,
	playback *usecase.PlaybackUsecase,
	media *usecase.MediaUsecase,
	radio *usecase.RadioUsecase,
	manager *ws.ConnectionManager,
	config *config.Config,
	rdb *redis.Client,
	log *slog.Logger,
) *AutoAdvance {
	return &AutoAdvance{
		room:    room,
		playback: playback,
		media:   media,
		radio:   radio,
		manager: manager,
		config:  config,
		rdb:     rdb,
		log:     log,
		lastStarvedLog: make(map[string]time.Time),
	}
}

func (w *AutoAdvance) Run(ctx context.Context) {
	interval := time.Duration(w.config.AutoAdvanceInterval * float64(time.Second))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w.log.Info("auto-advance worker started")

	for {
		select {
		case <-ctx.Done():
			w.log.Info("auto-advance worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *AutoAdvance) tick(ctx context.Context) {
	// Discover rooms from every Redis slice we own: a fresh/cold stream may
	// have a queue or a stream_filter but no state hash yet (nothing ever
	// played). Scanning only *:state would leave such rooms undiscovered
	// forever, so the 24/7 refill would never kick in without a manual add.
	seen := make(map[string]bool)
	for _, pattern := range []string{"room:*:state", "room:*:queue", "room:*:stream_filter"} {
		iter := w.rdb.Scan(ctx, 0, pattern, 100).Iterator()
		for iter.Next(ctx) {
			if roomID := extractRoomID(iter.Val()); roomID != "" && !seen[roomID] {
				seen[roomID] = true
				w.processRoom(ctx, roomID)
			}
		}
	}
}

func (w *AutoAdvance) processRoom(ctx context.Context, roomID string) {
	count, _ := redisc.GetPresenceCount(ctx, w.rdb, roomID)

	// Fast path: rooms with listeners use the classic flow below.
	// Rooms without listeners are skipped unless they are 24/7 streams,
	// which must keep playing (and refilling from mashups) with 0 online.
	if count == 0 {
		w.processStreamRoom(ctx, roomID)
		return
	}

	ps, _ := redisc.GetPlayback(ctx, w.rdb, roomID)
	if ps == nil || !ps.IsPlaying {
		return
	}

	track := redisc.CurrentTrack(ctx, w.rdb, roomID)
	if track == nil {
		// Queue drained while listeners are online: streams loop from the
		// library, plain rooms wait for manual adds.
		if rm, err := w.room.GetOrLoadRoom(ctx, roomID); err == nil {
			if w.radio.NeedsMashupRefill(ctx, roomID, rm) {
				go w.backgroundMashupRefill(ctx, rm, roomID)
			}
		}
		return
	}

	advanced := false
	if track.Duration > 0 {
		pos := redisc.GetCurrentPosition(ctx, w.rdb, roomID)
		if pos >= float64(track.Duration)+w.config.AutoAdvanceGrace {
			advanced = w.playback.GoNext(ctx, roomID)
		}
	} else if !ps.CurrentStartedAt.IsZero() && time.Since(ps.CurrentStartedAt) > durationZeroSkipAfter {
		advanced = w.playback.GoNext(ctx, roomID)
	}

	// Load room metadata for EnsureRoomMedia and SaveTracks.
	rm, err := w.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		return
	}

	refreshed := w.media.EnsureRoomMedia(ctx, roomID)

	if w.radio.NeedsRefill(ctx, roomID, rm.AutoRadio) {
		go w.backgroundRefill(ctx, rm, roomID)
	}
	if w.radio.NeedsMashupRefill(ctx, roomID, rm) {
		go w.backgroundMashupRefill(ctx, rm, roomID)
	}

	if advanced || refreshed {
		if err := w.room.SaveTracks(ctx, rm); err != nil {
			w.log.Error("auto-advance save tracks failed", "room_id", roomID, "error", err)
		}
		w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))
	}
}

// processStreamRoom keeps IsStream rooms alive with zero listeners: advances
// finished tracks by wall clock and refills the queue from the mashup library
// (honouring the room's stream filter). Plain rooms stay paused here.
func (w *AutoAdvance) processStreamRoom(ctx context.Context, roomID string) {
	rm, err := w.room.GetOrLoadRoom(ctx, roomID)
	if err != nil || !rm.IsStream {
		return
	}

	track := redisc.CurrentTrack(ctx, w.rdb, roomID)
	if track == nil {
		// Nothing queued: refill synchronously so the stream restarts even
		// if RefillMashups is the only source of tracks. With MashupRadio
		// off this is a no-op and the stream stays paused for manual adds.
		//
		// A crash between "mark filling" and "clear filling" leaves
		// radio_filling stuck at 1, which would stall the stream forever
		// (every refill path bails on it). An empty queue proves no refill
		// is usefully in flight — a concurrent one only duplicates picks
		// that AppendFreshTrack dedupes — so bust the flag and proceed.
		if ps, _ := redisc.GetPlayback(ctx, w.rdb, roomID); ps != nil && ps.RadioFilling {
			w.log.Warn("stream stuck in filling state, resetting", "room_id", roomID)
			redisc.SetPlaybackField(ctx, w.rdb, roomID, "radio_filling", "0")
		}
		if picks, err := w.radio.RefillMashups(ctx, roomID, rm); err == nil && len(picks) > 0 {
			for _, t := range picks {
				redisc.AppendFreshTrack(ctx, w.rdb, roomID, t)
			}
			w.ensureStreamPlaying(ctx, roomID)
			_ = w.room.SaveTracks(ctx, rm)
			w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))
		} else {
			// Starved: library/filter yield nothing playable. Keep retrying
			// (tracks may be uploaded later) but log throttled, not per tick.
			w.logStarvedThrottled(roomID)
			if w.radio.NeedsMashupRefill(ctx, roomID, rm) {
				go w.backgroundMashupRefill(ctx, rm, roomID)
			}
		}
		return
	}

	ps, _ := redisc.GetPlayback(ctx, w.rdb, roomID)
	if ps == nil {
		// Playback hash lost (e.g. Redis flush) but queue survived:
		// restart playback instead of stalling the stream.
		w.ensureStreamPlaying(ctx, roomID)
		if w.radio.NeedsMashupRefill(ctx, roomID, rm) {
			go w.backgroundMashupRefill(ctx, rm, roomID)
		}
		return
	}
	// Nobody sends sync/position updates, so drive by wall clock from
	// CurrentStartedAt/LastSyncAt via GetCurrentPosition.
	advanced := false
	if track.Duration > 0 {
		if pos := redisc.GetCurrentPosition(ctx, w.rdb, roomID); pos >= float64(track.Duration)+w.config.AutoAdvanceGrace {
			advanced = w.playback.GoNext(ctx, roomID)
		}
	} else if !ps.CurrentStartedAt.IsZero() && time.Since(ps.CurrentStartedAt) > durationZeroSkipAfter {
		advanced = w.playback.GoNext(ctx, roomID)
	}
	// A stream that was paused with tracks queued resumes on its own.
	if !ps.IsPlaying {
		w.ensureStreamPlaying(ctx, roomID)
		advanced = true
	}

	if w.radio.NeedsMashupRefill(ctx, roomID, rm) {
		go w.backgroundMashupRefill(ctx, rm, roomID)
	}

	if advanced {
		_ = w.room.SaveTracks(ctx, rm)
		w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))
	}
}

// logStarvedThrottled warns that a stream queue is empty and the library or
// its filter yield nothing playable. The condition re-triggers every tick,
// so the warning is throttled per room.
func (w *AutoAdvance) logStarvedThrottled(roomID string) {
	if last, ok := w.lastStarvedLog[roomID]; ok && time.Since(last) < starvedLogInterval {
		return
	}
	w.lastStarvedLog[roomID] = time.Now()
	w.log.Warn("stream starved: empty queue and no ready mashups match (check library and stream filter)",
		"room_id", roomID)
}

func (w *AutoAdvance) ensureStreamPlaying(ctx context.Context, roomID string) {
	ps, _ := redisc.GetPlayback(ctx, w.rdb, roomID)
	if ps == nil {
		ps = &redisc.PlaybackState{}
	}
	tracks, _ := redisc.GetQueue(ctx, w.rdb, roomID)
	if len(tracks) == 0 {
		return
	}
	if ps.CurrentIndex < 0 || ps.CurrentIndex >= len(tracks) {
		ps.CurrentIndex = 0
	}
	ps.IsPlaying = true
	ps.Position = 0
	now := time.Now()
	ps.LastSyncAt = now
	ps.CurrentStartedAt = now
	redisc.SetPlayback(ctx, w.rdb, roomID, ps)
}

func (w *AutoAdvance) backgroundMashupRefill(ctx context.Context, rm *entity.Room, roomID string) {
	w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))

	picks, err := w.radio.RefillMashups(ctx, roomID, rm)
	if err != nil {
		w.log.Error("mashup refill failed", "room_id", roomID, "error", err)
		return
	}
	appended := 0
	for _, t := range picks {
		if ok, _ := redisc.AppendFreshTrack(ctx, w.rdb, roomID, t); ok {
			appended++
			if t.ID != "" {
				redisc.AddRadioSeen(ctx, w.rdb, roomID, t.ID)
			}
		}
	}
	if appended > 0 {
		w.ensureStreamPlaying(ctx, roomID)
		if err := w.room.SaveTracks(ctx, rm); err != nil {
			w.log.Error("save tracks after mashup refill failed", "room_id", roomID, "error", err)
		}
	}
	w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))
}

func (w *AutoAdvance) backgroundRefill(ctx context.Context, rm *entity.Room, roomID string) {
	w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))

	tracks, err := w.radio.Refill(ctx, roomID)
	if err != nil {
		w.log.Error("radio refill failed", "room_id", roomID, "error", err)
		return
	}

	// AppendFreshTrack re-validates each pick against the live queue: picks
	// went stale if a skip landed while Refill was doing network I/O.
	appended := 0
	for _, t := range tracks {
		if ok, _ := redisc.AppendFreshTrack(ctx, w.rdb, roomID, t); ok {
			appended++
		}
	}

	if appended > 0 {
		ps, _ := redisc.GetPlayback(ctx, w.rdb, roomID)
		if ps != nil && !ps.IsPlaying {
			ps.IsPlaying = true
			ps.Position = 0
			ps.LastSyncAt = time.Now()
			redisc.SetPlayback(ctx, w.rdb, roomID, ps)
		}

		if err := w.room.SaveTracks(ctx, rm); err != nil {
			w.log.Error("save tracks after refill failed", "room_id", roomID, "error", err)
		}
	}

	w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))
}

func extractRoomID(key string) string {
	// key is "room:{id}:state" — extract {id}
	if len(key) < 6 || key[:5] != "room:" {
		return ""
	}
	rest := key[5:]
	for i, c := range rest {
		if c == ':' {
			return rest[:i]
		}
	}
	return rest
}

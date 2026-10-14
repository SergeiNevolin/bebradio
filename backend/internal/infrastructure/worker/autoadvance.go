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

	// Rooms without listeners are either 24/7 streams (kept alive below)
	// or candidates for Redis eviction (see reapIfIdle).
	if count == 0 {
		if w.reapIfIdle(ctx, roomID) {
			return
		}
		w.processStreamRoom(ctx, roomID)
		return
	}
	redisc.ClearIdleSince(ctx, w.rdb, roomID)

	ps, _ := redisc.GetPlayback(ctx, w.rdb, roomID)
	if ps == nil || !ps.IsPlaying {
		return
	}

	tracks, _ := redisc.GetQueue(ctx, w.rdb, roomID)
	var track *entity.Track
	if ps.CurrentIndex >= 0 && ps.CurrentIndex < len(tracks) {
		track = tracks[ps.CurrentIndex]
	}
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
		// Wall-clock position from this snapshot (ps.IsPlaying here), no
		// extra Redis round trip via GetCurrentPosition.
		pos := ps.Position + time.Since(ps.LastSyncAt).Seconds()
		if pos >= float64(track.Duration)+w.config.AutoAdvanceGrace {
			advanced = w.playback.GoNext(ctx, roomID)
		}
	} else if !ps.CurrentStartedAt.IsZero() && time.Since(ps.CurrentStartedAt) > durationZeroSkipAfter {
		advanced = w.playback.GoNext(ctx, roomID)
	}

	// Fast path: nothing advanced and the queue is healthy. Media URLs are
	// still refreshed (Redis-only); the Postgres room load + refills run
	// only when the queue is low or a transition happened. This keeps the
	// per-tick cost at a few Redis calls instead of a DB hit per room.
	if !advanced && len(tracks) > w.config.RadioRefillAt {
		if w.media.EnsureRoomMedia(ctx, roomID) {
			if rm, err := w.room.GetOrLoadRoom(ctx, roomID); err == nil {
				if err := w.room.SaveTracks(ctx, rm); err != nil {
					w.log.Error("auto-advance save tracks failed", "room_id", roomID, "error", err)
				}
				w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))
			}
		}
		return
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

// idleTTL bounds how long an abandoned room lingers in Redis. Zero config
// (tests, unset env) falls back to the production default instead of
// reaping everything immediately.
func (w *AutoAdvance) idleTTL() time.Duration {
	if w.config.RoomIdleHours > 0 {
		return time.Duration(w.config.RoomIdleHours) * time.Hour
	}
	return 24 * time.Hour
}

// reapIfIdle evicts a long-abandoned room from Redis: no listeners, empty
// queue, not playing, and first observed idle longer than idleTTL ago.
// Queues persist in Postgres through write-through saves, so the room
// rehydrates transparently on the next visit; votes and presence are
// ephemeral by design. Stream rooms are exempt — their Redis-only filter
// and seen state must survive listener droughts. Returns true when the room
// was reaped and the tick must skip it.
func (w *AutoAdvance) reapIfIdle(ctx context.Context, roomID string) bool {
	if qlen, _ := redisc.GetQueueLen(ctx, w.rdb, roomID); qlen > 0 {
		redisc.ClearIdleSince(ctx, w.rdb, roomID)
		return false
	}
	if ps, _ := redisc.GetPlayback(ctx, w.rdb, roomID); ps != nil && ps.IsPlaying {
		redisc.ClearIdleSince(ctx, w.rdb, roomID)
		return false
	}

	since, ok := redisc.GetIdleSince(ctx, w.rdb, roomID)
	if !ok {
		// First sighting of an idle room: start the clock, evict later.
		redisc.SetIdleSince(ctx, w.rdb, roomID, time.Now())
		return false
	}
	if time.Since(since) < w.idleTTL() {
		return false
	}

	rm, err := w.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		// Tribe without a village: the row is gone (or the DB is
		// unreachable — in which case deleting Redis state would destroy
		// data Postgres may still rehydrate). Never reap on DB errors.
		return false
	}
	if rm.IsStream {
		// Exempt, but refresh the marker so we don't pay a DB load on
		// every tick for the rest of time.
		redisc.SetIdleSince(ctx, w.rdb, roomID, time.Now())
		return false
	}

	if err := redisc.DeleteRoom(ctx, w.rdb, roomID); err != nil {
		w.log.Error("reap idle room failed", "room_id", roomID, "error", err)
		return false
	}
	w.log.Info("reaped idle room from redis", "room_id", roomID)
	return true
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
			redisc.SetPlaybackField(ctx, w.rdb, roomID, redisc.FieldRadioFilling, "0")
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

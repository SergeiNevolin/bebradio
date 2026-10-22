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
	room           *usecase.RoomUsecase
	stream         *usecase.StreamUsecase
	playback       *usecase.PlaybackUsecase
	media          *usecase.MediaUsecase
	radio          *usecase.RadioUsecase
	refillers      []usecase.StreamRefiller
	manager        *ws.ConnectionManager
	config         *config.Config
	rdb            *redis.Client
	log            *slog.Logger
	lastStarvedLog map[entity.SessionRef]time.Time
}

const starvedLogInterval = 5 * time.Minute
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
	extras ...any,
) *AutoAdvance {
	w := &AutoAdvance{
		room:           room,
		playback:       playback,
		media:          media,
		radio:          radio,
		manager:        manager,
		config:         config,
		rdb:            rdb,
		log:            log,
		lastStarvedLog: make(map[entity.SessionRef]time.Time),
	}
	for _, extra := range extras {
		switch v := extra.(type) {
		case *usecase.StreamUsecase:
			w.stream = v
		case usecase.StreamRefiller:
			w.refillers = append(w.refillers, v)
		case []usecase.StreamRefiller:
			w.refillers = append(w.refillers, v...)
		}
	}
	return w
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
	seen := make(map[entity.SessionRef]bool)
	for _, pattern := range []string{"session:*:*:state", "session:*:*:queue", "session:*:*:feature:mashup-radio"} {
		iter := w.rdb.Scan(ctx, 0, pattern, 100).Iterator()
		for iter.Next(ctx) {
			ref, _, ok := redisc.SessionRefFromKey(iter.Val())
			if !ok || seen[ref] {
				continue
			}
			seen[ref] = true
			if ref.IsStream() {
				w.processStream(ctx, ref)
			} else {
				w.processRoom(ctx, ref)
			}
		}
	}
}

func (w *AutoAdvance) processRoom(ctx context.Context, ref entity.SessionRef) {
	count, _ := redisc.GetPresenceCount(ctx, w.rdb, ref)
	if count == 0 {
		if w.reapIfIdle(ctx, ref) {
			return
		}
		return
	}
	redisc.ClearIdleSince(ctx, w.rdb, ref.ID)

	ps, _ := redisc.GetPlayback(ctx, w.rdb, ref)
	if ps == nil || !ps.IsPlaying {
		return
	}
	tracks, _ := redisc.GetQueue(ctx, w.rdb, ref)
	var track *entity.Track
	if ps.CurrentIndex >= 0 && ps.CurrentIndex < len(tracks) {
		track = tracks[ps.CurrentIndex]
	}
	if track == nil {
		return
	}

	advanced := false
	if track.Duration > 0 {
		pos := ps.Position + time.Since(ps.LastSyncAt).Seconds()
		if pos >= float64(track.Duration)+w.config.AutoAdvanceGrace {
			advanced = w.playback.GoNext(ctx, ref)
		}
	} else if !ps.CurrentStartedAt.IsZero() && time.Since(ps.CurrentStartedAt) > durationZeroSkipAfter {
		advanced = w.playback.GoNext(ctx, ref)
	}

	if !advanced && len(tracks) > w.config.RadioRefillAt {
		if w.media.EnsureRoomMedia(ctx, ref) {
			if rm, err := w.room.GetOrLoadRoom(ctx, ref.ID); err == nil {
				if err := w.room.SaveTracks(ctx, rm); err != nil {
					w.log.Error("auto-advance save tracks failed", "session", ref.String(), "error", err)
				}
				w.manager.Broadcast(ref, redisc.BuildToDict(ctx, w.rdb, rm))
			}
		}
		return
	}

	rm, err := w.room.GetOrLoadRoom(ctx, ref.ID)
	if err != nil {
		return
	}
	refreshed := w.media.EnsureRoomMedia(ctx, ref)
	if w.radio.NeedsRefill(ctx, ref.ID, rm.AutoRadio) {
		go w.backgroundRefill(ctx, rm, ref.ID)
	}
	if advanced || refreshed {
		if err := w.room.SaveTracks(ctx, rm); err != nil {
			w.log.Error("auto-advance save tracks failed", "session", ref.String(), "error", err)
		}
		w.manager.Broadcast(ref, redisc.BuildToDict(ctx, w.rdb, rm))
	}
}

func (w *AutoAdvance) idleTTL() time.Duration {
	if w.config.RoomIdleHours > 0 {
		return time.Duration(w.config.RoomIdleHours) * time.Hour
	}
	return 24 * time.Hour
}

func (w *AutoAdvance) reapIfIdle(ctx context.Context, ref entity.SessionRef) bool {
	if qlen, _ := redisc.GetQueueLen(ctx, w.rdb, ref); qlen > 0 {
		redisc.ClearIdleSince(ctx, w.rdb, ref.ID)
		return false
	}
	if ps, _ := redisc.GetPlayback(ctx, w.rdb, ref); ps != nil && ps.IsPlaying {
		redisc.ClearIdleSince(ctx, w.rdb, ref.ID)
		return false
	}
	since, ok := redisc.GetIdleSince(ctx, w.rdb, ref.ID)
	if !ok {
		redisc.SetIdleSince(ctx, w.rdb, ref.ID, time.Now())
		return false
	}
	if time.Since(since) < w.idleTTL() {
		return false
	}
	if _, err := w.room.GetOrLoadRoom(ctx, ref.ID); err != nil {
		return false
	}
	if err := redisc.DeleteSession(ctx, w.rdb, ref); err != nil {
		w.log.Error("reap idle room failed", "session", ref.String(), "error", err)
		return false
	}
	w.log.Info("reaped idle room from redis", "session", ref.String())
	return true
}

func (w *AutoAdvance) processStream(ctx context.Context, ref entity.SessionRef) {
	if w.stream == nil {
		return
	}
	st, err := w.stream.GetOrLoadStream(ctx, ref.ID)
	if err != nil {
		return
	}
	ref = st.Ref()

	track := redisc.CurrentTrack(ctx, w.rdb, ref)
	if track == nil {
		if ps, _ := redisc.GetPlayback(ctx, w.rdb, ref); ps != nil && ps.RadioFilling {
			w.log.Warn("stream stuck in filling state, resetting", "session", ref.String())
			redisc.SetPlaybackField(ctx, w.rdb, ref, redisc.FieldRadioFilling, "0")
		}
		added := false
		attempted := false
		for _, refiller := range w.refillers {
			if !refiller.Needs(ctx, ref) {
				continue
			}
			attempted = true
			picks, err := refiller.Refill(ctx, ref)
			if err != nil {
				w.log.Error("stream refill failed", "session", ref.String(), "refiller", refiller.Key(), "error", err)
				continue
			}
			for _, t := range picks {
				if ok, _ := redisc.AppendFreshTrack(ctx, w.rdb, ref, t); ok {
					added = true
				}
			}
		}
		if added {
			w.ensureStreamPlaying(ctx, ref)
			if err := w.stream.SaveTracks(ctx, st); err != nil {
				w.log.Error("save stream tracks failed", "session", ref.String(), "error", err)
			}
			w.manager.Broadcast(ref, w.stream.BuildToDict(ctx, st))
		} else if attempted {
			w.logStarvedThrottled(ref)
		}
		return
	}

	ps, _ := redisc.GetPlayback(ctx, w.rdb, ref)
	if ps == nil {
		w.ensureStreamPlaying(ctx, ref)
		return
	}
	advanced := false
	if track.Duration > 0 {
		if pos := redisc.GetCurrentPosition(ctx, w.rdb, ref); pos >= float64(track.Duration)+w.config.AutoAdvanceGrace {
			advanced = w.playback.GoNext(ctx, ref)
		}
	} else if !ps.CurrentStartedAt.IsZero() && time.Since(ps.CurrentStartedAt) > durationZeroSkipAfter {
		advanced = w.playback.GoNext(ctx, ref)
	}
	if !ps.IsPlaying {
		w.ensureStreamPlaying(ctx, ref)
		advanced = true
	}

	for _, refiller := range w.refillers {
		if refiller.Needs(ctx, ref) {
			go w.backgroundStreamRefill(ctx, st, refiller)
		}
	}

	if advanced {
		if err := w.stream.SaveTracks(ctx, st); err != nil {
			w.log.Error("save stream tracks failed", "session", ref.String(), "error", err)
		}
		w.manager.Broadcast(ref, w.stream.BuildToDict(ctx, st))
	}
}

func (w *AutoAdvance) logStarvedThrottled(ref entity.SessionRef) {
	if last, ok := w.lastStarvedLog[ref]; ok && time.Since(last) < starvedLogInterval {
		return
	}
	w.lastStarvedLog[ref] = time.Now()
	w.log.Warn("stream starved: empty queue and no ready tracks match (check library and stream filter)", "session", ref.String())
}

func (w *AutoAdvance) ensureStreamPlaying(ctx context.Context, ref entity.SessionRef) {
	ps, _ := redisc.GetPlayback(ctx, w.rdb, ref)
	if ps == nil {
		ps = &redisc.PlaybackState{}
	}
	tracks, _ := redisc.GetQueue(ctx, w.rdb, ref)
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
	redisc.SetPlayback(ctx, w.rdb, ref, ps)
}

func (w *AutoAdvance) backgroundStreamRefill(ctx context.Context, st *entity.Stream, refiller usecase.StreamRefiller) {
	ref := st.Ref()
	picks, err := refiller.Refill(ctx, ref)
	if err != nil {
		w.log.Error("stream refill failed", "session", ref.String(), "refiller", refiller.Key(), "error", err)
		return
	}
	appended := 0
	for _, t := range picks {
		if ok, _ := redisc.AppendFreshTrack(ctx, w.rdb, ref, t); ok {
			appended++
		}
	}
	if appended > 0 {
		w.ensureStreamPlaying(ctx, ref)
		if err := w.stream.SaveTracks(ctx, st); err != nil {
			w.log.Error("save tracks after stream refill failed", "session", ref.String(), "error", err)
		}
		w.manager.Broadcast(ref, w.stream.BuildToDict(ctx, st))
	}
}

func (w *AutoAdvance) backgroundRefill(ctx context.Context, rm *entity.Room, roomID string) {
	w.manager.Broadcast(rm.Ref(), redisc.BuildToDict(ctx, w.rdb, rm))
	tracks, err := w.radio.Refill(ctx, roomID)
	if err != nil {
		w.log.Error("radio refill failed", "room_id", roomID, "error", err)
		return
	}
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
	w.manager.Broadcast(rm.Ref(), redisc.BuildToDict(ctx, w.rdb, rm))
}

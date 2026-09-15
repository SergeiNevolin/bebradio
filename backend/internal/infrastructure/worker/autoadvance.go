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
}

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
	// Scan Redis for room keys to find active rooms.
	iter := w.rdb.Scan(ctx, 0, "room:*:state", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Extract roomID from "room:{id}:state"
		roomID := extractRoomID(key)
		if roomID == "" {
			continue
		}
		w.processRoom(ctx, roomID)
	}
}

func (w *AutoAdvance) processRoom(ctx context.Context, roomID string) {
	count, _ := redisc.GetPresenceCount(ctx, w.rdb, roomID)
	if count == 0 {
		return
	}

	ps, _ := redisc.GetPlayback(ctx, w.rdb, roomID)
	if ps == nil || !ps.IsPlaying {
		return
	}

	track := redisc.CurrentTrack(ctx, w.rdb, roomID)
	if track == nil {
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

	if advanced || refreshed {
		if err := w.room.SaveTracks(ctx, rm); err != nil {
			w.log.Error("auto-advance save tracks failed", "room_id", roomID, "error", err)
		}
		w.manager.Broadcast(roomID, redisc.BuildToDict(ctx, w.rdb, rm))
	}
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

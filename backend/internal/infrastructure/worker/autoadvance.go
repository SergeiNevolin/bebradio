package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/delivery/ws"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/usecase"
)

type AutoAdvance struct {
	room      *usecase.RoomUsecase
	playback  *usecase.PlaybackUsecase
	media     *usecase.MediaUsecase
	radio     *usecase.RadioUsecase
	manager   *ws.ConnectionManager
	config    *config.Config
	log       *slog.Logger
}

func NewAutoAdvance(
	room *usecase.RoomUsecase,
	playback *usecase.PlaybackUsecase,
	media *usecase.MediaUsecase,
	radio *usecase.RadioUsecase,
	manager *ws.ConnectionManager,
	config *config.Config,
	log *slog.Logger,
) *AutoAdvance {
	return &AutoAdvance{
		room:    room,
		playback: playback,
		media:   media,
		radio:   radio,
		manager: manager,
		config:  config,
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
			w.tick()
		}
	}
}

func (w *AutoAdvance) tick() {
	rooms := w.getRooms()
	for _, rm := range rooms {
		w.processRoom(rm)
	}
}

func (w *AutoAdvance) getRooms() []*entity.Room {
	// Get all rooms from the room usecase
	// The room usecase maintains an in-memory map
	w.room.RoomsMu().RLock()
	defer w.room.RoomsMu().RUnlock()

	rooms := make([]*entity.Room, 0)
	for _, rm := range w.room.GetRooms() {
		rooms = append(rooms, rm)
	}
	return rooms
}

func (w *AutoAdvance) processRoom(rm *entity.Room) {
	roomID := rm.ID

	if w.manager.GetCount(roomID) == 0 {
		return
	}

	advanced := false
	track := rm.CurrentTrack()
	if rm.IsPlaying && track != nil && track.Duration > 0 {
		pos := rm.GetCurrentPosition()
		if pos >= float64(track.Duration)+w.config.AutoAdvanceGrace {
			advanced = w.playback.GoNext(rm)
		}
	}

	refreshed := w.media.EnsureRoomMedia(rm)

	if w.radio.NeedsRefill(rm) {
		go w.backgroundRefill(rm, roomID)
	}

	if advanced || refreshed {
		if err := w.room.SaveTracks(rm); err != nil {
			w.log.Error("auto-advance save tracks failed", "room_id", roomID, "error", err)
		}
		w.manager.Broadcast(roomID, rm.ToDict())
	}
}

func (w *AutoAdvance) backgroundRefill(rm *entity.Room, roomID string) {
	w.manager.Broadcast(roomID, rm.ToDict())

	tracks, err := w.radio.Refill(rm)
	if err != nil {
		w.log.Error("radio refill failed", "room_id", roomID, "error", err)
		return
	}

	if len(tracks) > 0 {
		rm.Mu.Lock()
		rm.Queue = append(rm.Queue, tracks...)
		if !rm.IsPlaying {
			rm.IsPlaying = true
			rm.Position = 0
			rm.LastSyncAt = time.Now()
		}
		rm.Mu.Unlock()

		if err := w.room.SaveTracks(rm); err != nil {
			w.log.Error("save tracks after refill failed", "room_id", roomID, "error", err)
		}
	}

	w.manager.Broadcast(roomID, rm.ToDict())
}

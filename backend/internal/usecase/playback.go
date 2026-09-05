package usecase

import (
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
)

const advanceDedupWindow = 1.0

type PlaybackUsecase struct{}

func NewPlaybackUsecase() *PlaybackUsecase {
	return &PlaybackUsecase{}
}

func (uc *PlaybackUsecase) GoNext(rm *entity.Room) bool {
	rm.Mu.Lock()
	defer rm.Mu.Unlock()

	if len(rm.Queue) == 0 {
		return false
	}

	now := time.Now()
	if now.Sub(rm.LastAdvanceAt).Seconds() < advanceDedupWindow {
		return false
	}
	rm.LastAdvanceAt = now

	idx := clamp(rm.CurrentIndex, 0, len(rm.Queue)-1)
	finished := rm.Queue[idx]
	if finished.SourceURL != "" {
		rm.RadioSeedURL = finished.SourceURL
	}
	rm.Queue = append(rm.Queue[:idx], rm.Queue[idx+1:]...)
	newIdx := idx
	if newIdx >= len(rm.Queue) {
		newIdx = len(rm.Queue) - 1
	}
	if newIdx < 0 {
		newIdx = 0
	}
	rm.CurrentIndex = newIdx
	rm.Position = 0

	if len(rm.Queue) == 0 {
		rm.IsPlaying = false
		return true
	}

	rm.IsPlaying = true
	rm.LastSyncAt = time.Now()
	return true
}

func (uc *PlaybackUsecase) GoPrev(rm *entity.Room) bool {
	rm.Mu.Lock()
	defer rm.Mu.Unlock()

	if len(rm.Queue) == 0 || rm.CurrentIndex <= 0 {
		return false
	}
	rm.CurrentIndex--
	rm.Position = 0
	rm.IsPlaying = true
	rm.LastSyncAt = time.Now()
	return true
}

func (uc *PlaybackUsecase) JumpTo(rm *entity.Room, index int) bool {
	rm.Mu.Lock()
	defer rm.Mu.Unlock()

	if index < 0 || index >= len(rm.Queue) {
		return false
	}
	rm.CurrentIndex = index
	rm.Position = 0
	rm.IsPlaying = true
	rm.LastSyncAt = time.Now()
	return true
}

func (uc *PlaybackUsecase) SeekTo(rm *entity.Room, position float64) {
	rm.Mu.Lock()
	defer rm.Mu.Unlock()
	if position < 0 {
		position = 0
	}
	rm.Position = position
	rm.LastSyncAt = time.Now()
}

func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

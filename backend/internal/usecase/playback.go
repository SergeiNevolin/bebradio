package usecase

import (
	"context"
	"time"

	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

type PlaybackUsecase struct {
	rdb *redis.Client
}

func NewPlaybackUsecase(rdb *redis.Client) *PlaybackUsecase {
	return &PlaybackUsecase{rdb: rdb}
}

const advanceDedupWindow = 1.0

func (uc *PlaybackUsecase) GoNext(ctx context.Context, roomID string) bool {
	ps, err := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if err != nil || ps == nil {
		return false
	}

	now := time.Now()
	if now.Sub(ps.LastAdvanceAt).Seconds() < advanceDedupWindow {
		return false
	}

	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	if len(tracks) == 0 {
		return false
	}

	idx := clamp(ps.CurrentIndex, 0, len(tracks)-1)
	finished := tracks[idx]
	if finished.SourceURL != "" {
		ps.RadioSeedURL = finished.SourceURL
	}

	// Remove the finished track.
	newLen, _ := redisc.RemoveAt(ctx, uc.rdb, roomID, idx)

	newIdx := idx
	if newIdx >= newLen {
		newIdx = newLen - 1
	}
	if newIdx < 0 {
		newIdx = 0
	}

	ps.CurrentIndex = newIdx
	ps.Position = 0
	ps.LastAdvanceAt = now
	if newLen == 0 {
		ps.IsPlaying = false
	} else {
		ps.IsPlaying = true
		ps.LastSyncAt = now
		ps.CurrentStartedAt = now
	}

	redisc.SetPlayback(ctx, uc.rdb, roomID, ps)
	return true
}

func (uc *PlaybackUsecase) GoPrev(ctx context.Context, roomID string) bool {
	ps, err := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if err != nil || ps == nil {
		return false
	}

	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	if len(tracks) == 0 || ps.CurrentIndex <= 0 {
		return false
	}

	ps.CurrentIndex--
	ps.Position = 0
	ps.IsPlaying = true
	ps.LastSyncAt = time.Now()
	ps.CurrentStartedAt = time.Now()

	redisc.SetPlayback(ctx, uc.rdb, roomID, ps)
	return true
}

func (uc *PlaybackUsecase) JumpTo(ctx context.Context, roomID string, index int) bool {
	ps, err := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if err != nil || ps == nil {
		return false
	}

	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	if index < 0 || index >= len(tracks) {
		return false
	}

	ps.CurrentIndex = index
	ps.Position = 0
	ps.IsPlaying = true
	ps.LastSyncAt = time.Now()
	ps.CurrentStartedAt = time.Now()

	redisc.SetPlayback(ctx, uc.rdb, roomID, ps)
	return true
}

func (uc *PlaybackUsecase) SeekTo(ctx context.Context, roomID string, position float64) {
	if position < 0 {
		position = 0
	}
	ps, err := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if err != nil || ps == nil {
		return
	}
	ps.Position = position
	ps.LastSyncAt = time.Now()
	redisc.SetPlayback(ctx, uc.rdb, roomID, ps)
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

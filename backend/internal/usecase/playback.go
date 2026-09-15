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
	return uc.goNext(ctx, roomID, false)
}

// ForceNext advances bypassing the duplicate-suppression window. Only for
// explicit user votes (RegisterVote): the vote carries its own target check
// (voted track must be current), so a forced advance can never double-skip
// the same transition. Automatic advances (client onEnded, autoadvance tick)
// keep the suppression via GoNext.
func (uc *PlaybackUsecase) ForceNext(ctx context.Context, roomID string) bool {
	return uc.goNext(ctx, roomID, true)
}

func (uc *PlaybackUsecase) goNext(ctx context.Context, roomID string, force bool) bool {
	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil {
		return false
	}

	now := time.Now()
	if !force && now.Sub(ps.LastAdvanceAt).Seconds() < advanceDedupWindow {
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

	redisc.RemoveAt(ctx, uc.rdb, roomID, idx)

	// Per-track ephemeral state dies with the track:
	// - its votes must not leak onto a re-added track with the same ID
	// - skip votes were cast against this track, not the next one
	redisc.DelVote(ctx, uc.rdb, roomID, finished.ID)
	redisc.ResetSkipVotes(ctx, uc.rdb, roomID)

	// The finished track must never be recommended by radio again.
	if finished.MediaID != "" {
		redisc.AddRadioSeen(ctx, uc.rdb, roomID, finished.MediaID)
	}

	newTracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	newLen := len(newTracks)
	newIdx := idx
	if newIdx >= newLen {
		newIdx = newLen - 1
	}
	if newIdx < 0 {
		newIdx = 0
	}

	// The new current track always starts with a clean slate:
	// votes cast earlier (or leaked entries) must not greet it.
	if newLen > 0 {
		redisc.DelVote(ctx, uc.rdb, roomID, newTracks[newIdx].ID)
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
	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil {
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
	redisc.ResetSkipVotes(ctx, uc.rdb, roomID)
	// New current track starts with a clean slate.
	redisc.DelVote(ctx, uc.rdb, roomID, tracks[ps.CurrentIndex].ID)

	redisc.SetPlayback(ctx, uc.rdb, roomID, ps)
	return true
}

func (uc *PlaybackUsecase) JumpTo(ctx context.Context, roomID string, index int) bool {
	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	if index < 0 || index >= len(tracks) {
		return false
	}

	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil {
		return false
	}

	switched := ps.CurrentIndex != index
	ps.CurrentIndex = index
	ps.Position = 0
	ps.IsPlaying = true
	ps.LastSyncAt = time.Now()
	ps.CurrentStartedAt = time.Now()
	if switched {
		redisc.ResetSkipVotes(ctx, uc.rdb, roomID)
		// New current track starts with a clean slate.
		redisc.DelVote(ctx, uc.rdb, roomID, tracks[index].ID)
	}

	redisc.SetPlayback(ctx, uc.rdb, roomID, ps)
	return true
}

// RegisterVote records a user's vote (1 = like, -1 = dislike, else retract)
// with set semantics, idempotent per user.
//
// Invariants (votes exist only for the current track):
//   - votes for tracks absent from the queue are ignored: they come from
//     stale clients and previously caused leaked entries plus phantom
//     auto-skips of the new current track;
//   - dislike auto-skip fires only when the voted track IS the current one:
//     votes on queued tracks must never skip whatever is playing;
//   - returns true if the vote triggered an auto-skip.
func (uc *PlaybackUsecase) RegisterVote(ctx context.Context, roomID, userID, trackID string, vote int) bool {
	if userID == "" || trackID == "" {
		return false
	}

	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	inQueue := false
	for _, t := range tracks {
		if t.ID == trackID {
			inQueue = true
			break
		}
	}
	if !inQueue {
		return false
	}

	votes, _ := redisc.GetVotes(ctx, uc.rdb, roomID)
	ve, ok := votes[trackID]
	if !ok {
		ve = &redisc.VoteEntry{}
	}
	ve.ApplyVote(userID, vote)
	redisc.SetVote(ctx, uc.rdb, roomID, trackID, ve)

	if vote != -1 {
		return false
	}
	current := redisc.CurrentTrack(ctx, uc.rdb, roomID)
	if current == nil || current.ID != trackID {
		return false
	}
	if ve.DislikeCount() <= ve.LikeCount() {
		return false
	}
	return uc.ForceNext(ctx, roomID)
}

func (uc *PlaybackUsecase) SeekTo(ctx context.Context, roomID string, position float64) {
	if position < 0 {
		position = 0
	}
	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil {
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

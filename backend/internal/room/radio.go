package room

import (
	"context"
	"time"

	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/youtube"
)

// RadioTag is the "added by" credit given to tracks auto-radio queues.
const RadioTag = "\U0001F4FB Radio"

// radioTimeout bounds one refill. A batch is several yt-dlp calls in sequence,
// each of which can take tens of seconds.
const radioTimeout = 5 * time.Minute

// maybeRefill starts a background auto-radio top-up if the room is due one.
//
// It is cheap and safe to call after any change to a queue or to playback, and
// does nothing unless the room has auto-radio on, is running low, and is not
// already being refilled.
func (s *Service) maybeRefill(e *entry) {
	e.mu.Lock()
	due := s.needsRefill(e.state)
	if due {
		// Claim the refill under the same lock that decided it was needed, so
		// two callers arriving together cannot both start one.
		e.state.RadioFilling = true
	}
	e.mu.Unlock()

	if !due {
		return
	}
	s.goBackground("radio-refill", radioTimeout, func(ctx context.Context) {
		s.refill(ctx, e)
	})
}

// needsRefill reports whether a room is due an auto-radio top-up right now. The
// caller must hold the room's lock.
func (s *Service) needsRefill(r *domain.Room) bool {
	return r.AutoRadio &&
		!r.RadioFilling &&
		len(r.Queue) <= s.cfg.RadioRefillAt &&
		radioSeed(r) != ""
}

// radioSeed is the YouTube URL whose Mix the queue is grown from: the track
// that just finished, or failing that whatever is last in the queue.
func radioSeed(r *domain.Room) string {
	if r.RadioSeedURL != "" {
		return r.RadioSeedURL
	}
	if n := len(r.Queue); n > 0 {
		return r.Queue[n-1].SourceURL
	}
	return ""
}

// refill appends related tracks to a room's queue.
//
// The caller must already have claimed the refill by setting RadioFilling;
// clearing it again is this function's job, whatever happens. Listeners are
// told twice: once when the search starts, so the client can show that the room
// is looking for music, and once when it ends -- even if it found nothing, so
// the indicator always clears.
func (s *Service) refill(ctx context.Context, e *entry) {
	defer func() {
		e.mu.Lock()
		e.state.RadioFilling = false
		e.mu.Unlock()
		s.broadcast(e)
	}()

	e.mu.Lock()
	seed := radioSeed(e.state)
	seen := make(map[string]struct{}, len(e.state.RadioSeen))
	for id := range e.state.RadioSeen {
		seen[id] = struct{}{}
	}
	e.mu.Unlock()

	if seed == "" {
		return
	}
	s.broadcast(e)

	tracks := s.collectRadioTracks(ctx, e, seed, seen, s.cfg.RadioBatch)
	if len(tracks) == 0 {
		return
	}

	e.mu.Lock()
	e.state.Queue = append(e.state.Queue, tracks...)
	if !e.state.IsPlaying {
		e.state.IsPlaying = true
		e.state.Position = 0
		e.state.LastSyncAt = domain.Now()
	}
	e.mu.Unlock()

	s.persistTracks(ctx, e)
}

// collectRadioTracks resolves up to limit unseen tracks from the seed's YouTube
// Mix, recording every video id it considers so a later refill does not offer
// the same track again.
func (s *Service) collectRadioTracks(ctx context.Context, e *entry, seed string, seen map[string]struct{}, limit int) []*domain.Track {
	s.rememberRadioVideo(e, youtube.VideoID(seed))

	// Ask for more candidates than needed: some will already have been played,
	// and some will fail to resolve.
	candidates, err := s.yt.FetchRelated(ctx, seed, limit*4)
	if err != nil {
		s.log.Warn("fetching radio candidates", "seed", seed, "error", err)
		return nil
	}

	picked := make([]*domain.Track, 0, limit)
	for _, url := range candidates {
		if len(picked) >= limit || ctx.Err() != nil {
			break
		}
		vid := youtube.VideoID(url)
		if vid == "" {
			continue
		}
		if _, already := seen[vid]; already {
			continue
		}

		info, err := s.yt.FetchTrack(ctx, url)
		if err != nil {
			s.log.Debug("skipping unresolvable radio track", "url", url, "error", err)
			continue
		}

		seen[vid] = struct{}{}
		s.rememberRadioVideo(e, vid)
		picked = append(picked, domain.NewTrackFromInfo(toDomainInfo(info), RadioTag))
	}
	return picked
}

func (s *Service) rememberRadioVideo(e *entry, videoID string) {
	if videoID == "" {
		return
	}
	e.mu.Lock()
	if e.state.RadioSeen == nil {
		e.state.RadioSeen = map[string]struct{}{}
	}
	e.state.RadioSeen[videoID] = struct{}{}
	e.mu.Unlock()
}

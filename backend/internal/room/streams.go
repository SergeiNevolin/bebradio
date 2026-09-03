package room

import (
	"context"
	"errors"
	"fmt"

	"github.com/leenzstra/bebradio/backend/internal/domain"
)

// scope says which of a room's tracks need live stream URLs.
type scope int

const (
	// currentOnly covers just the track being played. It is what a room needs
	// the moment it is loaded from the store.
	currentOnly scope = iota
	// currentAndNext also covers the track after it, so the hand-off at the end
	// of a song -- and any client-side prefetch or crossfade -- never waits on
	// the network.
	currentAndNext
)

// staleTrack is one track that needs its stream URL re-resolved.
type staleTrack struct {
	id        string
	sourceURL string
}

// refreshTracks re-resolves the stream URLs that are at or near expiry, and
// reports whether any were replaced so the caller knows to persist the queue.
//
// yt-dlp hands back a googlevideo URL that stops working after a few hours. A
// track that has been sitting in a queue therefore needs a fresh URL before
// anybody plays it, and re-resolving one costs a subprocess and several network
// round-trips -- which is why the room lock is dropped for the duration and
// re-taken only to write the results back.
func (s *Service) refreshTracks(ctx context.Context, e *entry, sc scope) (bool, error) {
	stale := s.staleTracks(e, sc)
	if len(stale) == 0 {
		return false, nil
	}

	var (
		changed bool
		errs    []error
	)
	for _, target := range stale {
		info, err := s.yt.ResolveStream(ctx, target.sourceURL)
		if err != nil {
			errs = append(errs, fmt.Errorf("track %s: %w", target.id, err))
			continue
		}

		e.mu.Lock()
		// The queue may have moved on while the lookup was in flight. Apply the
		// result only if the same track is still queued against the same source
		// URL; otherwise the answer is for a track that no longer exists.
		if t := e.state.TrackByID(target.id); t != nil && t.SourceURL == target.sourceURL {
			t.URL = info.StreamURL
			t.StreamExpiresAt = info.ExpiresAt
			changed = true
		}
		e.mu.Unlock()
	}
	return changed, errors.Join(errs...)
}

// staleTracks lists the tracks in scope whose stream URL is at or near expiry.
// A track with no known source URL cannot be re-resolved and is left alone.
func (s *Service) staleTracks(e *entry, sc scope) []staleTrack {
	margin := s.cfg.StreamRefreshMargin.Seconds()
	now := domain.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	indexes := []int{e.state.CurrentIndex}
	if sc == currentAndNext {
		indexes = append(indexes, e.state.CurrentIndex+1)
	}

	var out []staleTrack
	for _, i := range indexes {
		if i < 0 || i >= len(e.state.Queue) {
			continue
		}
		t := e.state.Queue[i]
		if t.SourceURL == "" {
			continue
		}
		if t.StreamExpiresAt-now > margin {
			continue
		}
		out = append(out, staleTrack{id: t.ID, sourceURL: t.SourceURL})
	}
	return out
}

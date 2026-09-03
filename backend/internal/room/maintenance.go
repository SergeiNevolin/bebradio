package room

import (
	"context"
	"strconv"
	"time"
)

// idleEviction is how long a room may sit in memory with nobody connected
// before it is dropped. Everything except transient runtime state (who is
// listening, how far into the current track playback has got) is already
// persisted, so an evicted room is reloaded on demand.
const idleEviction = 30 * time.Minute

// tickTimeout bounds one pass of the maintenance loop.
const tickTimeout = 2 * time.Minute

// Run drives the background loop that keeps rooms moving. It returns when ctx
// is cancelled.
//
// Playback normally advances because a client reaches the end of a track and
// says so. This loop is what keeps a room going when every client has dropped
// or stalled: it advances a playing room whose position has run past the
// current track, keeps the next track's stream URL live so the hand-off never
// waits on the network, and tops up an auto-radio queue that is running dry.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.AutoAdvanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Service) tick(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, tickTimeout)
	defer cancel()

	defer func() {
		// One bad room must not take the loop down with it.
		if r := recover(); r != nil {
			s.log.Error("maintenance tick panicked", "panic", r)
		}
	}()

	for _, e := range s.activeRooms() {
		if ctx.Err() != nil {
			return
		}
		s.advanceIfFinished(ctx, e)
	}
	s.evictIdleRooms()
}

// activeRooms returns the loaded rooms that currently have listeners. A room
// nobody is connected to has nothing to keep in sync.
func (s *Service) activeRooms() []*entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*entry, 0, len(s.rooms))
	for id, e := range s.rooms {
		if s.hub.Count(id) > 0 {
			e.lastTouched = time.Now()
			out = append(out, e)
		}
	}
	return out
}

// advanceIfFinished moves a room on if its current track has run out, then
// makes sure the tracks around the playhead have live stream URLs.
func (s *Service) advanceIfFinished(ctx context.Context, e *entry) {
	grace := s.cfg.AutoAdvanceGrace.Seconds()

	e.mu.Lock()
	advanced := false
	if track := e.state.CurrentTrack(); e.state.IsPlaying && track != nil && track.Duration > 0 {
		if e.state.CurrentPosition() >= float64(track.Duration)+grace {
			advanced = e.state.GoNext(s.cfg.AdvanceDedupWindow.Seconds())
		}
	}
	e.mu.Unlock()

	// Auto-radio is network-bound, so it runs detached: one slow refill must
	// not stall the loop for every other room.
	s.maybeRefill(e)

	refreshed, err := s.refreshTracks(ctx, e, currentAndNext)
	if err != nil {
		s.log.Warn("refreshing streams during maintenance", "error", err)
	}

	if advanced || refreshed {
		s.persistTracks(ctx, e)
		s.broadcast(e)
	}
}

// evictIdleRooms drops rooms that nobody has been connected to for a while.
// Without this the registry only ever grows: every room anybody has ever opened
// would stay resident for the life of the process.
func (s *Service) evictIdleRooms() {
	cutoff := time.Now().Add(-idleEviction)

	s.mu.Lock()
	defer s.mu.Unlock()
	for id, e := range s.rooms {
		if s.hub.Count(id) > 0 || e.lastTouched.After(cutoff) {
			continue
		}
		delete(s.rooms, id)
		s.log.Debug("evicted idle room from memory", "room_id", id)
	}
}

func timeNow() time.Time { return time.Now() }

func itoa(v uint64) string { return strconv.FormatUint(v, 10) }

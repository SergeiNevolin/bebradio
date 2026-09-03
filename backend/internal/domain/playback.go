package domain

import "sort"

func sortStrings(s []string) { sort.Strings(s) }

// GoNext advances to the next track, dropping the one that just finished, and
// reports whether the room's position actually moved.
//
// dedupWindow guards the queue against double-skips: a client "ended" event and
// the server-side auto-advance loop (or several clients at once) can all fire
// within a few milliseconds, and only the first should count.
func (r *Room) GoNext(dedupWindow float64) bool {
	if len(r.Queue) == 0 {
		return false
	}

	now := Now()
	if now-r.LastAdvanceAt < dedupWindow {
		return false
	}
	r.LastAdvanceAt = now

	// CurrentIndex can drift out of range if the queue shrank elsewhere; clamp
	// before removing so we never index past the end.
	idx := r.CurrentIndex
	if idx < 0 {
		idx = 0
	}
	if idx > len(r.Queue)-1 {
		idx = len(r.Queue) - 1
	}

	finished := r.Queue[idx]
	if finished.SourceURL != "" {
		r.RadioSeedURL = finished.SourceURL
	}

	r.Queue = append(r.Queue[:idx], r.Queue[idx+1:]...)
	r.CurrentIndex = clampIndex(idx, len(r.Queue))
	r.Position = 0

	if len(r.Queue) == 0 {
		r.IsPlaying = false
		return true
	}

	r.IsPlaying = true
	r.LastSyncAt = Now()
	return true
}

// GoPrev steps back one track and reports whether anything changed.
func (r *Room) GoPrev() bool {
	if r.CurrentIndex <= 0 {
		return false
	}
	r.CurrentIndex--
	r.Position = 0
	r.IsPlaying = true
	r.LastSyncAt = Now()
	return true
}

// JumpTo moves to a specific queue position and reports whether the index was
// in range.
func (r *Room) JumpTo(index int) bool {
	if index < 0 || index >= len(r.Queue) {
		return false
	}
	r.CurrentIndex = index
	r.Position = 0
	r.IsPlaying = true
	r.LastSyncAt = Now()
	return true
}

// SeekTo moves playback to a position within the current track and re-anchors
// the sync point, so every client extrapolates from the same instant.
func (r *Room) SeekTo(position float64) {
	r.Position = position
	r.LastSyncAt = Now()
}

// Enqueue appends a track and starts playback if the room was idle with an
// empty queue.
func (r *Room) Enqueue(track *Track) {
	r.Queue = append(r.Queue, track)
	if track.SourceURL != "" {
		r.RadioSeedURL = track.SourceURL
	}
	if len(r.Queue) == 1 {
		r.IsPlaying = true
		r.Position = 0
		r.LastSyncAt = Now()
	}
}

func clampIndex(idx, length int) int {
	if length <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx > length-1 {
		return length - 1
	}
	return idx
}

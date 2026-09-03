package domain

import "testing"

// noDedup disables the double-skip guard, so a test can advance repeatedly.
const noDedup = 0

func queued(ids ...string) []*Track {
	out := make([]*Track, len(ids))
	for i, id := range ids {
		out[i] = &Track{ID: id}
	}
	return out
}

func TestGoNextRemovesTheFinishedTrack(t *testing.T) {
	r := NewRoom("", "", "")
	r.Queue = queued("a", "b", "c")
	r.Position = 50

	if !r.GoNext(noDedup) {
		t.Fatal("GoNext() = false, want true")
	}
	if len(r.Queue) != 2 || r.Queue[0].ID != "b" {
		t.Errorf("queue = %v, want b and c", trackIDs(r.Queue))
	}
	if r.CurrentIndex != 0 {
		t.Errorf("current_index = %d, want 0", r.CurrentIndex)
	}
	if r.Position != 0 {
		t.Errorf("position = %v, want 0", r.Position)
	}
	if !r.IsPlaying {
		t.Error("the room should keep playing while tracks remain")
	}
}

func TestGoNextOnTheLastTrackStopsPlayback(t *testing.T) {
	r := NewRoom("", "", "")
	r.Queue = queued("only")
	r.IsPlaying = true

	if !r.GoNext(noDedup) {
		t.Fatal("GoNext() = false, want true")
	}
	if len(r.Queue) != 0 {
		t.Errorf("queue = %v, want empty", trackIDs(r.Queue))
	}
	if r.CurrentIndex != 0 {
		t.Errorf("current_index = %d, want 0", r.CurrentIndex)
	}
	if r.IsPlaying {
		t.Error("an empty queue should not be playing")
	}
}

func TestGoNextOnAnEmptyQueueDoesNothing(t *testing.T) {
	if NewRoom("", "", "").GoNext(noDedup) {
		t.Error("GoNext() on an empty queue = true, want false")
	}
}

// A queue that shrank elsewhere can leave the index past the end. Advancing
// must clamp rather than run off the slice.
func TestGoNextWithAnOutOfRangeIndex(t *testing.T) {
	r := NewRoom("", "", "")
	r.Queue = queued("a", "b")
	r.CurrentIndex = 9

	if !r.GoNext(noDedup) {
		t.Fatal("GoNext() = false, want true")
	}
	if len(r.Queue) != 1 {
		t.Errorf("queue = %v, want one track", trackIDs(r.Queue))
	}
	if r.CurrentIndex < 0 || r.CurrentIndex >= len(r.Queue) {
		t.Errorf("current_index = %d, out of range for %d tracks", r.CurrentIndex, len(r.Queue))
	}
}

// A client "ended" event and the server's own auto-advance loop can fire within
// milliseconds of each other; only the first should move the queue.
func TestGoNextIgnoresRepeatsInsideTheDedupWindow(t *testing.T) {
	r := NewRoom("", "", "")
	r.Queue = queued("a", "b", "c")

	if !r.GoNext(1) {
		t.Fatal("first GoNext() = false, want true")
	}
	if r.GoNext(1) {
		t.Error("second GoNext() inside the window = true, want false")
	}
	if len(r.Queue) != 2 {
		t.Errorf("queue = %v, want two tracks", trackIDs(r.Queue))
	}
}

func TestGoNextAllowsAdvanceAfterTheWindow(t *testing.T) {
	r := NewRoom("", "", "")
	r.Queue = queued("a", "b", "c")

	if !r.GoNext(1) {
		t.Fatal("first GoNext() = false, want true")
	}
	r.LastAdvanceAt = Now() - 5
	if !r.GoNext(1) {
		t.Fatal("GoNext() after the window = false, want true")
	}
	if len(r.Queue) != 1 {
		t.Errorf("queue = %v, want one track", trackIDs(r.Queue))
	}
}

// The track that just finished seeds auto-radio, so the next batch of
// suggestions follows on from what the room was actually listening to.
func TestGoNextRecordsTheRadioSeed(t *testing.T) {
	r := NewRoom("", "", "")
	r.Queue = []*Track{
		{ID: "a", SourceURL: "https://youtu.be/aaa"},
		{ID: "b", SourceURL: "https://youtu.be/bbb"},
	}

	r.GoNext(noDedup)

	if r.RadioSeedURL != "https://youtu.be/aaa" {
		t.Errorf("radio seed = %q, want the finished track", r.RadioSeedURL)
	}
}

func TestGoPrev(t *testing.T) {
	t.Run("steps back", func(t *testing.T) {
		r := NewRoom("", "", "")
		r.Queue = queued("a", "b", "c")
		r.CurrentIndex = 2
		r.Position = 50

		if !r.GoPrev() {
			t.Fatal("GoPrev() = false, want true")
		}
		if r.CurrentIndex != 1 {
			t.Errorf("current_index = %d, want 1", r.CurrentIndex)
		}
		if r.Position != 0 {
			t.Errorf("position = %v, want 0", r.Position)
		}
	})

	t.Run("does nothing at the start", func(t *testing.T) {
		r := NewRoom("", "", "")
		r.Queue = queued("a")

		if r.GoPrev() {
			t.Error("GoPrev() at the start = true, want false")
		}
		if r.CurrentIndex != 0 {
			t.Errorf("current_index = %d, want 0", r.CurrentIndex)
		}
	})
}

func TestJumpTo(t *testing.T) {
	r := NewRoom("", "", "")
	r.Queue = queued("a", "b", "c")
	r.Position = 100

	if !r.JumpTo(2) {
		t.Fatal("JumpTo(2) = false, want true")
	}
	if r.CurrentIndex != 2 || r.Position != 0 || !r.IsPlaying {
		t.Errorf("after jump: index=%d position=%v playing=%v", r.CurrentIndex, r.Position, r.IsPlaying)
	}

	for _, index := range []int{-1, 3, 99} {
		if r.JumpTo(index) {
			t.Errorf("JumpTo(%d) = true, want false", index)
		}
	}
}

func TestSeekTo(t *testing.T) {
	r := NewRoom("", "", "")
	before := r.LastSyncAt

	r.SeekTo(42.5)

	if r.Position != 42.5 {
		t.Errorf("position = %v, want 42.5", r.Position)
	}
	if r.LastSyncAt < before {
		t.Error("seeking should re-anchor the sync point")
	}
}

func TestEnqueueStartsPlaybackOnTheFirstTrack(t *testing.T) {
	r := NewRoom("", "", "")

	r.Enqueue(&Track{ID: "a", SourceURL: "https://youtu.be/aaa"})
	if !r.IsPlaying {
		t.Error("adding the first track should start playback")
	}
	if r.RadioSeedURL != "https://youtu.be/aaa" {
		t.Errorf("radio seed = %q, want the added track", r.RadioSeedURL)
	}

	r.IsPlaying = false
	r.Enqueue(&Track{ID: "b"})
	if r.IsPlaying {
		t.Error("adding a later track should not resume a paused room")
	}
}

func trackIDs(tracks []*Track) []string {
	out := make([]string, len(tracks))
	for i, t := range tracks {
		out[i] = t.ID
	}
	return out
}

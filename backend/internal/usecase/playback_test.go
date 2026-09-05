package usecase

import (
	"testing"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
)

func TestGoNextEmptyQueue(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")

	if uc.GoNext(rm) {
		t.Error("expected false for empty queue")
	}
}

func TestGoNextRemovesCurrentTrack(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{
		{ID: "t1", Title: "Song 1"},
		{ID: "t2", Title: "Song 2"},
		{ID: "t3", Title: "Song 3"},
	}
	rm.CurrentIndex = 0

	if !uc.GoNext(rm) {
		t.Error("expected true")
	}
	if len(rm.Queue) != 2 {
		t.Errorf("expected 2 tracks, got %d", len(rm.Queue))
	}
	if rm.CurrentIndex != 0 {
		t.Errorf("expected index 0, got %d", rm.CurrentIndex)
	}
	if rm.Queue[0].ID != "t2" {
		t.Errorf("expected first track 't2', got '%s'", rm.Queue[0].ID)
	}
}

func TestGoNextAtEnd(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}}

	if !uc.GoNext(rm) {
		t.Error("expected true")
	}
	if len(rm.Queue) != 0 {
		t.Errorf("expected empty queue, got %d tracks", len(rm.Queue))
	}
	if rm.IsPlaying {
		t.Error("expected is_playing false when queue is empty")
	}
}

func TestGoNextResetsPosition(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	rm.Position = 50.0

	uc.GoNext(rm)
	if rm.Position != 0 {
		t.Errorf("expected position 0 after go_next, got %f", rm.Position)
	}
}

func TestGoNextRecordsRadioSeed(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{
		{ID: "t1", SourceURL: "https://youtube.com/watch?v=abc"},
		{ID: "t2"},
	}

	uc.GoNext(rm)
	if rm.RadioSeedURL != "https://youtube.com/watch?v=abc" {
		t.Errorf("expected radio_seed_url set, got '%s'", rm.RadioSeedURL)
	}
}

func TestGoNextSetsPlaying(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}}

	uc.GoNext(rm)
	if !rm.IsPlaying {
		t.Error("expected is_playing true after advancing to next track")
	}
}

func TestGoNextOutOfBoundsIndex(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	rm.CurrentIndex = 9

	if !uc.GoNext(rm) {
		t.Error("expected true (should clamp index)")
	}
	if len(rm.Queue) != 1 {
		t.Errorf("expected 1 track, got %d", len(rm.Queue))
	}
}

func TestGoNextDedup(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}

	if !uc.GoNext(rm) {
		t.Error("expected true for first call")
	}
	// Second call within dedup window should return false
	if uc.GoNext(rm) {
		t.Error("expected false for rapid second call (dedup)")
	}
	if len(rm.Queue) != 2 {
		t.Errorf("expected 2 tracks (only one skip), got %d", len(rm.Queue))
	}
}

func TestGoNextAllowsAfterWindow(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}

	uc.GoNext(rm)

	// Simulate time passing beyond dedup window
	rm.LastAdvanceAt = time.Now().Add(-2 * time.Second)

	if !uc.GoNext(rm) {
		t.Error("expected true after dedup window")
	}
	if len(rm.Queue) != 1 {
		t.Errorf("expected 1 track, got %d", len(rm.Queue))
	}
}

func TestGoPrevEmptyQueue(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")

	if uc.GoPrev(rm) {
		t.Error("expected false for empty queue")
	}
}

func TestGoPrevAtStart(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}}
	rm.CurrentIndex = 0

	if uc.GoPrev(rm) {
		t.Error("expected false at index 0")
	}
	if rm.CurrentIndex != 0 {
		t.Errorf("expected index 0, got %d", rm.CurrentIndex)
	}
}

func TestGoPrevMovesBack(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}
	rm.CurrentIndex = 2

	if !uc.GoPrev(rm) {
		t.Error("expected true")
	}
	if rm.CurrentIndex != 1 {
		t.Errorf("expected index 1, got %d", rm.CurrentIndex)
	}
}

func TestGoPrevResetsPosition(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	rm.CurrentIndex = 1
	rm.Position = 100.0

	uc.GoPrev(rm)
	if rm.Position != 0 {
		t.Errorf("expected position 0, got %f", rm.Position)
	}
}

func TestGoPrevSetsPlaying(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	rm.CurrentIndex = 1
	rm.IsPlaying = false

	uc.GoPrev(rm)
	if !rm.IsPlaying {
		t.Error("expected is_playing true after go_prev")
	}
}

func TestJumpToValid(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}

	if !uc.JumpTo(rm, 2) {
		t.Error("expected true")
	}
	if rm.CurrentIndex != 2 {
		t.Errorf("expected index 2, got %d", rm.CurrentIndex)
	}
}

func TestJumpToResetsPosition(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	rm.Position = 100.0

	uc.JumpTo(rm, 1)
	if rm.Position != 0 {
		t.Errorf("expected position 0, got %f", rm.Position)
	}
}

func TestJumpToNegative(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}}

	if uc.JumpTo(rm, -1) {
		t.Error("expected false for negative index")
	}
}

func TestJumpToOutOfRange(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")
	rm.Queue = []*entity.Track{{ID: "t1"}}

	if uc.JumpTo(rm, 5) {
		t.Error("expected false for out of range index")
	}
}

func TestSeekTo(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")

	uc.SeekTo(rm, 42.5)
	if rm.Position != 42.5 {
		t.Errorf("expected position 42.5, got %f", rm.Position)
	}
}

func TestSeekToNegativeClamped(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")

	uc.SeekTo(rm, -50)
	if rm.Position != 0 {
		t.Errorf("expected position 0 (clamped from -50), got %f", rm.Position)
	}
}

func TestSeekToZero(t *testing.T) {
	uc := NewPlaybackUsecase()
	rm := entity.NewRoom("X", "R", "O")

	uc.SeekTo(rm, 0)
	if rm.Position != 0 {
		t.Errorf("expected position 0, got %f", rm.Position)
	}
}

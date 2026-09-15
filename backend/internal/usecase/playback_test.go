package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

func setupPlaybackRoom(t *testing.T, rdb *redis.Client, roomID string, tracks []*entity.Track, ps *redisc.PlaybackState) {
	t.Helper()
	ctx := context.Background()
	rm := entity.NewRoom(roomID, "R", "O")
	roomRepo := repository.NewMockRoomRepo()
	roomRepo.Save(rm)
	if ps == nil {
		ps = &redisc.PlaybackState{}
	}
	redisc.SetPlayback(ctx, rdb, roomID, ps)
	if len(tracks) > 0 {
		redisc.SetQueue(ctx, rdb, roomID, tracks)
	}
}

func TestGoNextEmptyQueue(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	setupPlaybackRoom(t, rdb, roomID, nil, nil)

	if uc.GoNext(ctx, roomID) {
		t.Error("expected false for empty queue")
	}
}

func TestGoNextRemovesCurrentTrack(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{
		{ID: "t1", Title: "Song 1"},
		{ID: "t2", Title: "Song 2"},
		{ID: "t3", Title: "Song 3"},
	}
	ps := &redisc.PlaybackState{CurrentIndex: 0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if !uc.GoNext(ctx, roomID) {
		t.Error("expected true")
	}
	newTracks, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(newTracks) != 2 {
		t.Errorf("expected 2 tracks, got %d", len(newTracks))
	}
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.CurrentIndex != 0 {
		t.Errorf("expected index 0, got %d", newPs.CurrentIndex)
	}
	if newTracks[0].ID != "t2" {
		t.Errorf("expected first track 't2', got '%s'", newTracks[0].ID)
	}
}

func TestGoNextAtEnd(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}}
	ps := &redisc.PlaybackState{CurrentIndex: 0, IsPlaying: true}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if !uc.GoNext(ctx, roomID) {
		t.Error("expected true")
	}
	newTracks, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(newTracks) != 0 {
		t.Errorf("expected empty queue, got %d tracks", len(newTracks))
	}
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.IsPlaying {
		t.Error("expected is_playing false when queue is empty")
	}
}

func TestGoNextResetsPosition(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	ps := &redisc.PlaybackState{CurrentIndex: 0, Position: 50.0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	uc.GoNext(ctx, roomID)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.Position != 0 {
		t.Errorf("expected position 0 after go_next, got %f", newPs.Position)
	}
}

func TestGoNextRecordsRadioSeed(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{
		{ID: "t1", SourceURL: "https://youtube.com/watch?v=abc"},
		{ID: "t2"},
	}
	ps := &redisc.PlaybackState{CurrentIndex: 0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	uc.GoNext(ctx, roomID)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.RadioSeedURL != "https://youtube.com/watch?v=abc" {
		t.Errorf("expected radio_seed_url set, got '%s'", newPs.RadioSeedURL)
	}
}

func TestGoNextSetsPlaying(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	ps := &redisc.PlaybackState{CurrentIndex: 0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	uc.GoNext(ctx, roomID)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if !newPs.IsPlaying {
		t.Error("expected is_playing true after advancing to next track")
	}
}

func TestGoNextOutOfBoundsIndex(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	ps := &redisc.PlaybackState{CurrentIndex: 9}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if !uc.GoNext(ctx, roomID) {
		t.Error("expected true (should clamp index)")
	}
	newTracks, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(newTracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(newTracks))
	}
}

func TestGoNextDedup(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}
	ps := &redisc.PlaybackState{CurrentIndex: 0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if !uc.GoNext(ctx, roomID) {
		t.Error("expected true for first call")
	}
	if uc.GoNext(ctx, roomID) {
		t.Error("expected false for rapid second call (dedup)")
	}
	newTracks, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(newTracks) != 2 {
		t.Errorf("expected 2 tracks (only one skip), got %d", len(newTracks))
	}
}

func TestGoNextAllowsAfterWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}
	ps := &redisc.PlaybackState{CurrentIndex: 0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	uc.GoNext(ctx, roomID)

	// Simulate time passing beyond dedup window
	redisc.SetPlaybackField(ctx, rdb, roomID, "last_advance_at", time.Now().Add(-2*time.Second).Format(time.RFC3339Nano))

	if !uc.GoNext(ctx, roomID) {
		t.Error("expected true after dedup window")
	}
	newTracks, _ := redisc.GetQueue(ctx, rdb, roomID)
	if len(newTracks) != 1 {
		t.Errorf("expected 1 track, got %d", len(newTracks))
	}
}

func TestGoPrevEmptyQueue(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	setupPlaybackRoom(t, rdb, roomID, nil, nil)

	if uc.GoPrev(ctx, roomID) {
		t.Error("expected false for empty queue")
	}
}

func TestGoPrevAtStart(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}}
	ps := &redisc.PlaybackState{CurrentIndex: 0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if uc.GoPrev(ctx, roomID) {
		t.Error("expected false at index 0")
	}
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.CurrentIndex != 0 {
		t.Errorf("expected index 0, got %d", newPs.CurrentIndex)
	}
}

func TestGoPrevMovesBack(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}
	ps := &redisc.PlaybackState{CurrentIndex: 2}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if !uc.GoPrev(ctx, roomID) {
		t.Error("expected true")
	}
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.CurrentIndex != 1 {
		t.Errorf("expected index 1, got %d", newPs.CurrentIndex)
	}
}

func TestGoPrevResetsPosition(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	ps := &redisc.PlaybackState{CurrentIndex: 1, Position: 100.0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	uc.GoPrev(ctx, roomID)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.Position != 0 {
		t.Errorf("expected position 0, got %f", newPs.Position)
	}
}

func TestGoPrevSetsPlaying(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	ps := &redisc.PlaybackState{CurrentIndex: 1, IsPlaying: false}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	uc.GoPrev(ctx, roomID)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if !newPs.IsPlaying {
		t.Error("expected is_playing true after go_prev")
	}
}

func TestJumpToValid(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}, {ID: "t3"}}
	ps := &redisc.PlaybackState{CurrentIndex: 0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if !uc.JumpTo(ctx, roomID, 2) {
		t.Error("expected true")
	}
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.CurrentIndex != 2 {
		t.Errorf("expected index 2, got %d", newPs.CurrentIndex)
	}
}

func TestJumpToResetsPosition(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}, {ID: "t2"}}
	ps := &redisc.PlaybackState{CurrentIndex: 0, Position: 100.0}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	uc.JumpTo(ctx, roomID, 1)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.Position != 0 {
		t.Errorf("expected position 0, got %f", newPs.Position)
	}
}

func TestJumpToNegative(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}}
	ps := &redisc.PlaybackState{}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if uc.JumpTo(ctx, roomID, -1) {
		t.Error("expected false for negative index")
	}
}

func TestJumpToOutOfRange(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	tracks := []*entity.Track{{ID: "t1"}}
	ps := &redisc.PlaybackState{}
	setupPlaybackRoom(t, rdb, roomID, tracks, ps)

	if uc.JumpTo(ctx, roomID, 5) {
		t.Error("expected false for out of range index")
	}
}

func TestSeekTo(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	ps := &redisc.PlaybackState{}
	setupPlaybackRoom(t, rdb, roomID, nil, ps)

	uc.SeekTo(ctx, roomID, 42.5)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.Position != 42.5 {
		t.Errorf("expected position 42.5, got %f", newPs.Position)
	}
}

func TestSeekToNegativeClamped(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	ps := &redisc.PlaybackState{}
	setupPlaybackRoom(t, rdb, roomID, nil, ps)

	uc.SeekTo(ctx, roomID, -50)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.Position != 0 {
		t.Errorf("expected position 0 (clamped from -50), got %f", newPs.Position)
	}
}

func TestSeekToZero(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "X"

	uc := NewPlaybackUsecase(rdb)
	ps := &redisc.PlaybackState{}
	setupPlaybackRoom(t, rdb, roomID, nil, ps)

	uc.SeekTo(ctx, roomID, 0)
	newPs, _ := redisc.GetPlayback(ctx, rdb, roomID)
	if newPs.Position != 0 {
		t.Errorf("expected position 0, got %f", newPs.Position)
	}
}

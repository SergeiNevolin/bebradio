package usecase

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

func setupStreamUC(t *testing.T, rdb *redis.Client) (*RadioUsecase, *repository.MockTrackRepo) {
	t.Helper()
	mediaClient := repository.NewMockMediaClient()
	uc := NewRadioUsecase(mediaClient, testRadioConfig(), radioLog, rdb)
	tracks := repository.NewMockTrackRepo()
	uc.SetTrackRepo(tracks)
	return uc, tracks
}

func streamRoom(rm *entity.Room) *entity.Room {
	rm.IsStream = true
	rm.MashupRadio = true
	return rm
}

func TestMashupRefillNeedsStream(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	uc, _ := setupStreamUC(t, rdb)

	// Non-stream never needs mashup refill.
	setupRadioRoom(t, rdb, "PLAIN", true, "", nil, &redisc.PlaybackState{IsPlaying: true})
	plain := &entity.Room{ID: "PLAIN"}
	if uc.NeedsMashupRefill(ctx, "PLAIN", plain) {
		t.Error("expected false for non-stream room")
	}
	// Stream with the kill switch off never needs it either.
	off := &entity.Room{ID: "STREAM", IsStream: true, MashupRadio: false}
	if uc.NeedsMashupRefill(ctx, "STREAM", off) {
		t.Error("expected false when mashup_radio is off")
	}
}

func TestMashupRefillFiltersByArtist(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "STREAM"
	uc, tracks := setupStreamUC(t, rdb)

	tracks.Create(&entity.Track{ID: "m1", Source: entity.TrackSourceUpload, Title: "Bootleg A", Artist: "DJ Earworm", Status: entity.TrackStatusReady})
	tracks.Create(&entity.Track{ID: "m2", Source: entity.TrackSourceUpload, Title: "Bootleg B", Artist: "Another Guy", Status: entity.TrackStatusReady})
	tracks.Create(&entity.Track{ID: "m3", Source: entity.TrackSourceUpload, Title: "Processing", Artist: "DJ Earworm", Status: entity.TrackStatusProcessing})

	setupRadioRoom(t, rdb, roomID, false, "", nil, &redisc.PlaybackState{IsPlaying: true})
	redisc.SetStreamFilter(ctx, rdb, roomID, &redisc.StreamFilter{Artist: "earworm"})
	rm := streamRoom(entity.NewRoom(roomID, "R", "O"))

	if !uc.NeedsMashupRefill(ctx, roomID, rm) {
		t.Fatal("expected mashup refill when queue is empty")
	}
	picks, err := uc.RefillMashups(ctx, roomID, rm)
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 || picks[0].ID != "m1" {
		t.Errorf("expected only m1 (artist filter + ready), got %v", picks)
	}
}

func TestMashupRefillEmptyFilterLoops(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "LOOP"
	uc, tracks := setupStreamUC(t, rdb)

	tracks.Create(&entity.Track{ID: "m1", Source: entity.TrackSourceUpload, Title: "A", Status: entity.TrackStatusReady})
	setupRadioRoom(t, rdb, roomID, false, "", nil, &redisc.PlaybackState{IsPlaying: true})
	// Everything seen: refill must loop the base rather than stall.
	redisc.AddRadioSeen(ctx, rdb, roomID, "m1")
	rm := streamRoom(entity.NewRoom(roomID, "R", "O"))

	picks, err := uc.RefillMashups(ctx, roomID, rm)
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 || picks[0].ID != "m1" {
		t.Errorf("expected looped pick m1, got %v", picks)
	}
}

func TestMashupRefillResetsSeenOnLoop(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	roomID := "RESET"
	uc, tracks := setupStreamUC(t, rdb)

	tracks.Create(&entity.Track{ID: "m1", Source: entity.TrackSourceUpload, Title: "A", Status: entity.TrackStatusReady})
	setupRadioRoom(t, rdb, roomID, false, "", nil, &redisc.PlaybackState{IsPlaying: true})
	redisc.AddRadioSeen(ctx, rdb, roomID, "m1")
	rm := streamRoom(entity.NewRoom(roomID, "R", "O"))

	picks, err := uc.RefillMashups(ctx, roomID, rm)
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 {
		t.Fatalf("expected looped pick, got %v", picks)
	}
	// New rotation started: the old seen set must be gone, otherwise it
	// would grow unbounded for as long as the 24/7 stream runs. The caller
	// re-marks whatever it actually appends.
	if seen, _ := redisc.GetRadioSeen(ctx, rdb, roomID); len(seen) != 0 {
		t.Errorf("expected radio_seen reset on loop, got %v", seen)
	}
}

func TestTrackFilterByOwner(t *testing.T) {
	repo := repository.NewMockTrackRepo()
	repo.Create(&entity.Track{ID: "a", OwnerID: "u1", OwnerName: "alice", Title: "Mix", Artist: "DJ", Status: "ready"})
	repo.Create(&entity.Track{ID: "b", OwnerID: "u2", OwnerName: "bob", Title: "Mix", Artist: "DJ", Status: "ready"})

	got, _ := repo.ListFiltered(repository.TrackFilter{Owner: "bob"}, "recent", 10, 0, "")
	if len(got) != 1 || got[0].ID != "b" {
		t.Errorf("expected only bob's track, got %v", got)
	}
	got, _ = repo.ListFiltered(repository.TrackFilter{Title: "mix", Artist: "dj"}, "recent", 10, 0, "")
	if len(got) != 2 {
		t.Errorf("expected 2 tracks by title+artist, got %d", len(got))
	}
}

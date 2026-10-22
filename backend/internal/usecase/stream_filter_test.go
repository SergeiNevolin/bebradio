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

func setupMashupRefiller(t *testing.T) (*MashupRadioRefiller, *repository.MockTrackRepo, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tracks := repository.NewMockTrackRepo()
	return NewMashupRadioRefiller(tracks, testRadioConfig(), radioLog, rdb), tracks, rdb
}

func TestMashupRefillNeedsEnabledFeature(t *testing.T) {
	uc, _, rdb := setupMashupRefiller(t)
	ctx := context.Background()
	ref := entity.NewStreamRef("STREAM")
	redisc.SetPlayback(ctx, rdb, ref, &redisc.PlaybackState{IsPlaying: true})
	redisc.SetMashupRadioFeature(ctx, rdb, ref, entity.MashupRadioFeature{Enabled: false})

	if uc.Needs(ctx, ref) {
		t.Error("expected false when mashup radio is off")
	}
	redisc.SetMashupRadioFeature(ctx, rdb, ref, entity.MashupRadioFeature{Enabled: true})
	if !uc.Needs(ctx, ref) {
		t.Error("expected true when queue is empty and feature is enabled")
	}
}

func TestMashupRefillFiltersByArtist(t *testing.T) {
	uc, tracks, rdb := setupMashupRefiller(t)
	ctx := context.Background()
	ref := entity.NewStreamRef("STREAM")

	tracks.Create(&entity.Track{ID: "m1", Source: entity.TrackSourceUpload, Title: "Bootleg A", Artist: "DJ Earworm", Status: entity.TrackStatusReady})
	tracks.Create(&entity.Track{ID: "m2", Source: entity.TrackSourceUpload, Title: "Bootleg B", Artist: "Another Guy", Status: entity.TrackStatusReady})
	tracks.Create(&entity.Track{ID: "m3", Source: entity.TrackSourceUpload, Title: "Processing", Artist: "DJ Earworm", Status: entity.TrackStatusProcessing})

	redisc.SetPlayback(ctx, rdb, ref, &redisc.PlaybackState{IsPlaying: true})
	redisc.SetMashupRadioFeature(ctx, rdb, ref, entity.MashupRadioFeature{
		Enabled: true,
		Filter:  entity.StreamFilter{Artist: "earworm"},
	})

	picks, err := uc.Refill(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 || picks[0].ID != "m1" {
		t.Errorf("expected only m1 (artist filter + ready), got %v", picks)
	}
}

func TestMashupRefillLoopsExhaustedBase(t *testing.T) {
	uc, tracks, rdb := setupMashupRefiller(t)
	ctx := context.Background()
	ref := entity.NewStreamRef("LOOP")

	tracks.Create(&entity.Track{ID: "m1", Source: entity.TrackSourceUpload, Title: "A", Status: entity.TrackStatusReady})
	redisc.SetPlayback(ctx, rdb, ref, &redisc.PlaybackState{IsPlaying: true})
	redisc.SetMashupRadioFeature(ctx, rdb, ref, entity.MashupRadioFeature{Enabled: true})
	redisc.AddRadioSeen(ctx, rdb, ref, "m1")

	picks, err := uc.Refill(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 || picks[0].ID != "m1" {
		t.Fatalf("expected looped pick m1, got %v", picks)
	}
	if seen, _ := redisc.GetRadioSeen(ctx, rdb, ref); len(seen) != 0 {
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

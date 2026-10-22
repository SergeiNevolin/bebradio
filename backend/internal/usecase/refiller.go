package usecase

import (
	"context"
	"log/slog"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

// StreamRefiller keeps a stream queue full. The worker is agnostic to what a
// refiller draws from; Mashup Radio is simply the first implementation.
type StreamRefiller interface {
	Key() string
	Needs(ctx context.Context, ref entity.SessionRef) bool
	Refill(ctx context.Context, ref entity.SessionRef) ([]*entity.Track, error)
}

type MashupRadioRefiller struct {
	tracks repository.TrackRepository
	config *config.Config
	log    *slog.Logger
	rdb    *redis.Client
}

func NewMashupRadioRefiller(tracks repository.TrackRepository, cfg *config.Config, log *slog.Logger, rdb *redis.Client) *MashupRadioRefiller {
	return &MashupRadioRefiller{tracks: tracks, config: cfg, log: log, rdb: rdb}
}

func (r *MashupRadioRefiller) Key() string {
	return redisc.FeatureMashupRadio
}

func (r *MashupRadioRefiller) Needs(ctx context.Context, ref entity.SessionRef) bool {
	feature, err := redisc.GetMashupRadioFeature(ctx, r.rdb, ref)
	if err != nil || !feature.Enabled {
		return false
	}
	ps, _ := redisc.GetPlayback(ctx, r.rdb, ref)
	if ps != nil && ps.RadioFilling {
		return false
	}
	tracks, _ := redisc.GetQueue(ctx, r.rdb, ref)
	return len(tracks) <= r.config.RadioRefillAt
}

func (r *MashupRadioRefiller) Refill(ctx context.Context, ref entity.SessionRef) ([]*entity.Track, error) {
	feature, err := redisc.GetMashupRadioFeature(ctx, r.rdb, ref)
	if err != nil || !feature.Enabled {
		return nil, nil
	}
	ps, _ := redisc.GetPlayback(ctx, r.rdb, ref)
	if ps == nil {
		ps = &redisc.PlaybackState{}
	}
	tracks, _ := redisc.GetQueue(ctx, r.rdb, ref)
	if ps.RadioFilling || len(tracks) > r.config.RadioRefillAt {
		return nil, nil
	}

	ps.RadioFilling = true
	redisc.SetPlayback(ctx, r.rdb, ref, ps)
	defer func() {
		ps2, _ := redisc.GetPlayback(ctx, r.rdb, ref)
		if ps2 != nil {
			ps2.RadioFilling = false
			redisc.SetPlayback(ctx, r.rdb, ref, ps2)
		}
	}()

	filter := repository.TrackFilter{
		Query:  feature.Filter.Query,
		Title:  feature.Filter.Title,
		Artist: feature.Filter.Artist,
		Owner:  feature.Filter.Owner,
	}

	exclude := make([]string, 0, len(tracks)+8)
	seenIDs := make(map[string]bool)
	for _, t := range tracks {
		exclude = append(exclude, t.ID)
		seenIDs[t.ID] = true
		if t.MediaID != "" {
			seenIDs[t.MediaID] = true
		}
	}
	if seen, err := redisc.GetRadioSeen(ctx, r.rdb, ref); err == nil {
		for id := range seen {
			if !seenIDs[id] {
				seenIDs[id] = true
				exclude = append(exclude, id)
			}
		}
	}

	batch := r.config.RadioBatch
	if batch <= 0 {
		batch = 3
	}
	libs, err := r.tracks.ListRandomReady(filter, batch, exclude)
	if err != nil {
		return nil, err
	}
	if len(libs) == 0 && len(exclude) > 0 {
		if err := redisc.ResetRadioSeen(ctx, r.rdb, ref); err != nil {
			r.log.Warn("radio_seen reset failed", "session", ref.String(), "error", err)
		}
		libs, err = r.tracks.ListRandomReady(filter, batch, nil)
		if err != nil {
			return nil, err
		}
		if len(libs) > 0 {
			fresh, _ := redisc.GetQueue(ctx, r.rdb, ref)
			queued := make(map[string]bool, len(fresh))
			for _, t := range fresh {
				queued[t.ID] = true
			}
			kept := libs[:0]
			for _, l := range libs {
				if !queued[l.ID] {
					kept = append(kept, l)
				}
			}
			libs = kept
		}
	}

	picked := make([]*entity.Track, 0, len(libs))
	for _, lib := range libs {
		if !entity.IsQueueableLibraryTrack(lib) {
			continue
		}
		picked = append(picked, entity.QueueCopyFromLibrary(lib, RadioTag))
	}
	return picked, nil
}

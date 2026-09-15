package usecase

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/redis/go-redis/v9"
)

const RadioTag = "Radio"

type RadioUsecase struct {
	mediaClient repository.MediaClient
	config      *config.Config
	log         *slog.Logger
	rdb         *redis.Client
}

func NewRadioUsecase(mediaClient repository.MediaClient, config *config.Config, log *slog.Logger, rdb *redis.Client) *RadioUsecase {
	return &RadioUsecase{mediaClient: mediaClient, config: config, log: log, rdb: rdb}
}

func (uc *RadioUsecase) NeedsRefill(ctx context.Context, roomID string, autoRadio bool) bool {
	if !autoRadio {
		return false
	}
	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil {
		return false
	}
	if ps.RadioFilling {
		return false
	}
	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	if len(tracks) > uc.config.RadioRefillAt {
		return false
	}
	return uc.seedURL(ctx, roomID, ps, tracks) != ""
}

func (uc *RadioUsecase) seedURL(ctx context.Context, roomID string, ps *redisc.PlaybackState, tracks []*entity.Track) string {
	if ps.RadioSeedURL != "" {
		return ps.RadioSeedURL
	}
	if len(tracks) > 0 {
		return tracks[len(tracks)-1].SourceURL
	}
	return ""
}

type resolveResult struct {
	url  string
	info map[string]any
	err  error
}

func (uc *RadioUsecase) Refill(ctx context.Context, roomID string) ([]*entity.Track, error) {
	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil {
		return nil, nil
	}
	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	if !uc.needsRefill(ctx, roomID, ps, tracks) {
		return nil, nil
	}

	// Mark filling.
	ps.RadioFilling = true
	redisc.SetPlayback(ctx, uc.rdb, roomID, ps)

	defer func() {
		ps2, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
		if ps2 != nil {
			ps2.RadioFilling = false
			redisc.SetPlayback(ctx, uc.rdb, roomID, ps2)
		}
	}()

	seed := uc.seedURL(ctx, roomID, ps, tracks)
	candidates, err := uc.mediaClient.Related(seed, uc.config.RadioBatch*4)
	if err != nil {
		return nil, err
	}

	// Build seen set from queue + persistent radio_seen
	// (finished/skipped tracks must never be recommended again).
	seenMediaIDs := make(map[string]bool)
	for _, t := range tracks {
		if t.MediaID != "" {
			seenMediaIDs[t.MediaID] = true
		}
	}
	if seen, err := redisc.GetRadioSeen(ctx, uc.rdb, roomID); err == nil {
		for mediaID := range seen {
			seenMediaIDs[mediaID] = true
		}
	}

	results := make([]resolveResult, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	for i, url := range candidates {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info, err := uc.mediaClient.Resolve(url)
			results[i] = resolveResult{url: url, info: info, err: err}
		}(i, url)
	}
	wg.Wait()

	var picked []*entity.Track
	for _, r := range results {
		if len(picked) >= uc.config.RadioBatch {
			break
		}
		if r.err != nil || r.info == nil {
			continue
		}
		mediaID, _ := r.info["media_id"].(string)
		rowID, _ := r.info["id"].(string)
		if mediaID == "" || rowID == "" || seenMediaIDs[mediaID] {
			continue
		}
		duration, _ := r.info["duration"].(float64)
		if int(duration) > uc.config.MaxDuration {
			continue
		}

		// No persistent marking here: tracks are marked seen at append time
		// (AppendFreshTrack), after re-validating against the live queue.
		// Marking at pick time would ban tracks that never made it into the
		// queue. seenMediaIDs only dedupes within this batch.
		seenMediaIDs[mediaID] = true

		track := entity.TrackFromYouTube(r.info, RadioTag)
		picked = append(picked, track)
	}

	return picked, nil
}

func (uc *RadioUsecase) needsRefill(ctx context.Context, roomID string, ps *redisc.PlaybackState, tracks []*entity.Track) bool {
	return ps != nil && !ps.RadioFilling && len(tracks) <= uc.config.RadioRefillAt && uc.seedURL(ctx, roomID, ps, tracks) != ""
}

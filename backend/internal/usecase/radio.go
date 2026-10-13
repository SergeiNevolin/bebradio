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
	tracks      repository.TrackRepository
	config      *config.Config
	log         *slog.Logger
	rdb         *redis.Client
}

func NewRadioUsecase(mediaClient repository.MediaClient, config *config.Config, log *slog.Logger, rdb *redis.Client) *RadioUsecase {
	return &RadioUsecase{mediaClient: mediaClient, config: config, log: log, rdb: rdb}
}

// SetTrackRepo wires the library repo for mashup-stream refills. Kept as a
// setter so existing constructor call sites keep working; call it in main.
func (uc *RadioUsecase) SetTrackRepo(tracks repository.TrackRepository) {
	uc.tracks = tracks
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

// NeedsMashupRefill reports whether a 24/7 stream room needs more library
// mashups. Unlike YouTube radio it needs no seed URL: an empty filter means
// the whole base. Only ready tracks are picked downstream. The room's
// MashupRadio kill switch disables it: off = play only manually added tracks.
func (uc *RadioUsecase) NeedsMashupRefill(ctx context.Context, roomID string, rm *entity.Room) bool {
	if rm == nil || !rm.IsStream || !rm.MashupRadio || uc.tracks == nil {
		return false
	}
	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil {
		// Cold stream room: no playback hash yet — needs its first fill.
		tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
		return len(tracks) <= uc.config.RadioRefillAt
	}
	if ps.RadioFilling {
		return false
	}
	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	return len(tracks) <= uc.config.RadioRefillAt
}

// RefillMashups picks random ready library tracks honouring the room's stream
// filter (author/title), skipping queued and recently played ids. Picks are
// returned as queue snapshots; appending + seen-marking stays with the caller
// (AppendFreshTrack) so stale picks never resurrect a skipped track.
// Respects the MashupRadio kill switch: off = no picks.
func (uc *RadioUsecase) RefillMashups(ctx context.Context, roomID string, rm *entity.Room) ([]*entity.Track, error) {
	if uc.tracks == nil || rm == nil || !rm.IsStream || !rm.MashupRadio {
		return nil, nil
	}
	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil {
		// Cold stream room: create the playback hash so the first fill can
		// proceed without a manual kickstart track.
		ps = &redisc.PlaybackState{}
		redisc.SetPlayback(ctx, uc.rdb, roomID, ps)
	}
	if ps.RadioFilling {
		return nil, nil
	}
	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	if len(tracks) > uc.config.RadioRefillAt {
		return nil, nil
	}

	ps.RadioFilling = true
	redisc.SetPlayback(ctx, uc.rdb, roomID, ps)
	defer func() {
		ps2, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
		if ps2 != nil {
			ps2.RadioFilling = false
			redisc.SetPlayback(ctx, uc.rdb, roomID, ps2)
		}
	}()

	sf, _ := redisc.GetStreamFilter(ctx, uc.rdb, roomID)
	filter := repository.TrackFilter{}
	if sf != nil {
		filter = repository.TrackFilter{Query: sf.Query, Title: sf.Title, Artist: sf.Artist, Owner: sf.Owner}
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
	if seen, err := redisc.GetRadioSeen(ctx, uc.rdb, roomID); err == nil {
		for id := range seen {
			if !seenIDs[id] {
				seenIDs[id] = true
				exclude = append(exclude, id)
			}
		}
	}

	batch := uc.config.RadioBatch
	if batch <= 0 {
		batch = 3
	}
	libs, err := uc.tracks.ListRandomReady(filter, batch, exclude)
	if err != nil {
		return nil, err
	}
	// Base exhausted (everything matching the filter already played): start a
	// new rotation rather than stalling the stream. Resetting first keeps
	// radio_seen bounded no matter how long the 24/7 stream runs; the caller
	// re-marks whatever it actually appends.
	if len(libs) == 0 && len(exclude) > 0 {
		if err := redisc.ResetRadioSeen(ctx, uc.rdb, roomID); err != nil {
			uc.log.Warn("radio_seen reset failed", "room_id", roomID, "error", err)
		}
		libs, err = uc.tracks.ListRandomReady(filter, batch, nil)
		if err != nil {
			return nil, err
		}
		// Still honour the live queue: drop ids that are queued right now.
		if len(libs) > 0 {
			fresh, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
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

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

type MediaUsecase struct {
	mediaClient repository.MediaClient
	config      *config.Config
	log         *slog.Logger
	rdb         *redis.Client
}

func NewMediaUsecase(mediaClient repository.MediaClient, config *config.Config, log *slog.Logger, rdb *redis.Client) *MediaUsecase {
	return &MediaUsecase{mediaClient: mediaClient, config: config, log: log, rdb: rdb}
}

func (uc *MediaUsecase) FetchTrack(url string) (map[string]any, error) {
	return uc.mediaClient.Resolve(url)
}

func (uc *MediaUsecase) EnsureMedia(items []map[string]any) ([]string, error) {
	return uc.mediaClient.Ensure(items)
}

func (uc *MediaUsecase) EnsureTrackReady(track *entity.Track) bool {
	if track == nil || track.MediaID == "" {
		return false
	}
	ready, err := uc.mediaClient.Ensure([]map[string]any{
		{"media_id": track.MediaID, "source_url": track.SourceURL},
	})
	if err != nil {
		return false
	}
	for _, id := range ready {
		if id == track.MediaID {
			if track.URL == "" {
				track.LocalPath = track.MediaID + ".m4a"
				track.URL = "/api/music/" + track.MediaID
				return true
			}
			return false
		}
	}
	return false
}

func (uc *MediaUsecase) EnsureRoomMedia(ctx context.Context, roomID string) bool {
	tracks, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	ps, _ := redisc.GetPlayback(ctx, uc.rdb, roomID)
	if ps == nil || len(tracks) == 0 {
		return false
	}

	var candidates []*entity.Track
	// Current track.
	if ps.CurrentIndex >= 0 && ps.CurrentIndex < len(tracks) {
		candidates = append(candidates, tracks[ps.CurrentIndex])
	}
	// Next track.
	nextIdx := ps.CurrentIndex + 1
	if nextIdx >= 0 && nextIdx < len(tracks) {
		candidates = append(candidates, tracks[nextIdx])
	}

	var pending []*entity.Track
	for _, t := range candidates {
		if t.Source == entity.TrackSourceUpload {
			continue
		}
		if t.MediaID != "" && t.URL == "" {
			pending = append(pending, t)
		}
	}
	if len(pending) == 0 {
		return false
	}

	items := make([]map[string]any, 0, len(pending))
	for _, t := range pending {
		items = append(items, map[string]any{"media_id": t.MediaID, "source_url": t.SourceURL})
	}
	ready, err := uc.mediaClient.Ensure(items)
	if err != nil {
		return false
	}

	readySet := make(map[string]bool)
	for _, id := range ready {
		readySet[id] = true
	}

	changed := false
	for _, t := range pending {
		if readySet[t.MediaID] {
			if t.URL == "" {
				changed = true
			}
			t.LocalPath = t.MediaID + ".m4a"
			t.URL = "/api/music/" + t.MediaID
		}
	}

	// Write updated tracks back to Redis.
	if changed {
		redisc.SetQueue(ctx, uc.rdb, roomID, tracks)
	}
	return changed
}

func (uc *MediaUsecase) FetchSubtitles(mediaID, lang string) (map[string]any, error) {
	return uc.mediaClient.MediaCaptions(mediaID, lang)
}

func (uc *MediaUsecase) StreamContent(mediaID, rangeHeader string) (int64, string, []byte, error) {
	return uc.mediaClient.Content(mediaID, rangeHeader)
}

func (uc *MediaUsecase) UpdateReferences(mediaIDs []string) error {
	return uc.mediaClient.UpdateReferences(mediaIDs)
}

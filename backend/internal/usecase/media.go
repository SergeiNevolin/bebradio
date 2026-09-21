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

	type pendingItem struct {
		idx     int
		id      string
		mediaID string
	}
	var pending []pendingItem
	// Current track. Only non-library sources (youtube today, future URL
	// providers too) need Ensure(); library-backed tracks (uploads etc.)
	// are served locally and never have an empty URL.
	if ps.CurrentIndex >= 0 && ps.CurrentIndex < len(tracks) {
		t := tracks[ps.CurrentIndex]
		if !entity.IsLibrarySource(t.Source) && t.MediaID != "" && t.URL == "" {
			pending = append(pending, pendingItem{ps.CurrentIndex, t.ID, t.MediaID})
		}
	}
	// Next track.
	nextIdx := ps.CurrentIndex + 1
	if nextIdx >= 0 && nextIdx < len(tracks) {
		t := tracks[nextIdx]
		if !entity.IsLibrarySource(t.Source) && t.MediaID != "" && t.URL == "" {
			pending = append(pending, pendingItem{nextIdx, t.ID, t.MediaID})
		}
	}
	if len(pending) == 0 {
		return false
	}

	items := make([]map[string]any, 0, len(pending))
	for _, p := range pending {
		t := tracks[p.idx]
		items = append(items, map[string]any{"media_id": p.mediaID, "source_url": t.SourceURL})
	}
	ready, err := uc.mediaClient.Ensure(items)
	if err != nil {
		return false
	}

	readySet := make(map[string]bool)
	for _, id := range ready {
		readySet[id] = true
	}

	// The queue may have changed (skip/remove/append) while Ensure() was in
	// flight — it does network I/O. Never write back the stale snapshot:
	// re-read and stamp resolved URLs only onto tracks still sitting at the
	// same index with the same ID. Anything else is retried on a later tick.
	// (The old code did a blind full-queue SetQueue here and resurrected
	// just-skipped tracks on every autoadvance tick.)
	fresh, _ := redisc.GetQueue(ctx, uc.rdb, roomID)
	changed := false
	for _, p := range pending {
		if !readySet[p.mediaID] {
			continue
		}
		if p.idx >= len(fresh) || fresh[p.idx].ID != p.id || fresh[p.idx].URL != "" {
			continue
		}
		fresh[p.idx].LocalPath = p.mediaID + ".m4a"
		fresh[p.idx].URL = "/api/music/" + p.mediaID
		if err := redisc.SetQueueAt(ctx, uc.rdb, roomID, p.idx, fresh[p.idx]); err != nil {
			continue
		}
		changed = true
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

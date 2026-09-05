package usecase

import (
	"log/slog"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

type MediaUsecase struct {
	mediaClient repository.MediaClient
	config      *config.Config
	log         *slog.Logger
}

func NewMediaUsecase(mediaClient repository.MediaClient, config *config.Config, log *slog.Logger) *MediaUsecase {
	return &MediaUsecase{mediaClient: mediaClient, config: config, log: log}
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
				track.URL = "/api/media/" + track.MediaID
				return true
			}
			return false
		}
	}
	return false
}

func (uc *MediaUsecase) EnsureRoomMedia(rm *entity.Room) bool {
	rm.Mu.RLock()
	tracks := make([]*entity.Track, 0)
	current := rm.CurrentTrackUnlocked()
	if current != nil {
		tracks = append(tracks, current)
	}
	nextIdx := rm.CurrentIndex + 1
	if nextIdx >= 0 && nextIdx < len(rm.Queue) {
		tracks = append(tracks, rm.Queue[nextIdx])
	}
	rm.Mu.RUnlock()

	var pending []*entity.Track
	for _, t := range tracks {
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

	changed := false
	readySet := make(map[string]bool)
	for _, id := range ready {
		readySet[id] = true
	}
	for _, t := range pending {
		if readySet[t.MediaID] {
			if t.URL == "" {
				changed = true
			}
			t.LocalPath = t.MediaID + ".m4a"
			t.URL = "/api/media/" + t.MediaID
		}
	}
	return changed
}

func (uc *MediaUsecase) FetchSubtitles(sourceURL, lang string) (map[string]any, error) {
	return uc.mediaClient.Captions(sourceURL, lang)
}

func (uc *MediaUsecase) StreamContent(mediaID, rangeHeader string) (int64, string, []byte, error) {
	return uc.mediaClient.Content(mediaID, rangeHeader)
}

func (uc *MediaUsecase) UpdateReferences(mediaIDs []string) error {
	return uc.mediaClient.UpdateReferences(mediaIDs)
}

package usecase

import (
	"log/slog"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
)

const RadioTag = "Radio"

type RadioUsecase struct {
	mediaClient repository.MediaClient
	config      *config.Config
	log         *slog.Logger
}

func NewRadioUsecase(mediaClient repository.MediaClient, config *config.Config, log *slog.Logger) *RadioUsecase {
	return &RadioUsecase{mediaClient: mediaClient, config: config, log: log}
}

func (uc *RadioUsecase) NeedsRefill(rm *entity.Room) bool {
	rm.Mu.RLock()
	defer rm.Mu.RUnlock()
	return uc.needsRefillUnlocked(rm)
}

func (uc *RadioUsecase) needsRefillUnlocked(rm *entity.Room) bool {
	return rm.AutoRadio && !rm.RadioFilling && len(rm.Queue) <= uc.config.RadioRefillAt && uc.seedURLUnlocked(rm) != ""
}

func (uc *RadioUsecase) seedURLUnlocked(rm *entity.Room) string {
	if rm.RadioSeedURL != "" {
		return rm.RadioSeedURL
	}
	if len(rm.Queue) > 0 {
		return rm.Queue[len(rm.Queue)-1].SourceURL
	}
	return ""
}

func (uc *RadioUsecase) Refill(rm *entity.Room) ([]*entity.Track, error) {
	rm.Mu.Lock()
	if !uc.needsRefillUnlocked(rm) {
		rm.Mu.Unlock()
		return nil, nil
	}
	rm.RadioFilling = true
	rm.Mu.Unlock()

	defer func() {
		rm.Mu.Lock()
		rm.RadioFilling = false
		rm.Mu.Unlock()
	}()

	seed := uc.seedURLUnlocked(rm)
	candidates, err := uc.mediaClient.Related(seed, uc.config.RadioBatch*4)
	if err != nil {
		return nil, err
	}

	rm.Mu.RLock()
	seenMediaIDs := make(map[string]bool)
	for _, t := range rm.Queue {
		if t.MediaID != "" {
			seenMediaIDs[t.MediaID] = true
		}
	}
	for mid := range rm.RadioSeen {
		seenMediaIDs[mid] = true
	}
	rm.Mu.RUnlock()

	var picked []*entity.Track
	for _, url := range candidates {
		if len(picked) >= uc.config.RadioBatch {
			break
		}
		info, err := uc.mediaClient.Resolve(url)
		if err != nil {
			continue
		}
		mediaID, _ := info["media_id"].(string)
		if mediaID == "" || seenMediaIDs[mediaID] {
			continue
		}
		duration, _ := info["duration"].(float64)
		if int(duration) > uc.config.MaxDuration {
			continue
		}

		rm.Mu.Lock()
		rm.RadioSeen[mediaID] = true
		rm.Mu.Unlock()
		seenMediaIDs[mediaID] = true

		track := entity.TrackFromYouTube(info, RadioTag)
		track.ID = shortID(8)
		picked = append(picked, track)
	}

	if len(picked) > 0 {
		rm.Mu.Lock()
		if !rm.IsPlaying {
			rm.IsPlaying = true
			rm.Position = 0.0
			rm.LastSyncAt = time.Now()
		}
		rm.Mu.Unlock()
	}

	return picked, nil
}

func shortID(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

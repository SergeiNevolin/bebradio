package usecase

import (
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/pkg/id"
)

var (
	ErrMashupQuota      = &BusinessError{Code: 409, Message: "Upload quota reached, delete an old mashup first"}
	ErrMashupNotFound   = &BusinessError{Code: 404, Message: "Mashup not found"}
	ErrMashupNotOwner   = &BusinessError{Code: 403, Message: "You can only delete your own mashups"}
	ErrMashupUploadFail = &BusinessError{Code: 502, Message: "Media service could not accept the upload"}
	ErrMashupMediaDown  = &BusinessError{Code: 502, Message: "Media service unavailable, mashup not deleted"}
)

// vars, not consts, so tests can shrink them.
var (
	mashupPollInterval = 2 * time.Second
	mashupPollTimeout  = 5 * time.Minute
)

const mashupTitleMax = 200

type MashupUsecase struct {
	repo        repository.MashupRepository
	mediaClient repository.MediaClient
	config      *config.Config
	log         *slog.Logger
}

func NewMashupUsecase(repo repository.MashupRepository, mediaClient repository.MediaClient, cfg *config.Config, log *slog.Logger) *MashupUsecase {
	return &MashupUsecase{repo: repo, mediaClient: mediaClient, config: cfg, log: log}
}

func (uc *MashupUsecase) List(query string, limit, offset int) ([]map[string]any, error) {
	items, err := uc.repo.List(strings.TrimSpace(query), limit, offset)
	if err != nil {
		return nil, err
	}
	return toDicts(items), nil
}

func (uc *MashupUsecase) ListMine(ownerID string) ([]map[string]any, error) {
	items, err := uc.repo.ListByOwner(ownerID)
	if err != nil {
		return nil, err
	}
	return toDicts(items), nil
}

func (uc *MashupUsecase) Get(mashupID string) (*entity.Mashup, error) {
	m, err := uc.repo.FindByID(mashupID)
	if err != nil {
		return nil, ErrMashupNotFound
	}
	return m, nil
}

// Create enforces the per-user quota, inserts a processing row, streams the file
// to media-service and starts a poller that flips the row to ready/failed.
func (uc *MashupUsecase) Create(ownerID, title, artist, filename string, body io.Reader) (*entity.Mashup, error) {
	count, err := uc.repo.CountByOwner(ownerID)
	if err != nil {
		return nil, err
	}
	if count >= uc.config.MashupUserQuota {
		return nil, ErrMashupQuota
	}

	m := &entity.Mashup{
		ID:        id.NewHex(8),
		OwnerID:   ownerID,
		Title:     cleanTitle(title, filename),
		Artist:    clip(strings.TrimSpace(artist), mashupTitleMax),
		MediaID:   id.NewHex(16),
		Status:    "processing",
		CreatedAt: time.Now(),
	}
	if err := uc.repo.Create(m); err != nil {
		return nil, err
	}

	if err := uc.mediaClient.UploadMashup(m.MediaID, filename, body); err != nil {
		uc.log.Error("mashup upload to media service failed", "mashup_id", m.ID, "error", err)
		_ = uc.repo.UpdateStatus(m.ID, "failed", "upload failed", 0, 0, false)
		m.Status = "failed"
		m.Error = "upload failed"
		return m, ErrMashupUploadFail
	}

	go uc.poll(m.ID, m.MediaID)
	return m, nil
}

func (uc *MashupUsecase) Delete(mashupID, userID string) error {
	m, err := uc.repo.FindByID(mashupID)
	if err != nil {
		return ErrMashupNotFound
	}
	if m.OwnerID != userID {
		return ErrMashupNotOwner
	}
	if err := uc.mediaClient.DeleteMashup(m.MediaID); err != nil {
		uc.log.Error("mashup delete on media service failed", "mashup_id", mashupID, "error", err)
		return ErrMashupMediaDown
	}
	return uc.repo.Delete(mashupID)
}

// ResumeProcessing re-attaches a poller to every row still marked processing.
// Called once at startup so a restart never leaves an eternal "processing".
func (uc *MashupUsecase) ResumeProcessing() {
	items, err := uc.repo.ListProcessing()
	if err != nil {
		uc.log.Error("could not list processing mashups", "error", err)
		return
	}
	for _, m := range items {
		uc.log.Info("resuming mashup poll", "mashup_id", m.ID)
		go uc.poll(m.ID, m.MediaID)
	}
}

func (uc *MashupUsecase) poll(mashupID, mediaID string) {
	deadline := time.Now().Add(mashupPollTimeout)
	ticker := time.NewTicker(mashupPollInterval)
	defer ticker.Stop()

	for range ticker.C {
		if time.Now().After(deadline) {
			uc.log.Warn("mashup processing timed out", "mashup_id", mashupID)
			_ = uc.repo.UpdateStatus(mashupID, "failed", "processing timed out", 0, 0, false)
			return
		}

		st, err := uc.mediaClient.MashupStatus(mediaID)
		if err != nil {
			uc.log.Warn("mashup status poll failed", "mashup_id", mashupID, "error", err)
			continue
		}

		switch status, _ := st["status"].(string); status {
		case "ready":
			duration := int(toFloat(st["duration"]))
			hasCover, _ := st["has_cover"].(bool)
			if err := uc.repo.UpdateStatus(mashupID, "ready", "", duration, 0, hasCover); err != nil {
				uc.log.Error("mashup status update failed", "mashup_id", mashupID, "error", err)
			}
			return
		case "failed":
			msg, _ := st["error"].(string)
			if msg == "" {
				msg = "transcoding failed"
			}
			_ = uc.repo.UpdateStatus(mashupID, "failed", msg, 0, 0, false)
			return
		}
	}
}

func toDicts(items []*entity.Mashup) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, m := range items {
		out = append(out, m.ToDict())
	}
	return out
}

func cleanTitle(title, filename string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		base := filepath.Base(filename)
		t = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if t == "" {
		t = "Untitled"
	}
	return clip(t, mashupTitleMax)
}

func clip(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

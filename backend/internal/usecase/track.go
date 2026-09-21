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
)

var (
	ErrTrackQuota      = &BusinessError{Code: 409, Message: "Upload quota reached, delete an old track first"}
	ErrTrackNotFound   = &BusinessError{Code: 404, Message: "Track not found"}
	ErrTrackNotOwner   = &BusinessError{Code: 403, Message: "You can only modify your own tracks"}
	ErrTrackUploadFail = &BusinessError{Code: 502, Message: "music service could not accept the upload"}
	ErrTrackMediaDown  = &BusinessError{Code: 502, Message: "music service unavailable, track not deleted"}
	ErrTrackCoverFail  = &BusinessError{Code: 502, Message: "music service could not accept the cover"}
)

// vars, not consts, so tests can shrink them.
var (
	trackPollInterval = 2 * time.Second
	trackPollTimeout  = 5 * time.Minute
)

const trackTitleMax = 200

type TrackUsecase struct {
	repo        repository.TrackRepository
	mediaClient repository.MediaClient
	config      *config.Config
	log         *slog.Logger
}

func NewTrackUsecase(repo repository.TrackRepository, mediaClient repository.MediaClient, cfg *config.Config, log *slog.Logger) *TrackUsecase {
	return &TrackUsecase{repo: repo, mediaClient: mediaClient, config: cfg, log: log}
}

func (uc *TrackUsecase) List(query, sort string, limit, offset int, viewerID string) ([]map[string]any, error) {
	if sort != "top" {
		sort = "recent"
	}
	items, err := uc.repo.List(strings.TrimSpace(query), sort, limit, offset, viewerID)
	if err != nil {
		return nil, err
	}
	return toDicts(items), nil
}

func (uc *TrackUsecase) ListMine(ownerID string) ([]map[string]any, error) {
	items, err := uc.repo.ListByOwner(ownerID)
	if err != nil {
		return nil, err
	}
	return toDicts(items), nil
}

func (uc *TrackUsecase) ListLiked(userID string, limit, offset int) ([]map[string]any, error) {
	items, err := uc.repo.ListLikedByUser(userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return toDicts(items), nil
}

func (uc *TrackUsecase) Get(trackID, viewerID string) (*entity.Track, error) {
	m, err := uc.repo.Detail(trackID, viewerID)
	if err != nil {
		return nil, ErrTrackNotFound
	}
	return m, nil
}

// FindByID returns the raw row (any track, library or queue copy) for
// streaming endpoints that only need the media id.
func (uc *TrackUsecase) FindByID(trackID string) (*entity.Track, error) {
	m, err := uc.repo.FindByID(trackID)
	if err != nil {
		return nil, ErrTrackNotFound
	}
	return m, nil
}

// Like / Unlike are idempotent; they return {"likes": n, "liked": bool}.
func (uc *TrackUsecase) Like(trackID, userID string) (map[string]any, error) {
	if _, err := uc.repo.FindByID(trackID); err != nil {
		return nil, ErrTrackNotFound
	}
	n, err := uc.repo.Like(trackID, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"likes": n, "liked": true}, nil
}

func (uc *TrackUsecase) Unlike(trackID, userID string) (map[string]any, error) {
	if _, err := uc.repo.FindByID(trackID); err != nil {
		return nil, ErrTrackNotFound
	}
	n, err := uc.repo.Unlike(trackID, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"likes": n, "liked": false}, nil
}

// UpdateMetadata changes the title and artist of a library track.
// Owner-only (admins can bypass via DeleteAsAdmin + re-import).
func (uc *TrackUsecase) UpdateMetadata(trackID, userID, title, artist string) (*entity.Track, error) {
	title = strings.TrimSpace(title)
	artist = strings.TrimSpace(artist)
	if title == "" {
		return nil, &BusinessError{Code: 422, Message: "Title cannot be empty"}
	}
	if len([]rune(title)) > trackTitleMax {
		return nil, &BusinessError{Code: 422, Message: "Title too long"}
	}
	m, err := uc.repo.FindByID(trackID)
	if err != nil {
		return nil, ErrTrackNotFound
	}
	if m.OwnerID != userID {
		return nil, ErrTrackNotOwner
	}
	if err := uc.repo.UpdateMetadata(trackID, title, artist); err != nil {
		return nil, err
	}
	m.Title = title
	m.Artist = artist
	return m, nil
}

// SetCover streams a cover image through music-service (owner only) and flips
// has_cover / cover_updated_at on the row.
func (uc *TrackUsecase) SetCover(trackID, userID, filename string, body io.Reader) error {
	m, err := uc.repo.FindByID(trackID)
	if err != nil {
		return ErrTrackNotFound
	}
	if m.OwnerID != userID {
		return ErrTrackNotOwner
	}
	if err := uc.mediaClient.UploadTrackCover(m.MediaID, filename, body); err != nil {
		uc.log.Error("track cover upload to music service failed", "track_id", trackID, "error", err)
		return ErrTrackCoverFail
	}
	return uc.repo.SetCoverUploaded(trackID)
}

// Import stores already-fetched audio (+ optional cover) as a new library
// track owned by ownerID — e.g. an admin saving a YouTube queue track into
// bebradio. It mirrors Create but takes bytes instead of a multipart body:
// the row starts "processing" and the poller flips it once music-service
// transcodes. The cover is best-effort: a failed cover never fails the import.
func (uc *TrackUsecase) Import(ownerID, title, artist, filename string, audio io.Reader, coverName string, cover io.Reader) (*entity.Track, error) {
	count, err := uc.repo.CountByOwner(ownerID)
	if err != nil {
		return nil, err
	}
	if count >= uc.config.MashupUserQuota {
		return nil, ErrTrackQuota
	}

	res, err := uc.mediaClient.UploadTrack(filename, audio)
	if err != nil {
		uc.log.Error("track import to music service failed", "error", err)
		return nil, ErrTrackUploadFail
	}
	rowID, _ := res["id"].(string)
	mediaID, _ := res["media_id"].(string)
	if rowID == "" || mediaID == "" {
		uc.log.Error("import returned no track identity")
		return nil, ErrTrackUploadFail
	}

	m := entity.TrackFromUpload(rowID, ownerID, cleanTitle(title, filename),
		clip(strings.TrimSpace(artist), trackTitleMax))
	m.MediaID = mediaID
	if err := uc.repo.Create(m); err != nil {
		// Don't orphan the file in MinIO when the row can't be stored.
		_ = uc.mediaClient.DeleteTrack(mediaID)
		return nil, err
	}

	if cover != nil {
		if err := uc.SetCover(m.ID, ownerID, coverName, cover); err != nil {
			uc.log.Warn("imported track cover upload failed", "track_id", m.ID, "error", err)
		}
	}

	go uc.poll(m.ID, m.MediaID)
	return m, nil
}

// Create enforces the per-user quota, streams the file to music-service —
// which mints the track id and the media id along with the data — then
// inserts the processing row and starts a poller that flips it to
// ready/failed.
func (uc *TrackUsecase) Create(ownerID, title, artist, filename string, body io.Reader) (*entity.Track, error) {
	count, err := uc.repo.CountByOwner(ownerID)
	if err != nil {
		return nil, err
	}
	if count >= uc.config.MashupUserQuota {
		return nil, ErrTrackQuota
	}

	res, err := uc.mediaClient.UploadTrack(filename, body)
	if err != nil {
		uc.log.Error("track upload to music service failed", "error", err)
		return nil, ErrTrackUploadFail
	}
	rowID, _ := res["id"].(string)
	mediaID, _ := res["media_id"].(string)
	if rowID == "" || mediaID == "" {
		uc.log.Error("upload returned no track identity")
		return nil, ErrTrackUploadFail
	}

	m := entity.TrackFromUpload(rowID, ownerID, cleanTitle(title, filename),
		clip(strings.TrimSpace(artist), trackTitleMax))
	m.MediaID = mediaID
	if err := uc.repo.Create(m); err != nil {
		// Don't orphan the file in MinIO when the row can't be stored.
		_ = uc.mediaClient.DeleteTrack(mediaID)
		return nil, err
	}

	go uc.poll(m.ID, m.MediaID)
	return m, nil
}

func (uc *TrackUsecase) Delete(trackID, userID string) error {
	m, err := uc.repo.FindByID(trackID)
	if err != nil {
		return ErrTrackNotFound
	}
	if m.OwnerID != userID {
		return ErrTrackNotOwner
	}
	if err := uc.mediaClient.DeleteTrack(m.MediaID); err != nil {
		uc.log.Error("track delete on music service failed", "track_id", trackID, "error", err)
		return ErrTrackMediaDown
	}
	return uc.repo.Delete(trackID)
}

func (uc *TrackUsecase) DeleteAsAdmin(trackID string) error {
	m, err := uc.repo.FindByID(trackID)
	if err != nil {
		return ErrTrackNotFound
	}
	if err := uc.mediaClient.DeleteTrack(m.MediaID); err != nil {
		uc.log.Error("track delete on music service failed", "track_id", trackID, "error", err)
		return ErrTrackMediaDown
	}
	return uc.repo.Delete(trackID)
}

// ResumeProcessing re-attaches a poller to every row still marked processing.
// Called once at startup so a restart never leaves an eternal "processing".
func (uc *TrackUsecase) ResumeProcessing() {
	items, err := uc.repo.ListProcessing()
	if err != nil {
		uc.log.Error("could not list processing tracks", "error", err)
		return
	}
	for _, m := range items {
		uc.log.Info("resuming track poll", "track_id", m.ID)
		go uc.poll(m.ID, m.MediaID)
	}
}

func (uc *TrackUsecase) poll(trackID, mediaID string) {
	deadline := time.Now().Add(trackPollTimeout)
	ticker := time.NewTicker(trackPollInterval)
	defer ticker.Stop()

	for range ticker.C {
		if time.Now().After(deadline) {
			uc.log.Warn("track processing timed out", "track_id", trackID)
			_ = uc.repo.UpdateStatus(trackID, "failed", "processing timed out", 0, 0, false)
			return
		}

		st, err := uc.mediaClient.TrackUploadStatus(mediaID)
		if err != nil {
			uc.log.Warn("track status poll failed", "track_id", trackID, "error", err)
			continue
		}

		switch status, _ := st["status"].(string); status {
		case "ready":
			duration := int(toFloat(st["duration"]))
			hasCover, _ := st["has_cover"].(bool)
			if err := uc.repo.UpdateStatus(trackID, "ready", "", duration, 0, hasCover); err != nil {
				uc.log.Error("track status update failed", "track_id", trackID, "error", err)
			}
			return
		case "failed":
			msg, _ := st["error"].(string)
			if msg == "" {
				msg = "transcoding failed"
			}
			_ = uc.repo.UpdateStatus(trackID, "failed", msg, 0, 0, false)
			return
		}
	}
}

func toDicts(items []*entity.Track) []map[string]any {
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
	return clip(t, trackTitleMax)
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




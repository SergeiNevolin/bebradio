package entity

import (
	"strconv"
	"time"
)

// Track sources: "youtube" for resolved YouTube entries, "upload" for
// user-uploaded files transcoded by music-service. One model backs both the
// room queue and the standalone library: a queue row is just a track with
// room_id set, a library row has room_id NULL.
//
// Adding a new source (e.g. "spotify", "soundcloud") means:
//   1. adding a TrackSource* constant here,
//   2. teaching the queue dispatcher (handleAddToQueue/resolveURLTrack) and,
//      for library-backed sources, the importer that writes tracks rows,
//   3. no changes to playback, votes or persistence: they key off IDs,
//      StreamURL and queue order only.
const (
	TrackSourceYouTube = "youtube"
	TrackSourceUpload  = "upload"

	TrackStatusReady      = "ready"
	TrackStatusProcessing = "processing"
	TrackStatusFailed     = "failed"
)

// IsLibrarySource reports whether tracks of this source live in the tracks
// table and are added to a room by id (uploads today; future imports reuse
// the same path). URL-resolved sources (YouTube today) return false.
func IsLibrarySource(source string) bool {
	switch source {
	case TrackSourceUpload:
		return true
	default:
		return false
	}
}

// IsQueueableLibraryTrack reports whether a stored library row may be copied
// into a room queue. Any ready row qualifies regardless of source, so future
// library-backed sources become queueable without handler changes.
func IsQueueableLibraryTrack(lib *Track) bool {
	return lib != nil && lib.Status == TrackStatusReady
}

type Track struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist"`
	Duration  int       `json:"duration"`
	URL       string    `json:"url"`
	Thumbnail string    `json:"thumbnail"`
	AddedBy   string    `json:"added_by"`
	AddedAt   time.Time `json:"added_at"`

	// YouTube plumbing (empty for uploads).
	SourceURL string `json:"source_url,omitempty"`
	LocalPath string `json:"local_path,omitempty"`
	MediaID   string `json:"media_id,omitempty"`
	// Queue order, meaningful only on room rows.
	Position int `json:"position,omitempty"`

	// Upload metadata (empty for YouTube tracks). The audio file lives on
	// music-service; this row is the source of truth for its state.
	OwnerID   string `json:"owner_id"`
	SizeBytes int64  `json:"size_bytes"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	HasCover  bool   `json:"has_cover"`
	Plays     int    `json:"plays"`
	Likes     int    `json:"likes"`

	// Populated per request, not stored on the row itself.
	OwnerName      string     `json:"-"`
	Liked          bool       `json:"-"`
	CoverUpdatedAt *time.Time `json:"-"`
}

func TrackFromYouTube(info map[string]any, addedBy string) *Track {
	return &Track{
		ID:        getString(info, "id", ""),
		Source:    TrackSourceYouTube,
		Status:    TrackStatusReady,
		Title:     getString(info, "title", "Unknown"),
		Artist:    getString(info, "artist", "Unknown"),
		Thumbnail: getString(info, "thumbnail", ""),
		Duration:  getInt(info, "duration"),
		AddedBy:   addedBy,
		SourceURL: getString(info, "source_url", ""),
		MediaID:   getString(info, "media_id", ""),
		AddedAt:   time.Now(),
	}
}

// TrackFromUpload builds the "processing" row for a fresh user upload.
func TrackFromUpload(id, ownerID, title, artist string) *Track {
	return &Track{
		ID:       id,
		Source:   TrackSourceUpload,
		OwnerID:  ownerID,
		Title:    title,
		Artist:   artist,
		Status:   TrackStatusProcessing,
		AddedBy:  ownerID,
		AddedAt:  time.Now(),
	}
}

// QueueCopyFromLibrary snapshots a ready library track into a room queue row
// under the library id itself (votes are scoped per room, so sharing the id
// across rooms is safe). AddedBy/AddedAt belong to the queue entry, everything
// else — to the track. Playable URLs derive from the id, so URL/Thumbnail
// stay empty here just like on the library row.
// Works for every library-backed source (uploads today, new imports tomorrow).
func QueueCopyFromLibrary(lib *Track, addedBy string) *Track {
	return &Track{
		ID:        lib.ID,
		Source:    lib.Source,
		Title:     lib.Title,
		Artist:    lib.Artist,
		Duration:  lib.Duration,
		AddedBy:   addedBy,
		AddedAt:   time.Now(),
		MediaID:   lib.MediaID,
		OwnerID:   lib.OwnerID,
		OwnerName: lib.OwnerName,
		SizeBytes: lib.SizeBytes,
		Status:    TrackStatusReady,
		HasCover:  lib.HasCover,
	}
}

// QueueCopyFromUpload is kept for callers written before the generic
// QueueCopyFromLibrary existed.
func QueueCopyFromUpload(lib *Track, addedBy string) *Track {
	return QueueCopyFromLibrary(lib, addedBy)
}

// StreamURL is the playable URL once the file is ready ("" otherwise).
// MediaID is never exposed directly, only baked into a non-guessable URL.
func (t *Track) StreamURL() string {
	if IsLibrarySource(t.Source) {
		// Library-backed sources (uploads today, new imports tomorrow):
		// playable URL derives from the id; a row carrying its own direct
		// URL keeps it.
		if t.Status != TrackStatusReady {
			return ""
		}
		if t.URL != "" {
			return t.URL
		}
		return "/api/tracks/" + t.ID + "/audio"
	}
	return t.URL
}

func (t *Track) coverURL() string {
	if !t.HasCover {
		return ""
	}
	ver := t.AddedAt.Unix()
	if t.CoverUpdatedAt != nil {
		ver = t.CoverUpdatedAt.Unix()
	}
	// ?v= busts client/CDN caches after the owner replaces the cover.
	return "/api/tracks/" + t.ID + "/cover?v=" + strconv.FormatInt(ver, 10)
}

func (t *Track) ToDict() map[string]any {
	out := map[string]any{
		"id":         t.ID,
		"source":     t.Source,
		"title":      t.Title,
		"artist":     t.Artist,
		"duration":   t.Duration,
		"url":        t.StreamURL(),
		"thumbnail":  t.Thumbnail,
		"added_by":   t.AddedBy,
		"owner_id":   t.OwnerID,
		"owner_name": t.OwnerName,
		"size_bytes": t.SizeBytes,
		"status":     t.Status,
		"has_cover":  t.HasCover,
		"plays":      t.Plays,
		"likes":      t.Likes,
		"liked":      t.Liked,
		"created_at": t.AddedAt,
	}
	if t.Thumbnail == "" {
		out["thumbnail"] = t.coverURL()
	}
	if t.Status == TrackStatusFailed && t.Error != "" {
		out["error"] = t.Error
	}
	return out
}

func getString(m map[string]any, key, fallback string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return fallback
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return 0
}

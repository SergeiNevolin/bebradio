package entity

import (
	"strconv"
	"time"
)

// Mashup is a user-uploaded audio track shown in the standalone /mashup section.
// The audio file lives on media-service; this row is the source of truth for
// its processing state.
type Mashup struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist"`
	MediaID   string    `json:"-"`
	Duration  int       `json:"duration"`
	SizeBytes int64     `json:"size_bytes"`
	Status    string    `json:"status"`
	Error     string    `json:"error"`
	HasCover  bool      `json:"has_cover"`
	Plays     int       `json:"plays"`
	Likes     int       `json:"likes"`
	CreatedAt time.Time `json:"created_at"`

	// Populated per request, not stored on the row itself.
	OwnerName      string     `json:"-"`
	Liked          bool       `json:"-"`
	CoverUpdatedAt *time.Time `json:"-"`
}

// ToDict mirrors entity/track.go: MediaID is never exposed directly, only as a
// non-guessable stream/cover URL once the file is ready.
func (m *Mashup) ToDict() map[string]any {
	out := map[string]any{
		"id":         m.ID,
		"owner_id":   m.OwnerID,
		"owner_name": m.OwnerName,
		"title":      m.Title,
		"artist":     m.Artist,
		"duration":   m.Duration,
		"size_bytes": m.SizeBytes,
		"status":     m.Status,
		"has_cover":  m.HasCover,
		"plays":      m.Plays,
		"likes":      m.Likes,
		"liked":      m.Liked,
		"created_at": m.CreatedAt,
	}
	if m.Status == "ready" {
		out["stream_url"] = "/api/mashups/media/" + m.MediaID
	}
	if m.HasCover {
		// ?v= busts client/CDN caches after the owner replaces the cover.
		ver := m.CreatedAt.Unix()
		if m.CoverUpdatedAt != nil {
			ver = m.CoverUpdatedAt.Unix()
		}
		out["cover_url"] = "/api/mashups/media/" + m.MediaID + "/cover?v=" + strconv.FormatInt(ver, 10)
	}
	if m.Status == "failed" && m.Error != "" {
		out["error"] = m.Error
	}
	return out
}

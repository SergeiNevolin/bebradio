package entity

import "time"

type Track struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Artist     string    `json:"artist"`
	URL        string    `json:"url"`
	Thumbnail  string    `json:"thumbnail"`
	Duration   int       `json:"duration"`
	AddedBy    string    `json:"added_by"`
	AddedAt    time.Time `json:"added_at"`
	SourceURL  string    `json:"-"`
	LocalPath  string    `json:"-"`
	MediaID    string    `json:"-"`
	Position   int       `json:"-"`
}

func TrackFromYouTube(info map[string]any, addedBy string) *Track {
	return &Track{
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

func (t *Track) ToDict() map[string]any {
	return map[string]any{
		"id":        t.ID,
		"title":     t.Title,
		"artist":    t.Artist,
		"url":       t.URL,
		"thumbnail": t.Thumbnail,
		"duration":  t.Duration,
		"added_by":  t.AddedBy,
	}
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

package entity

import (
	"testing"
)

func TestTrackDefaults(t *testing.T) {
	track := &Track{}

	if track.ID != "" {
		t.Errorf("expected empty default ID, got '%s'", track.ID)
	}
	if track.Title != "" {
		t.Errorf("expected empty title, got '%s'", track.Title)
	}
	if track.AddedBy != "" {
		t.Errorf("expected empty added_by, got '%s'", track.AddedBy)
	}
	if track.Duration != 0 {
		t.Errorf("expected duration 0, got %d", track.Duration)
	}
}

func TestTrackToDict(t *testing.T) {
	track := &Track{
		ID:        "t1",
		Title:     "My Song",
		Artist:    "My Artist",
		URL:       "http://example.com/song.mp3",
		Thumbnail: "http://example.com/thumb.jpg",
		Duration:  240,
		AddedBy:   "Alice",
	}

	dict := track.ToDict()
	if dict["id"] != "t1" {
		t.Errorf("expected id 't1', got '%v'", dict["id"])
	}
	if dict["title"] != "My Song" {
		t.Errorf("expected title 'My Song', got '%v'", dict["title"])
	}
	if dict["artist"] != "My Artist" {
		t.Errorf("expected artist 'My Artist', got '%v'", dict["artist"])
	}
	if dict["url"] != "http://example.com/song.mp3" {
		t.Errorf("expected url, got '%v'", dict["url"])
	}
	if dict["thumbnail"] != "http://example.com/thumb.jpg" {
		t.Errorf("expected thumbnail, got '%v'", dict["thumbnail"])
	}
	if dict["duration"] != 240 {
		t.Errorf("expected duration 240, got '%v'", dict["duration"])
	}
	if dict["added_by"] != "Alice" {
		t.Errorf("expected added_by 'Alice', got '%v'", dict["added_by"])
	}
}

func TestTrackToDictMinimal(t *testing.T) {
	track := &Track{}
	dict := track.ToDict()

	if dict["title"] != "" {
		t.Errorf("expected empty title, got '%v'", dict["title"])
	}
	if dict["duration"] != 0 {
		t.Errorf("expected duration 0, got '%v'", dict["duration"])
	}
}

func TestTrackFromYouTube(t *testing.T) {
	info := map[string]any{
		"title":      "Test Video",
		"artist":     "Test Channel",
		"thumbnail":  "http://example.com/thumb.jpg",
		"duration":   300,
		"source_url": "https://youtube.com/watch?v=abc",
		"media_id":   "media123",
	}

	track := TrackFromYouTube(info, "Bob")

	if track.Title != "Test Video" {
		t.Errorf("expected title 'Test Video', got '%s'", track.Title)
	}
	if track.Artist != "Test Channel" {
		t.Errorf("expected artist 'Test Channel', got '%s'", track.Artist)
	}
	if track.Thumbnail != "http://example.com/thumb.jpg" {
		t.Errorf("expected thumbnail, got '%s'", track.Thumbnail)
	}
	if track.Duration != 300 {
		t.Errorf("expected duration 300, got %d", track.Duration)
	}
	if track.AddedBy != "Bob" {
		t.Errorf("expected added_by 'Bob', got '%s'", track.AddedBy)
	}
	if track.SourceURL != "https://youtube.com/watch?v=abc" {
		t.Errorf("expected source_url, got '%s'", track.SourceURL)
	}
	if track.MediaID != "media123" {
		t.Errorf("expected media_id 'media123', got '%s'", track.MediaID)
	}
	if track.AddedAt.IsZero() {
		t.Error("expected AddedAt to be set")
	}
}

func TestTrackFromYouTubeEmpty(t *testing.T) {
	info := map[string]any{}

	track := TrackFromYouTube(info, "Anonymous")

	if track.Title != "Unknown" {
		t.Errorf("expected default title 'Unknown', got '%s'", track.Title)
	}
	if track.Artist != "Unknown" {
		t.Errorf("expected default artist 'Unknown', got '%s'", track.Artist)
	}
	if track.AddedBy != "Anonymous" {
		t.Errorf("expected added_by 'Anonymous', got '%s'", track.AddedBy)
	}
	if track.SourceURL != "" {
		t.Errorf("expected empty source_url, got '%s'", track.SourceURL)
	}
}

func TestTrackFromYouTubeFloatDuration(t *testing.T) {
	info := map[string]any{
		"duration": 300.5,
	}

	track := TrackFromYouTube(info, "User")
	if track.Duration != 300 {
		t.Errorf("expected duration 300 from float64, got %d", track.Duration)
	}
}

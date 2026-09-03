// Package youtube resolves YouTube watch URLs into playable audio streams,
// searches the catalogue, and pulls related tracks and captions.
//
// All of it is done by shelling out to yt-dlp, which is the only component that
// tracks YouTube's frequent player changes. Every call is therefore slow,
// fallible and network-bound: the Client interface exists so the rest of the
// service can be exercised without it.
package youtube

import (
	"context"
	"regexp"
	"strconv"
	"time"
)

// TrackInfo is everything known about a video after a lookup.
type TrackInfo struct {
	Title     string
	Artist    string
	Thumbnail string
	Duration  int
	StreamURL string
	SourceURL string
	// ExpiresAt is the epoch second at which StreamURL stops working.
	ExpiresAt float64
}

// StreamInfo is a freshly resolved playable URL for an already-known video.
type StreamInfo struct {
	StreamURL string
	ExpiresAt float64
}

// SearchResult is one hit from a catalogue search.
type SearchResult struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Thumbnail string `json:"thumbnail"`
	Duration  int    `json:"duration"`
	URL       string `json:"url"`
}

// Cue is one timed line of captions.
type Cue struct {
	Start float64 `json:"start"`
	Dur   float64 `json:"dur"`
	Text  string  `json:"text"`
}

// Subtitles is a caption track for one video. An empty Cues means the video has
// no usable captions, or the lookup failed.
type Subtitles struct {
	Lang string `json:"lang"`
	Auto bool   `json:"auto"`
	Cues []Cue  `json:"cues"`
}

// Client is the service's view of YouTube.
//
// Every method returns an error for a failed lookup rather than a zero value,
// so callers can tell "this video has no captions" from "we could not reach
// YouTube" and log accordingly.
type Client interface {
	// FetchTrack looks up a video and resolves a playable audio stream for it.
	FetchTrack(ctx context.Context, url string) (TrackInfo, error)
	// ResolveStream re-resolves just the playable URL of a known video.
	ResolveStream(ctx context.Context, sourceURL string) (StreamInfo, error)
	// FetchRelated returns watch URLs from the Mix (radio) playlist of a video.
	FetchRelated(ctx context.Context, sourceURL string, limit int) ([]string, error)
	// Search returns up to limit catalogue hits for a query.
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
	// FetchSubtitles returns timed caption cues for a video. Pass an empty lang
	// to take the video's own language, falling back to English.
	FetchSubtitles(ctx context.Context, sourceURL, lang string) (Subtitles, error)
}

// defaultStreamTTL is how long a resolved stream URL is assumed to last when it
// carries no expiry of its own.
const defaultStreamTTL = 5 * time.Hour

var (
	videoIDRe = regexp.MustCompile(`(?:v=|youtu\.be/|/shorts/|/embed/)([\w-]{11})`)
	expireRe  = regexp.MustCompile(`[?&/]expire[=/](\d+)`)
)

// VideoID extracts the eleven-character YouTube video id from a watch, short or
// embed URL. It returns an empty string when the URL is not recognisable.
func VideoID(url string) string {
	m := videoIDRe.FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1]
}

// WatchURL returns the canonical watch URL for a video id.
func WatchURL(id string) string {
	return "https://www.youtube.com/watch?v=" + id
}

// ParseStreamExpiry reads the epoch second at which a resolved googlevideo URL
// stops working, falling back to a conservative default when the URL carries no
// expiry.
func ParseStreamExpiry(streamURL string) float64 {
	if m := expireRe.FindStringSubmatch(streamURL); m != nil {
		if secs, err := strconv.ParseFloat(m[1], 64); err == nil {
			return secs
		}
	}
	return float64(time.Now().Add(defaultStreamTTL).UnixNano()) / float64(time.Second)
}

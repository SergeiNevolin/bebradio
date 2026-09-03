// Package youtubetest provides a scriptable stand-in for youtube.Client.
//
// The real client shells out to yt-dlp and talks to the network, so nothing
// above it could be tested without one of these.
package youtubetest

import (
	"context"
	"fmt"
	"sync"

	"github.com/leenzstra/bebradio/backend/internal/youtube"
)

// Fake is a youtube.Client whose answers the test decides.
//
// Every field is optional: an unset hook returns a plausible default, so a test
// only has to script the behaviour it actually cares about. It is safe for
// concurrent use, which the room service's background refills require.
type Fake struct {
	mu sync.Mutex

	// FetchTrackFn, when set, answers FetchTrack.
	FetchTrackFn func(ctx context.Context, url string) (youtube.TrackInfo, error)
	// ResolveStreamFn, when set, answers ResolveStream.
	ResolveStreamFn func(ctx context.Context, sourceURL string) (youtube.StreamInfo, error)
	// RelatedFn, when set, answers FetchRelated.
	RelatedFn func(ctx context.Context, sourceURL string, limit int) ([]string, error)
	// SearchFn, when set, answers Search.
	SearchFn func(ctx context.Context, query string, limit int) ([]youtube.SearchResult, error)
	// SubtitlesFn, when set, answers FetchSubtitles.
	SubtitlesFn func(ctx context.Context, sourceURL, lang string) (youtube.Subtitles, error)

	// Calls counts each method by name, for assertions about how often the
	// service reached for the network.
	Calls map[string]int
}

// New returns a fake with no scripted behaviour.
func New() *Fake { return &Fake{Calls: map[string]int{}} }

func (f *Fake) record(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Calls == nil {
		f.Calls = map[string]int{}
	}
	f.Calls[name]++
}

// CallCount returns how many times a method was called.
func (f *Fake) CallCount(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Calls[name]
}

func (f *Fake) FetchTrack(ctx context.Context, url string) (youtube.TrackInfo, error) {
	f.record("FetchTrack")
	if f.FetchTrackFn != nil {
		return f.FetchTrackFn(ctx, url)
	}
	return youtube.TrackInfo{
		Title:     "Test Track",
		Artist:    "Test Artist",
		Thumbnail: "https://img.example/thumb.jpg",
		Duration:  180,
		StreamURL: "https://stream.example/" + youtube.VideoID(url),
		SourceURL: url,
		ExpiresAt: farFuture,
	}, nil
}

func (f *Fake) ResolveStream(ctx context.Context, sourceURL string) (youtube.StreamInfo, error) {
	f.record("ResolveStream")
	if f.ResolveStreamFn != nil {
		return f.ResolveStreamFn(ctx, sourceURL)
	}
	return youtube.StreamInfo{
		StreamURL: fmt.Sprintf("fresh:%s", sourceURL),
		ExpiresAt: farFuture,
	}, nil
}

func (f *Fake) FetchRelated(ctx context.Context, sourceURL string, limit int) ([]string, error) {
	f.record("FetchRelated")
	if f.RelatedFn != nil {
		return f.RelatedFn(ctx, sourceURL, limit)
	}
	return nil, nil
}

func (f *Fake) Search(ctx context.Context, query string, limit int) ([]youtube.SearchResult, error) {
	f.record("Search")
	if f.SearchFn != nil {
		return f.SearchFn(ctx, query, limit)
	}
	return nil, nil
}

func (f *Fake) FetchSubtitles(ctx context.Context, sourceURL, lang string) (youtube.Subtitles, error) {
	f.record("FetchSubtitles")
	if f.SubtitlesFn != nil {
		return f.SubtitlesFn(ctx, sourceURL, lang)
	}
	return youtube.Subtitles{Cues: []youtube.Cue{}}, nil
}

// farFuture is an expiry no test will reach, so a fake stream never looks
// stale by accident.
const farFuture = 9_999_999_999.0

// FarFuture is an expiry timestamp that never triggers a refresh.
const FarFuture = farFuture

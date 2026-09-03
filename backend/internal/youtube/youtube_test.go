package youtube

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":       "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ":                      "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":        "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ":         "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s": "dQw4w9WgXcQ",
		"not a url": "",
		"":          "",
	}
	for input, want := range cases {
		if got := VideoID(input); got != want {
			t.Errorf("VideoID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseStreamExpiry(t *testing.T) {
	t.Run("reads the expire parameter", func(t *testing.T) {
		got := ParseStreamExpiry("https://r1.googlevideo.com/videoplayback?expire=1700000000&x=y")
		if got != 1700000000 {
			t.Errorf("ParseStreamExpiry() = %v, want 1700000000", got)
		}
	})

	t.Run("falls back when absent", func(t *testing.T) {
		got := ParseStreamExpiry("https://example.com/stream.m4a")
		now := float64(time.Now().Unix())
		if got <= now {
			t.Errorf("ParseStreamExpiry() = %v, want a time in the future", got)
		}
	})
}

func TestParseJSON3ExtractsTimedCues(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"events": []map[string]any{
			// No segments: not a caption line.
			{"tStartMs": 0, "dDurationMs": 500},
			{"tStartMs": 1000, "dDurationMs": 2000, "segs": []map[string]string{{"utf8": "Hello "}, {"utf8": "world"}}},
			// Whitespace only: nothing to show.
			{"tStartMs": 3000, "dDurationMs": 2000, "segs": []map[string]string{{"utf8": "\n"}}},
			{"tStartMs": 3500, "dDurationMs": 1500, "segs": []map[string]string{{"utf8": "second line"}}},
		},
	})
	if err != nil {
		t.Fatalf("building fixture: %v", err)
	}

	cues := parseJSON3(string(raw))

	if got := cueTexts(cues); len(got) != 2 || got[0] != "Hello world" || got[1] != "second line" {
		t.Fatalf("cues = %v, want two lines", got)
	}
	if cues[0].Start != 1.0 || cues[0].Dur != 2.0 {
		t.Errorf("first cue = %+v, want start 1 and duration 2", cues[0])
	}
}

// Auto-generated tracks repeat the growing line as each word appears; the
// karaoke view would stutter if every repeat became its own cue.
func TestParseJSON3CollapsesImmediateRepeats(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"events": []map[string]any{
			{"tStartMs": 1000, "dDurationMs": 500, "segs": []map[string]string{{"utf8": "same"}}},
			{"tStartMs": 1010, "dDurationMs": 500, "segs": []map[string]string{{"utf8": "same"}}},
		},
	})
	if err != nil {
		t.Fatalf("building fixture: %v", err)
	}

	if got := parseJSON3(string(raw)); len(got) != 1 {
		t.Errorf("cues = %v, want the repeat collapsed", cueTexts(got))
	}
}

func TestParseVTTExtractsTimedCues(t *testing.T) {
	raw := "WEBVTT\n\n" +
		"00:00:01.000 --> 00:00:04.000\n" +
		"Hello <c>world</c>\n\n" +
		"00:00:04.000 --> 00:00:07.500\n" +
		"second line\n"

	cues := parseVTT(raw)

	if got := cueTexts(cues); len(got) != 2 || got[0] != "Hello world" || got[1] != "second line" {
		t.Fatalf("cues = %v, want two lines", got)
	}
	if cues[0].Start != 1.0 {
		t.Errorf("first cue start = %v, want 1", cues[0].Start)
	}
	if cues[1].Dur != 3.5 {
		t.Errorf("second cue duration = %v, want 3.5", cues[1].Dur)
	}
}

func TestParseVTTDecodesEntities(t *testing.T) {
	raw := "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nit&#39;s &quot;fine&quot; &amp; good\n"

	cues := parseVTT(raw)

	if len(cues) != 1 || cues[0].Text != `it's "fine" & good` {
		t.Errorf("cues = %v, want the entities decoded", cueTexts(cues))
	}
}

func TestPickLangPrefersRequestedThenEnglish(t *testing.T) {
	tracks := map[string][]subtitleEntry{
		"de":    {{Ext: "json3", URL: "u"}},
		"en-GB": {{Ext: "json3", URL: "u"}},
		"fr":    {{Ext: "json3", URL: "u"}},
	}

	if got := pickLang(tracks, "de"); got != "de" {
		t.Errorf("pickLang(de) = %q, want de", got)
	}
	// A request for "en" is satisfied by any English variant.
	if got := pickLang(tracks, "en"); got != "en-GB" {
		t.Errorf("pickLang(en) = %q, want en-GB", got)
	}
	// With nothing requested, English is preferred over the alternatives.
	if got := pickLang(tracks, ""); got != "en-GB" {
		t.Errorf("pickLang(\"\") = %q, want en-GB", got)
	}
	if got := pickLang(nil, "en"); got != "" {
		t.Errorf("pickLang(nil) = %q, want empty", got)
	}
}

func TestPickFormatPrefersJSON3(t *testing.T) {
	entry, ok := pickFormat([]subtitleEntry{
		{Ext: "vtt", URL: "vtt-url"},
		{Ext: "json3", URL: "json3-url"},
	})
	if !ok || entry.Ext != "json3" {
		t.Errorf("pickFormat() = %+v (%v), want the json3 entry", entry, ok)
	}

	if _, ok := pickFormat(nil); ok {
		t.Error("pickFormat(nil) reported a format")
	}
	if _, ok := pickFormat([]subtitleEntry{{Ext: "json3"}}); ok {
		t.Error("pickFormat() accepted an entry with no URL")
	}
}

func TestSubtitleCacheEvictsOldestEntries(t *testing.T) {
	cache := newSubtitleCache(2)

	cache.put("a", Subtitles{Lang: "a"})
	cache.put("b", Subtitles{Lang: "b"})
	cache.put("c", Subtitles{Lang: "c"})

	if _, ok := cache.get("a"); ok {
		t.Error("the oldest entry should have been evicted")
	}
	for _, key := range []string{"b", "c"} {
		if _, ok := cache.get(key); !ok {
			t.Errorf("entry %q should still be cached", key)
		}
	}
}

func TestSubtitleCacheOverwriteKeepsOneSlot(t *testing.T) {
	cache := newSubtitleCache(2)

	cache.put("a", Subtitles{Lang: "first"})
	cache.put("a", Subtitles{Lang: "second"})
	cache.put("b", Subtitles{Lang: "b"})

	got, ok := cache.get("a")
	if !ok || got.Lang != "second" {
		t.Errorf("cached a = %+v (%v), want the overwritten value still present", got, ok)
	}
}

func TestFetchSubtitlesRejectsAnUnrecognisedURL(t *testing.T) {
	client := New(Options{})

	subs, err := client.FetchSubtitles(t.Context(), "not a youtube url", "")

	if err == nil {
		t.Error("FetchSubtitles() error = nil, want one for an unrecognisable URL")
	}
	if len(subs.Cues) != 0 {
		t.Errorf("cues = %v, want none", subs.Cues)
	}
}

func TestWatchURL(t *testing.T) {
	if got := WatchURL("dQw4w9WgXcQ"); got != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Errorf("WatchURL() = %q", got)
	}
}

func cueTexts(cues []Cue) []string {
	out := make([]string, len(cues))
	for i, c := range cues {
		out[i] = c.Text
	}
	return out
}

// Extra arguments are how a deployment works around YouTube blocking it, so
// they have to reach yt-dlp on every call without displacing the caller's own
// arguments.
func TestArgvKeepsCallerArgumentsLast(t *testing.T) {
	y := New(Options{JSRuntime: "node", ExtraArgs: []string{"--cookies", "/data/cookies.txt"}})

	got := y.argv([]string{"--dump-json", "https://youtu.be/x"})
	want := []string{"--js-runtimes", "node", "--cookies", "/data/cookies.txt", "--dump-json", "https://youtu.be/x"}

	if !slices.Equal(got, want) {
		t.Errorf("argv() = %v, want %v", got, want)
	}
}

func TestYtdlpErrorKeepsTheFailureNotTheWarnings(t *testing.T) {
	stderr := "WARNING: [youtube] Unable to download webpage: HTTP Error 429\n" +
		"WARNING: [youtube] No supported JavaScript runtime could be found\n" +
		"ERROR: [youtube] abc: Sign in to confirm you are not a bot\n"

	got := ytdlpError(stderr)
	if !strings.Contains(got, "Sign in to confirm") {
		t.Errorf("ytdlpError() = %q, want the ERROR line", got)
	}
	if strings.Contains(got, "WARNING") {
		t.Errorf("ytdlpError() = %q, want the warnings dropped", got)
	}
}

// Without an ERROR line there is nothing to pick out, so the whole thing has to
// come through.
func TestYtdlpErrorFallsBackToTheWholeOutput(t *testing.T) {
	if got := ytdlpError("something unexpected"); got != "something unexpected" {
		t.Errorf("ytdlpError() = %q, want the raw output", got)
	}
}

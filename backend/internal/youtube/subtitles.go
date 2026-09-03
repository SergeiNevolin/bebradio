package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Preferred subtitle languages when the caller does not ask for a specific one.
var langPreference = []string{"en", "en-US", "en-GB", "en-orig"}

// Preferred caption formats, easiest to parse first.
var subFormats = []string{"json3", "vtt", "srv1"}

var (
	vttTimestampRe = regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2})[.,](\d{3})\s*-->\s*(\d{2}):(\d{2}):(\d{2})[.,](\d{3})`)
	vttTagRe       = regexp.MustCompile(`<[^>]+>`)
	blankLineRe    = regexp.MustCompile(`\r?\n\r?\n`)
)

// FetchSubtitles returns timed caption cues for a video, for the karaoke view.
//
// yt-dlp's --dump-json already lists every caption track (manual and
// auto-generated) with a downloadable URL, so no files are written. The
// timedtext URLs it hands out expire quickly, but the cues extracted from them
// do not -- so the parsed result is cached rather than the URL.
func (y *YTDLP) FetchSubtitles(ctx context.Context, sourceURL, lang string) (Subtitles, error) {
	vid := VideoID(sourceURL)
	if vid == "" {
		return Subtitles{Cues: []Cue{}}, ErrNotFound
	}

	key := vid + ":" + lang
	if cached, ok := y.subtitleTTL.get(key); ok {
		return cached, nil
	}

	subs, err := y.fetchSubtitles(ctx, vid, lang)
	if err != nil {
		return Subtitles{Cues: []Cue{}}, err
	}
	y.subtitleTTL.put(key, subs)
	return subs, nil
}

func (y *YTDLP) fetchSubtitles(ctx context.Context, vid, lang string) (Subtitles, error) {
	empty := Subtitles{Cues: []Cue{}}

	out, err := y.run(ctx, "--dump-json", "--no-download", "--no-playlist", WatchURL(vid))
	if err != nil {
		return empty, fmt.Errorf("youtube: fetching caption list: %w", err)
	}
	var data videoJSON
	if err := json.Unmarshal(bytes.TrimSpace(out), &data); err != nil {
		return empty, fmt.Errorf("youtube: parsing caption list: %w", err)
	}

	want := lang
	if want == "" {
		want = data.Language
	}

	// Human-written captions beat auto-generated ones when both exist.
	isAuto := false
	chosen := pickLang(data.Subtitles, want)
	entries := data.Subtitles[chosen]
	if len(entries) == 0 {
		isAuto = true
		chosen = pickLang(data.AutomaticCaptions, want)
		entries = data.AutomaticCaptions[chosen]
	}

	entry, ok := pickFormat(entries)
	if !ok {
		return empty, nil
	}

	raw, err := y.download(ctx, entry.URL)
	if err != nil {
		return empty, fmt.Errorf("youtube: downloading captions: %w", err)
	}

	var cues []Cue
	if entry.Ext == "json3" {
		cues = parseJSON3(raw)
	} else {
		cues = parseVTT(raw)
	}
	if len(cues) == 0 {
		return empty, nil
	}
	return Subtitles{Lang: chosen, Auto: isAuto, Cues: cues}, nil
}

// pickLang chooses a language key from a yt-dlp subtitles or captions map.
func pickLang(tracks map[string][]subtitleEntry, want string) string {
	if len(tracks) == 0 {
		return ""
	}
	if want != "" {
		if _, ok := tracks[want]; ok {
			return want
		}
		// "en-GB" should satisfy a request for "en" and vice versa.
		base := baseLang(want)
		for _, key := range sortedKeys(tracks) {
			if baseLang(key) == base {
				return key
			}
		}
	}
	for _, pref := range langPreference {
		if _, ok := tracks[pref]; ok {
			return pref
		}
	}
	for _, key := range sortedKeys(tracks) {
		if baseLang(key) == "en" {
			return key
		}
	}
	return sortedKeys(tracks)[0]
}

// sortedKeys makes language selection deterministic; Go map iteration order is
// random, and picking a different caption track on each request would make the
// karaoke view flicker between languages.
func sortedKeys(tracks map[string][]subtitleEntry) []string {
	keys := make([]string, 0, len(tracks))
	for k := range tracks {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func baseLang(code string) string {
	if i := strings.IndexByte(code, '-'); i >= 0 {
		return code[:i]
	}
	return code
}

func pickFormat(entries []subtitleEntry) (subtitleEntry, bool) {
	byExt := make(map[string]subtitleEntry, len(entries))
	for _, e := range entries {
		if e.URL != "" {
			byExt[e.Ext] = e
		}
	}
	for _, ext := range subFormats {
		if e, ok := byExt[ext]; ok {
			return e, true
		}
	}
	for _, e := range entries {
		if e.URL != "" {
			return e, true
		}
	}
	return subtitleEntry{}, false
}

func cleanText(text string) string {
	text = vttTagRe.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

// parseJSON3 reads YouTube's json3 caption format.
func parseJSON3(raw string) []Cue {
	var data struct {
		Events []struct {
			TStartMs    float64 `json:"tStartMs"`
			DDurationMs float64 `json:"dDurationMs"`
			Segs        []struct {
				UTF8 string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if json.Unmarshal([]byte(raw), &data) != nil {
		return nil
	}

	var cues []Cue
	for _, ev := range data.Events {
		if len(ev.Segs) == 0 {
			continue
		}
		var b strings.Builder
		for _, s := range ev.Segs {
			b.WriteString(s.UTF8)
		}
		text := cleanText(b.String())
		if text == "" {
			continue
		}
		start := ev.TStartMs / 1000
		// Auto-generated tracks repeat the previous line as each word appears;
		// collapse those so the karaoke view does not stutter.
		if n := len(cues); n > 0 && math.Abs(cues[n-1].Start-start) < 0.05 && cues[n-1].Text == text {
			continue
		}
		cues = append(cues, Cue{
			Start: round3(start),
			Dur:   round3(ev.DDurationMs / 1000),
			Text:  text,
		})
	}
	return cues
}

// parseVTT reads WebVTT and SRT-style caption files.
func parseVTT(raw string) []Cue {
	var cues []Cue
	for _, block := range blankLineRe.Split(raw, -1) {
		var lines []string
		for _, ln := range strings.Split(block, "\n") {
			if strings.TrimSpace(ln) != "" {
				lines = append(lines, ln)
			}
		}

		tsIndex := -1
		for i, ln := range lines {
			if strings.Contains(ln, "-->") {
				tsIndex = i
				break
			}
		}
		if tsIndex < 0 {
			continue
		}
		m := vttTimestampRe.FindStringSubmatch(lines[tsIndex])
		if m == nil {
			continue
		}
		start := vttSeconds(m[1], m[2], m[3], m[4])
		end := vttSeconds(m[5], m[6], m[7], m[8])

		text := cleanText(strings.Join(lines[tsIndex+1:], " "))
		if text == "" {
			continue
		}
		if n := len(cues); n > 0 && cues[n-1].Text == text {
			continue
		}
		cues = append(cues, Cue{
			Start: round3(start),
			Dur:   round3(math.Max(end-start, 0)),
			Text:  text,
		})
	}
	return cues
}

func vttSeconds(h, m, s, ms string) float64 {
	hi, _ := strconv.Atoi(h)
	mi, _ := strconv.Atoi(m)
	si, _ := strconv.Atoi(s)
	msi, _ := strconv.Atoi(ms)
	return float64(hi*3600+mi*60+si) + float64(msi)/1000
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// subtitleCache keeps parsed caption tracks, evicting the oldest entry once it
// is full. Captions never change, so there is no expiry.
type subtitleCache struct {
	mu    sync.Mutex
	max   int
	items map[string]Subtitles
	order []string
}

func newSubtitleCache(max int) *subtitleCache {
	return &subtitleCache{max: max, items: make(map[string]Subtitles, max)}
}

func (c *subtitleCache) get(key string) (Subtitles, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.items[key]
	return v, ok
}

func (c *subtitleCache) put(key string, value Subtitles) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.items[key]; !exists {
		if len(c.order) >= c.max && len(c.order) > 0 {
			delete(c.items, c.order[0])
			c.order = c.order[1:]
		}
		c.order = append(c.order, key)
	}
	c.items[key] = value
}

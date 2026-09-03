package youtube

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// audioFormat is the format selector every stream lookup uses: m4a where it
// exists, since it needs no remuxing, and whatever else is best where it does
// not.
const audioFormat = "bestaudio[ext=m4a]/bestaudio"

// ErrNotFound means yt-dlp ran successfully but the video yielded nothing
// usable -- no stream, no captions, no results.
var ErrNotFound = errors.New("youtube: no usable result")

// Options configure a YTDLP client.
type Options struct {
	// BinaryPath is the yt-dlp executable; the default resolves "yt-dlp" on
	// PATH.
	BinaryPath string
	// JSRuntime is passed to yt-dlp's --js-runtimes. YouTube's player requires
	// a JavaScript runtime to derive stream URLs.
	JSRuntime string
	// Timeout bounds a single yt-dlp invocation.
	Timeout time.Duration
	// ExtraArgs is prepended to every invocation's arguments, for options that
	// have to be changed on a running deployment: cookies, an extractor client
	// override, a proxy.
	ExtraArgs []string
	// Concurrency bounds how many yt-dlp processes may run at once. Each one
	// costs a process, a JS runtime and several network round-trips, so an
	// unbounded burst (a busy room refilling its queue) would otherwise be able
	// to exhaust the host.
	Concurrency int
	// SubtitleCacheMax is how many parsed caption tracks to keep.
	SubtitleCacheMax int
	// HTTPClient downloads caption files. Defaults to a client with a timeout.
	HTTPClient *http.Client
	// Logger receives lookup failures. Defaults to slog.Default().
	Logger *slog.Logger
}

// YTDLP is a Client backed by the yt-dlp command-line tool.
type YTDLP struct {
	binary      string
	jsRuntime   string
	extraArgs   []string
	timeout     time.Duration
	slots       chan struct{}
	http        *http.Client
	log         *slog.Logger
	subtitleTTL *subtitleCache
}

// New returns a yt-dlp backed client.
func New(opts Options) *YTDLP {
	if opts.BinaryPath == "" {
		opts.BinaryPath = "yt-dlp"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	if opts.Concurrency < 1 {
		opts.Concurrency = 4
	}
	if opts.SubtitleCacheMax < 1 {
		opts.SubtitleCacheMax = 256
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &YTDLP{
		binary:      opts.BinaryPath,
		jsRuntime:   opts.JSRuntime,
		extraArgs:   opts.ExtraArgs,
		timeout:     opts.Timeout,
		slots:       make(chan struct{}, opts.Concurrency),
		http:        opts.HTTPClient,
		log:         opts.Logger,
		subtitleTTL: newSubtitleCache(opts.SubtitleCacheMax),
	}
}

// videoJSON is the subset of yt-dlp's --dump-json output the service reads.
type videoJSON struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Uploader   string  `json:"uploader"`
	Channel    string  `json:"channel"`
	Thumbnail  string  `json:"thumbnail"`
	Duration   float64 `json:"duration"`
	WebpageURL string  `json:"webpage_url"`
	// URL is the selected format's stream, present only when the invocation
	// asked for a format.
	URL               string                     `json:"url"`
	Language          string                     `json:"language"`
	Subtitles         map[string][]subtitleEntry `json:"subtitles"`
	AutomaticCaptions map[string][]subtitleEntry `json:"automatic_captions"`
}

type subtitleEntry struct {
	Ext string `json:"ext"`
	URL string `json:"url"`
}

func (v videoJSON) artist() string {
	if v.Uploader != "" {
		return v.Uploader
	}
	if v.Channel != "" {
		return v.Channel
	}
	return "Unknown"
}

// FetchTrack looks up a video and resolves a playable audio stream for it.
func (y *YTDLP) FetchTrack(ctx context.Context, url string) (TrackInfo, error) {
	// The format is selected in the same invocation that dumps the metadata:
	// given -f, yt-dlp fills the top-level "url" with the chosen format's
	// stream. Asking separately would double the requests each added track makes
	// to YouTube, and being rate-limited is the failure mode that matters here.
	out, err := y.run(ctx, "-f", audioFormat, "--dump-json", "--no-download", "--no-playlist", url)
	if err != nil {
		return TrackInfo{}, fmt.Errorf("youtube: fetching video info: %w", err)
	}

	var data videoJSON
	if err := json.Unmarshal(bytes.TrimSpace(out), &data); err != nil {
		return TrackInfo{}, fmt.Errorf("youtube: parsing video info: %w", err)
	}

	stream := StreamInfo{StreamURL: data.URL, ExpiresAt: ParseStreamExpiry(data.URL)}
	if stream.StreamURL == "" {
		// No single format was selected, so nothing was filled in; ask for the
		// stream on its own.
		if stream, err = y.ResolveStream(ctx, url); err != nil {
			return TrackInfo{}, err
		}
	}

	source := data.WebpageURL
	if source == "" {
		source = url
	}
	title := data.Title
	if title == "" {
		title = "Unknown"
	}
	return TrackInfo{
		Title:     title,
		Artist:    data.artist(),
		Thumbnail: data.Thumbnail,
		Duration:  int(data.Duration),
		StreamURL: stream.StreamURL,
		SourceURL: source,
		ExpiresAt: stream.ExpiresAt,
	}, nil
}

// ResolveStream re-resolves just the playable URL of a known video.
func (y *YTDLP) ResolveStream(ctx context.Context, sourceURL string) (StreamInfo, error) {
	out, err := y.run(ctx, "-f", audioFormat, "-g", "--no-playlist", sourceURL)
	if err != nil {
		return StreamInfo{}, fmt.Errorf("youtube: resolving stream: %w", err)
	}
	// -g prints one URL per selected format; the first line is the audio track.
	stream := strings.TrimSpace(firstLine(string(out)))
	if stream == "" {
		return StreamInfo{}, ErrNotFound
	}
	return StreamInfo{StreamURL: stream, ExpiresAt: ParseStreamExpiry(stream)}, nil
}

// FetchRelated returns watch URLs from the Mix (radio) playlist of a video.
func (y *YTDLP) FetchRelated(ctx context.Context, sourceURL string, limit int) ([]string, error) {
	vid := VideoID(sourceURL)
	if vid == "" {
		return nil, ErrNotFound
	}
	if limit < 1 {
		return nil, nil
	}
	mixURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s&list=RD%s", vid, vid)

	out, err := y.run(ctx, mixURL,
		"--flat-playlist", "--dump-json", "--no-warnings",
		"-I", fmt.Sprintf("1:%d", limit))
	if err != nil {
		return nil, fmt.Errorf("youtube: fetching mix playlist: %w", err)
	}

	var urls []string
	forEachJSONLine(out, func(line []byte) {
		var entry struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(line, &entry) != nil || entry.ID == "" || entry.ID == vid {
			return
		}
		urls = append(urls, WatchURL(entry.ID))
	})
	return urls, nil
}

// Search returns up to limit catalogue hits for a query.
func (y *YTDLP) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit < 1 {
		return nil, nil
	}
	out, err := y.run(ctx, fmt.Sprintf("ytsearch%d:%s", limit, query),
		"--dump-json", "--no-download", "--flat-playlist")
	if err != nil {
		return nil, fmt.Errorf("youtube: searching: %w", err)
	}

	results := make([]SearchResult, 0, limit)
	forEachJSONLine(out, func(line []byte) {
		var data videoJSON
		if json.Unmarshal(line, &data) != nil {
			return
		}
		thumb := data.Thumbnail
		if thumb == "" && data.ID != "" {
			thumb = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", data.ID)
		}
		title := data.Title
		if title == "" {
			title = "Unknown"
		}
		results = append(results, SearchResult{
			ID:        data.ID,
			Title:     title,
			Artist:    data.artist(),
			Thumbnail: thumb,
			Duration:  int(data.Duration),
			URL:       WatchURL(data.ID),
		})
	})
	return results, nil
}

// run executes yt-dlp with the given arguments and returns its stdout.
//
// It waits for a concurrency slot first, so a burst of lookups queues rather
// than forking a process per request.
func (y *YTDLP) run(ctx context.Context, args ...string) ([]byte, error) {
	select {
	case y.slots <- struct{}{}:
		defer func() { <-y.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	ctx, cancel := context.WithTimeout(ctx, y.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, y.binary, y.argv(args)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("yt-dlp timed out after %s: %w", y.timeout, ctxErr)
		}
		return nil, fmt.Errorf("yt-dlp failed: %w: %s", err, ytdlpError(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// argv builds the full argument list for one invocation. The caller's arguments
// come last, so nothing in ExtraArgs can displace them.
func (y *YTDLP) argv(args []string) []string {
	full := make([]string, 0, len(args)+len(y.extraArgs)+2)
	if y.jsRuntime != "" {
		full = append(full, "--js-runtimes", y.jsRuntime)
	}
	full = append(full, y.extraArgs...)
	return append(full, args...)
}

// download fetches a caption file. YouTube serves these from timedtext URLs
// that expire quickly, which is why the parsed result is what gets cached.
func (y *YTDLP) download(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := y.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("youtube: caption download returned %s", resp.Status)
	}
	// Caption tracks are small; the cap stops a misbehaving response from
	// filling memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ytdlpError picks the failure out of yt-dlp's stderr. yt-dlp warns freely
// before giving up -- about rate limits, missing runtimes, deprecated formats --
// and says what actually went wrong last, so truncating from the front reliably
// throws away the only line worth reading.
func ytdlpError(stderr string) string {
	var failures []string
	for _, line := range strings.Split(stderr, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "ERROR:") {
			failures = append(failures, line)
		}
	}
	if len(failures) == 0 {
		return truncate(stderr, 1000)
	}
	return truncate(strings.Join(failures, "; "), 1000)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// forEachJSONLine walks yt-dlp's newline-delimited JSON output, skipping blanks.
func forEachJSONLine(out []byte, fn func([]byte)) {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	// Playlist entries carry long description fields; the default 64 KiB line
	// limit is not always enough.
	scanner.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		fn(line)
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

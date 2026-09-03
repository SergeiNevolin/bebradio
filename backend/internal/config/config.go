// Package config loads the service configuration from the environment.
//
// Every knob has a production-safe default, so the zero-configuration case
// still boots; anything that would be unsafe in production (a fallback signing
// key, a wide-open CORS policy) is reported through Config.Warnings so the
// operator sees it in the logs at startup.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultSecretKey is the signing key used when SECRET_KEY is unset. It exists
// only so local development works out of the box; using it in production is
// reported as a warning.
const DefaultSecretKey = "bebradio-secret-key-change-in-production"

// Config is the fully-resolved runtime configuration.
type Config struct {
	// --- Server ---
	Addr            string
	CORSOrigins     []string
	ShutdownTimeout time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	MaxRequestBytes int64
	LogLevel        slog.Level
	LogFormat       string

	// --- Database ---
	DatabaseURL      string
	DBMaxConns       int32
	DBMinConns       int32
	DBConnectTimeout time.Duration

	// --- Auth ---
	SecretKey  string
	JWTExpiry  time.Duration
	BcryptCost int

	// --- Chat ---
	MaxChatMessages int
	MaxChatTextLen  int

	// StreamRefreshMargin is how close to its stated expiry a resolved
	// googlevideo URL may get before it is re-resolved. yt-dlp hands back URLs
	// that stop working after a few hours, so a track that has been sitting in
	// the queue needs a fresh one before anybody presses play.
	StreamRefreshMargin time.Duration

	// --- Server-side auto-advance ---
	// Track advancement is normally driven by whichever client reaches the end
	// of the audio first. When every client has dropped or stalled, the
	// background loop takes over: it advances a playing room once its position
	// has run past the current track's duration by AutoAdvanceGrace.
	AutoAdvanceInterval time.Duration
	AutoAdvanceGrace    time.Duration
	// AdvanceDedupWindow makes a second advance within the window a no-op, so a
	// client "ended" event and the server loop (or several clients at once)
	// cannot skip two tracks.
	AdvanceDedupWindow time.Duration

	// ReactionEmojis is the allowlist of relayable emoji. Reactions are
	// ephemeral (never stored), and the allowlist stops a client broadcasting
	// arbitrary strings to the room.
	ReactionEmojis []string

	// --- Auto-radio ---
	// When a room has auto-radio on and its queue drops to RadioRefillAt or
	// fewer tracks, pull RadioBatch related tracks from the last track's
	// YouTube Mix.
	RadioRefillAt int
	RadioBatch    int

	// --- yt-dlp ---
	YTDLPPath        string
	YTDLPJSRuntime   string
	YTDLPConcurrency int
	YTDLPTimeout     time.Duration
	SubtitleCacheMax int
	// YTDLPExtraArgs is passed to every yt-dlp invocation. YouTube blocks
	// datacentre addresses harder than home ones, and the remedy changes as
	// often as their player does, so it has to be settable without a rebuild:
	// typically "--cookies /data/cookies.txt" or
	// "--extractor-args youtube:player_client=tv_simply". Split on whitespace,
	// so an argument cannot itself contain a space.
	YTDLPExtraArgs []string

	// Warnings holds non-fatal configuration problems worth logging at startup.
	Warnings []string
}

// DefaultReactionEmojis is the built-in reaction allowlist.
var DefaultReactionEmojis = []string{"❤️", "\U0001F525", "\U0001F602", "\U0001F44D", "\U0001F389", "\U0001F62E", "\U0001F64C", "\U0001F483"}

// Load reads the configuration from the process environment. It returns an
// error only for values that are present but unusable; missing values fall back
// to the defaults above.
func Load() (Config, error) {
	var errs []string
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	cfg := Config{
		Addr:             env("ADDR", ":8000"),
		ShutdownTimeout:  duration("SHUTDOWN_TIMEOUT", 20*time.Second, fail),
		ReadTimeout:      duration("HTTP_READ_TIMEOUT", 30*time.Second, fail),
		WriteTimeout:     duration("HTTP_WRITE_TIMEOUT", 60*time.Second, fail),
		IdleTimeout:      duration("HTTP_IDLE_TIMEOUT", 120*time.Second, fail),
		MaxRequestBytes:  int64(integer("MAX_REQUEST_BYTES", 1<<20, fail)),
		LogFormat:        env("LOG_FORMAT", "json"),
		DatabaseURL:      NormalizeDatabaseURL(env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/bebradio")),
		DBMaxConns:       int32(integer("DB_MAX_CONNS", 20, fail)),
		DBMinConns:       int32(integer("DB_MIN_CONNS", 2, fail)),
		DBConnectTimeout: duration("DB_CONNECT_TIMEOUT", 15*time.Second, fail),
		SecretKey:        env("SECRET_KEY", DefaultSecretKey),
		JWTExpiry:        duration("JWT_EXPIRE", 72*time.Hour, fail),
		BcryptCost:       integer("BCRYPT_COST", 12, fail),

		MaxChatMessages: integer("MAX_CHAT_MESSAGES", 100, fail),
		MaxChatTextLen:  integer("MAX_CHAT_TEXT_LEN", 2000, fail),

		StreamRefreshMargin: duration("STREAM_REFRESH_MARGIN", 600*time.Second, fail),

		AutoAdvanceInterval: duration("AUTO_ADVANCE_INTERVAL", 2000*time.Millisecond, fail),
		AutoAdvanceGrace:    duration("AUTO_ADVANCE_GRACE", 2500*time.Millisecond, fail),
		AdvanceDedupWindow:  duration("ADVANCE_DEDUP_WINDOW", time.Second, fail),

		ReactionEmojis: DefaultReactionEmojis,

		RadioRefillAt: integer("RADIO_REFILL_AT", 1, fail),
		RadioBatch:    integer("RADIO_BATCH", 3, fail),

		YTDLPPath:        env("YTDLP_PATH", "yt-dlp"),
		YTDLPJSRuntime:   env("YTDLP_JS_RUNTIME", "node"),
		YTDLPConcurrency: integer("YTDLP_CONCURRENCY", 4, fail),
		YTDLPTimeout:     duration("YTDLP_TIMEOUT", 60*time.Second, fail),
		SubtitleCacheMax: integer("SUBTITLE_CACHE_MAX", 256, fail),
		YTDLPExtraArgs:   strings.Fields(os.Getenv("YTDLP_EXTRA_ARGS")),
	}

	cfg.CORSOrigins = splitCSV(env("CORS_ORIGINS", "http://localhost:3000"))
	cfg.LogLevel = logLevel(env("LOG_LEVEL", "info"))

	if raw := os.Getenv("REACTION_EMOJIS"); raw != "" {
		cfg.ReactionEmojis = splitCSV(raw)
	}

	if cfg.SecretKey == DefaultSecretKey {
		cfg.Warnings = append(cfg.Warnings,
			"SECRET_KEY is unset; using the built-in development key. Set SECRET_KEY before deploying.")
	}
	for _, origin := range cfg.CORSOrigins {
		if origin == "*" {
			cfg.Warnings = append(cfg.Warnings,
				"CORS_ORIGINS contains a wildcard; credentialed cross-origin requests will be rejected by browsers.")
		}
	}
	if cfg.BcryptCost < 4 || cfg.BcryptCost > 31 {
		fail("BCRYPT_COST must be between 4 and 31, got %d", cfg.BcryptCost)
	}
	if cfg.YTDLPConcurrency < 1 {
		fail("YTDLP_CONCURRENCY must be at least 1, got %d", cfg.YTDLPConcurrency)
	}
	if cfg.RadioBatch < 1 {
		fail("RADIO_BATCH must be at least 1, got %d", cfg.RadioBatch)
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(errs, "; "))
	}
	return cfg, nil
}

// NormalizeDatabaseURL rewrites SQLAlchemy-style DSNs (the shape the previous
// Python service was configured with) into something pgx understands, so an
// existing deployment's environment keeps working untouched.
func NormalizeDatabaseURL(url string) string {
	for _, prefix := range []string{"postgresql+asyncpg://", "postgresql+psycopg://", "postgresql+psycopg2://"} {
		if strings.HasPrefix(url, prefix) {
			return "postgres://" + strings.TrimPrefix(url, prefix)
		}
	}
	return url
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func integer(key string, fallback int, fail func(string, ...any)) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		fail("%s must be an integer, got %q", key, raw)
		return fallback
	}
	return v
}

// duration accepts either a Go duration string ("2s", "600ms") or a bare number
// of seconds, which is how the previous configuration expressed these values.
func duration(key string, fallback time.Duration, fail func(string, ...any)) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if secs, err := strconv.ParseFloat(raw, 64); err == nil {
		return time.Duration(secs * float64(time.Second))
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		fail("%s must be a duration or a number of seconds, got %q", key, raw)
		return fallback
	}
	return d
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func logLevel(raw string) slog.Level {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

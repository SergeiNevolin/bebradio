import os

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql+asyncpg://postgres:postgres@localhost:5432/bebradio",
)

SECRET_KEY = os.getenv("SECRET_KEY", "bebradio-secret-key-change-in-production")
JWT_ALGORITHM = "HS256"
JWT_EXPIRE_HOURS = 72

MAX_CHAT_MESSAGES = 100

# --- Local media storage ---
# Directory where downloaded audio files are kept.
MEDIA_DIR = os.getenv("MEDIA_DIR", os.path.join(os.path.dirname(__file__), "media", "tracks"))
# Time-to-live for media files not referenced by any room queue (seconds).
MEDIA_TTL = int(os.getenv("MEDIA_TTL", str(4 * 3600)))  # 4 hours
# Maximum concurrent yt-dlp downloads.
MAX_DOWNLOADS = int(os.getenv("MAX_DOWNLOADS", "3"))
# Maximum total on-disk size for media files (bytes). Oldest unreferenced
# files are evicted when this limit is exceeded.  Default 10 GB.
MEDIA_MAX_SIZE = int(os.getenv("MEDIA_MAX_SIZE", str(10 * 1024 * 1024 * 1024)))
# Maximum track duration in seconds.  Videos longer than this are rejected
# when a user tries to add them to a queue.  Default 3600 s (1 hour).
MAX_DURATION = int(os.getenv("MAX_DURATION", "3600"))

# --- Server-side auto-advance ---
# Track advancement is normally driven by whichever client reaches the end of
# the audio first. When every client has dropped or stalled, this background
# loop takes over: it advances a playing room once its position has run past
# the current track's duration by ``AUTO_ADVANCE_GRACE`` seconds.
AUTO_ADVANCE_INTERVAL = 2.0
AUTO_ADVANCE_GRACE = 2.5
# ``go_next`` ignores a second call within this window so a client "ended"
# event and the server loop (or several clients at once) cannot skip twice.
ADVANCE_DEDUP_WINDOW = 1.0

# --- Emoji reactions ---
# Reactions are ephemeral (never stored); the server only relays emoji from
# this allowlist so a client cannot broadcast arbitrary strings.
REACTION_EMOJIS = ["❤️", "🔥", "😂", "👍", "🎉", "😮", "🙌", "💃"]

# --- Auto-radio ---
# When ``room.auto_radio`` is on and the queue drops to this length or below,
# pull this many related tracks from the last track's YouTube Mix.
RADIO_REFILL_AT = 1
RADIO_BATCH = 3

# --- PO Token provider ---
# bgutil-ytdlp-pot-provider HTTP server URL.
BGUTIL_BASE_URL = os.getenv("BGUTIL_BASE_URL", "http://127.0.0.1:4416")

# --- Rate limiting ---
# Per-IP limits for expensive yt-dlp endpoints.
RATE_LIMIT_SEARCH = int(os.getenv("RATE_LIMIT_SEARCH", "15"))   # requests per window
RATE_LIMIT_QUEUE = int(os.getenv("RATE_LIMIT_QUEUE", "10"))    # requests per window
RATE_LIMIT_WINDOW = int(os.getenv("RATE_LIMIT_WINDOW", "60"))  # window in seconds

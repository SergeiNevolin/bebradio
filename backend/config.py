import os

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql+asyncpg://postgres:postgres@localhost:5432/bebradio",
)

SECRET_KEY = os.getenv("SECRET_KEY", "bebradio-secret-key-change-in-production")
JWT_ALGORITHM = "HS256"
JWT_EXPIRE_HOURS = 72

MAX_CHAT_MESSAGES = 100

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

MEDIA_SERVICE_URL = os.getenv("MEDIA_SERVICE_URL", "http://127.0.0.1:8100")

# --- Rate limiting ---
# Per-IP limits for expensive media-service endpoints.
RATE_LIMIT_SEARCH = int(os.getenv("RATE_LIMIT_SEARCH", "15"))   # requests per window
RATE_LIMIT_QUEUE = int(os.getenv("RATE_LIMIT_QUEUE", "10"))    # requests per window
RATE_LIMIT_WINDOW = int(os.getenv("RATE_LIMIT_WINDOW", "60"))  # window in seconds

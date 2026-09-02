import os

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql+asyncpg://postgres:postgres@localhost:5432/bebradio",
)

SECRET_KEY = os.getenv("SECRET_KEY", "bebradio-secret-key-change-in-production")
JWT_ALGORITHM = "HS256"
JWT_EXPIRE_HOURS = 72

MAX_CHAT_MESSAGES = 100

# --- Stream URL refresh ---
# ``yt-dlp -g`` hands back a googlevideo URL that stops working after a few
# hours. Re-resolve it once it is within this many seconds of its stated
# expiry so a track that has been sitting in the queue still plays.
STREAM_REFRESH_MARGIN = 600

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

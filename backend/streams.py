import asyncio
import time
from typing import Optional

from config import STREAM_REFRESH_MARGIN
from models import Room, Track
from youtube import resolve_stream


async def ensure_fresh(room: Room, track: Optional[Track]) -> bool:
    """Re-resolve ``track``'s stream URL if it is at or near expiry.

    Returns ``True`` when the track's ``url`` was replaced, so the caller
    knows to persist the queue. A track with no known ``source_url`` (e.g. one
    added before this feature existed) is left untouched.
    """
    if track is None or not track.source_url:
        return False
    if track.stream_expires_at - time.time() > STREAM_REFRESH_MARGIN:
        return False

    data = await asyncio.to_thread(resolve_stream, track.source_url)
    if not data:
        return False

    track.url = data["stream_url"]
    track.stream_expires_at = data["expires_at"]
    return True


async def ensure_fresh_ahead(room: Room) -> bool:
    """Keep the current *and* next track's stream URLs live.

    Re-resolving the next track before playback reaches it means the switch
    at ``go_next`` has no wait on the network. Returns ``True`` if any URL
    was replaced, so the caller knows to persist the queue.
    """
    changed = False
    current = room.current_track()
    if await ensure_fresh(room, current):
        changed = True

    nxt = room.current_index + 1
    if 0 <= nxt < len(room.queue):
        if await ensure_fresh(room, room.queue[nxt]):
            changed = True

    return changed

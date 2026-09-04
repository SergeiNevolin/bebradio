"""Ensure tracks have local audio files available for streaming.

Replaces the old stream-URL refresh logic: instead of keeping googlevideo
URLs alive, we download tracks to the local media directory before playback.
"""

import asyncio
import logging
from typing import Optional

import media
from models import Room, Track

log = logging.getLogger(__name__)


async def ensure_local(room: Room, track: Optional[Track]) -> bool:
    """Download ``track``'s audio to disk if not already present.

    Returns ``True`` when a file was downloaded (so the caller knows to
    persist the queue). A track with no ``source_url`` is left untouched.
    """
    if track is None or not track.source_url:
        return False
    if track.local_path and media.is_downloaded(track.id):
        # Ensure url is set even if track was loaded from old DB row
        if not track.url:
            track.url = f"/api/media/{track.id}"
            return True
        return False

    success = await asyncio.to_thread(media.download_track, track.source_url, track.id)
    if not success:
        return False

    filename = media.get_local_filename(track.id)
    if filename:
        track.local_path = filename
        track.url = f"/api/media/{track.id}"
        return True
    return False


async def ensure_local_ahead(room: Room) -> bool:
    """Ensure the current *and* next track's audio files exist on disk.

    Downloading the next track before playback reaches it means the switch
    at ``go_next`` has no wait on the network. Returns ``True`` if any file
    was downloaded, so the caller knows to persist the queue.
    """
    changed = False
    current = room.current_track()
    if await ensure_local(room, current):
        changed = True

    nxt = room.current_index + 1
    if 0 <= nxt < len(room.queue):
        if await ensure_local(room, room.queue[nxt]):
            changed = True

    return changed

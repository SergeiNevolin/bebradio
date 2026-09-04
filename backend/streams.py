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


def _media_key(track: Track) -> str:
    """Return the key used for on-disk file naming.

    Falls back to deriving the video_id from source_url for tracks loaded
    from old DB rows that predate the video_id column.
    """
    if track.video_id:
        return track.video_id
    from youtube import video_id as extract_vid
    vid = extract_vid(track.source_url)
    if vid:
        track.video_id = vid
        return vid
    # Last resort — shouldn't happen for real YouTube tracks
    return track.id


async def ensure_local(room: Room, track: Optional[Track]) -> bool:
    """Download ``track``'s audio to disk if not already present.

    Returns ``True`` when a file was downloaded (so the caller knows to
    persist the queue). A track with no ``source_url`` is left untouched.
    """
    if track is None or not track.source_url:
        return False
    key = _media_key(track)
    if track.local_path and media.is_downloaded(key):
        if not track.url:
            track.url = f"/api/media/{key}"
            return True
        return False

    success = await asyncio.to_thread(media.download_track, track.source_url, key)
    if not success:
        return False

    filename = media.get_local_filename(key)
    if filename:
        track.local_path = filename
        track.url = f"/api/media/{key}"
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

"""Select room tracks that need media preparation."""

import logging
from typing import Optional

from media_client import ensure_media
from models import Room, Track

log = logging.getLogger(__name__)


async def ensure_track_ready(track: Optional[Track]) -> bool:
    if track is None:
        return False
    if not track.media_id:
        log.warning("track %s has no media-service media_id", track.id)
        return False
    ready_ids = await ensure_media([{
        "media_id": track.media_id,
        "source_url": track.source_url,
    }])
    if track.media_id not in ready_ids:
        return False

    changed = not track.url
    track.local_path = track.local_path or f"{track.media_id}.m4a"
    track.url = f"/api/media/{track.media_id}"
    return changed


async def ensure_room_media(room: Room) -> bool:
    tracks: list[Track] = []
    current = room.current_track()
    if current is not None:
        tracks.append(current)

    next_index = room.current_index + 1
    if 0 <= next_index < len(room.queue):
        tracks.append(room.queue[next_index])

    pending = [track for track in tracks if track.media_id and not track.url]
    if not pending:
        return False

    ready_ids = await ensure_media([
        {"media_id": track.media_id, "source_url": track.source_url}
        for track in pending
    ])
    changed = False
    for track in pending:
        if track.media_id not in ready_ids:
            continue
        changed = changed or not track.url
        track.local_path = track.local_path or f"{track.media_id}.m4a"
        track.url = f"/api/media/{track.media_id}"
    return changed

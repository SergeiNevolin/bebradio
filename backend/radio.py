import asyncio
import time

from config import RADIO_BATCH, RADIO_REFILL_AT
from connections import manager
from models import Room, Track
from youtube import fetch_related, fetch_track, video_id


def _seed_url(room: Room) -> str:
    if room.radio_seed_url:
        return room.radio_seed_url
    if room.queue:
        return room.queue[-1].source_url
    return ""


def needs_refill(room: Room) -> bool:
    return (
        room.auto_radio
        and not room.radio_filling
        and len(room.queue) <= RADIO_REFILL_AT
        and bool(_seed_url(room))
    )


async def refill(room: Room) -> bool:
    """Append related tracks from the seed track's YouTube Mix.

    Returns ``True`` when at least one track was added. Guarded by
    ``room.radio_filling`` so concurrent callers don't stack refills.
    """
    if not needs_refill(room):
        return False

    seed = _seed_url(room)
    room.radio_filling = True
    try:
        room.radio_seen.add(video_id(seed))
        candidates = await asyncio.to_thread(fetch_related, seed, RADIO_BATCH * 4)

        added = 0
        for url in candidates:
            if added >= RADIO_BATCH:
                break
            vid = video_id(url)
            if not vid or vid in room.radio_seen:
                continue
            info = await asyncio.to_thread(fetch_track, url)
            if not info:
                continue
            room.radio_seen.add(vid)
            room.queue.append(Track(
                title=info["title"],
                artist=info["artist"],
                url=info["stream_url"],
                thumbnail=info["thumbnail"],
                duration=info["duration"],
                added_by="📻 Radio",
                source_url=info["source_url"],
                stream_expires_at=info["expires_at"],
            ))
            added += 1

        if added and not room.is_playing and room.queue:
            room.is_playing = True
            room.position = 0.0
            room.last_sync_at = time.time()
        return added > 0
    finally:
        room.radio_filling = False


async def refill_and_broadcast(room: Room) -> None:
    """Fire-and-forget refill that persists and pushes the new queue."""
    from store import save_tracks  # local import avoids an import cycle

    if await refill(room):
        await save_tracks(room)
        await manager.broadcast(room.id, room.to_dict())

"""Auto-radio: keep a room's queue topped up with related tracks.

When ``room.auto_radio`` is on and the queue nears empty, we pull the "Mix"
(radio) playlist of the last-played track from YouTube and append a handful of
fresh tracks tagged as added by 📻 Radio.
"""

import asyncio
import time
from typing import Awaitable, Callable, Optional

from config import RADIO_BATCH, RADIO_REFILL_AT
from connections import manager
from models import Room, Track
from youtube import fetch_related, fetch_track, video_id

RADIO_TAG = "📻 Radio"


def _seed_url(room: Room) -> str:
    """The YouTube URL whose Mix we grow the queue from."""
    if room.radio_seed_url:
        return room.radio_seed_url
    if room.queue:
        return room.queue[-1].source_url
    return ""


def needs_refill(room: Room) -> bool:
    """Whether the room is due for an auto-radio top-up right now."""
    return (
        room.auto_radio
        and not room.radio_filling
        and len(room.queue) <= RADIO_REFILL_AT
        and bool(_seed_url(room))
    )


async def _collect_tracks(room: Room, seed: str, limit: int) -> list[Track]:
    """Resolve up to ``limit`` unseen tracks from ``seed``'s YouTube Mix.

    Records every video id it considers in ``room.radio_seen`` so a later
    refill does not offer the same track again.
    """
    room.radio_seen.add(video_id(seed))
    candidates = await asyncio.to_thread(fetch_related, seed, limit * 4)

    picked: list[Track] = []
    for url in candidates:
        if len(picked) >= limit:
            break
        vid = video_id(url)
        if not vid or vid in room.radio_seen:
            continue
        info = await asyncio.to_thread(fetch_track, url)
        if not info:
            continue
        room.radio_seen.add(vid)
        picked.append(Track.from_youtube(info, added_by=RADIO_TAG))
    return picked


async def refill(
    room: Room,
    on_start: Optional[Callable[[], Awaitable[None]]] = None,
) -> bool:
    """Append related tracks to the queue. Returns ``True`` if any were added.

    Guarded by ``room.radio_filling`` so overlapping callers don't stack
    refills on top of each other. ``on_start`` is awaited once the search is
    under way (``room.radio_filling`` set), so callers can surface a
    "searching" indicator before the network work begins.
    """
    if not needs_refill(room):
        return False

    seed = _seed_url(room)
    room.radio_filling = True
    try:
        if on_start is not None:
            await on_start()
        new_tracks = await _collect_tracks(room, seed, RADIO_BATCH)
        if not new_tracks:
            return False

        room.queue.extend(new_tracks)
        if not room.is_playing:
            room.is_playing = True
            room.position = 0.0
            room.last_sync_at = time.time()
        return True
    finally:
        room.radio_filling = False


async def refill_and_broadcast(room: Room) -> None:
    """Fire-and-forget refill that shows a "searching" state, then the result."""
    from store import save_tracks  # local import avoids an import cycle

    announced = False

    async def announce_searching() -> None:
        nonlocal announced
        announced = True
        await manager.broadcast(room.id, room.to_dict())

    added = await refill(room, on_start=announce_searching)
    if added:
        await save_tracks(room)
    if added or announced:
        # Push the final state so the "searching" indicator clears, even when
        # the search turned up nothing.
        await manager.broadcast(room.id, room.to_dict())


def maybe_refill(room: Room) -> None:
    """Kick off a background refill if the room needs one.

    Cheap and safe to call after any queue/playback change; does nothing
    unless :func:`needs_refill` says so.
    """
    if needs_refill(room):
        asyncio.create_task(refill_and_broadcast(room))

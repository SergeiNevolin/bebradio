import asyncio

from config import AUTO_ADVANCE_GRACE, AUTO_ADVANCE_INTERVAL
from connections import manager
from playback import go_next
from radio import maybe_refill
from store import rooms, save_tracks
from media_prefetch import ensure_room_media


async def run_auto_advance() -> None:
    """Background loop: keep playing rooms moving when clients go quiet."""
    while True:
        await asyncio.sleep(AUTO_ADVANCE_INTERVAL)
        try:
            await _tick()
        except asyncio.CancelledError:
            raise
        except Exception:
            # A single bad room must not kill the loop.
            pass


async def _tick() -> None:
    for room in list(rooms.values()):
        # Nobody listening -> nothing to keep in sync.
        if manager.get_count(room.id) == 0:
            continue

        advanced = False
        track = room.current_track()
        if room.is_playing and track and track.duration:
            if room.get_current_position() >= track.duration + AUTO_ADVANCE_GRACE:
                advanced = go_next(room)

        # Network-bound; runs detached so one slow refill can't stall
        # auto-advance for every other room.
        maybe_refill(room)

        # Ensure the current and next track's audio files are on disk so the
        # hand-off (and any client-side prefetch/crossfade) never waits.
        refreshed = await ensure_room_media(room)

        if advanced or refreshed:
            await save_tracks(room)
            await manager.broadcast(room.id, room.to_dict())

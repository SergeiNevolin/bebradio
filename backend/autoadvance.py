import asyncio

from config import AUTO_ADVANCE_GRACE, AUTO_ADVANCE_INTERVAL
from connections import manager
from playback import go_next
from radio import maybe_refill
from store import rooms, save_tracks
from streams import ensure_fresh


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

        changed = False
        track = room.current_track()
        if room.is_playing and track and track.duration:
            if room.get_current_position() >= track.duration + AUTO_ADVANCE_GRACE:
                changed = go_next(room)

        # Network-bound; runs detached so one slow refill can't stall
        # auto-advance for every other room.
        maybe_refill(room)

        if changed:
            # New track is about to start for everyone; make sure its URL is live.
            await ensure_fresh(room, room.current_track())
            await save_tracks(room)
            await manager.broadcast(room.id, room.to_dict())

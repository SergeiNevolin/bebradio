import time

from config import ADVANCE_DEDUP_WINDOW
from models import Room


def go_next(room: Room) -> bool:
    """Advance to next track, removing the finished one. Returns True if position changed."""
    if not room.queue:
        return False

    # A client "ended" event and the server-side auto-advance loop (or several
    # clients at once) can all fire within a few milliseconds. Ignore repeats
    # inside a short window so the queue only moves forward by one.
    now = time.time()
    if now - room.last_advance_at < ADVANCE_DEDUP_WINDOW:
        return False
    room.last_advance_at = now

    # current_index can drift out of range if the queue shrank elsewhere;
    # clamp before popping so we never raise IndexError.
    idx = max(0, min(room.current_index, len(room.queue) - 1))
    finished = room.queue[idx]
    if finished.source_url:
        room.radio_seed_url = finished.source_url
    room.queue.pop(idx)
    room.current_index = max(0, min(idx, len(room.queue) - 1))
    room.position = 0

    if not room.queue:
        room.is_playing = False
        return True

    room.is_playing = True
    room.last_sync_at = time.time()
    return True


def go_prev(room: Room) -> bool:
    """Go to previous track. Returns True if position changed."""
    if not room.queue or room.current_index <= 0:
        return False
    room.current_index -= 1
    room.position = 0
    room.is_playing = True
    room.last_sync_at = time.time()
    return True


def jump_to(room: Room, index) -> bool:
    """Jump to a specific track by index. Returns True if successful."""
    if not isinstance(index, (int, float)) or index != int(index):
        return False
    index = int(index)
    if 0 <= index < len(room.queue):
        room.current_index = index
        room.position = 0
        room.is_playing = True
        room.last_sync_at = time.time()
        return True
    return False


def seek_to(room: Room, position: float) -> None:
    """Seek to a specific position in the current track."""
    room.position = max(0.0, position)
    room.last_sync_at = time.time()

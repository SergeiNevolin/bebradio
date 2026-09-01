import time

from models import Room


def go_next(room: Room) -> bool:
    """Advance to next track, removing the finished one. Returns True if position changed."""
    if not room.queue:
        return False

    room.queue.pop(room.current_index)
    room.current_index = max(0, min(room.current_index, len(room.queue) - 1))
    room.position = 0

    if not room.queue:
        room.is_playing = False
        return True

    room.is_playing = True
    room.last_sync_at = time.time()
    return True


def go_prev(room: Room) -> bool:
    """Go to previous track. Returns True if position changed."""
    if room.current_index > 0:
        room.current_index -= 1
        room.position = 0
        room.is_playing = True
        room.last_sync_at = time.time()
        return True
    return False


def jump_to(room: Room, index: int) -> bool:
    """Jump to a specific track by index. Returns True if successful."""
    if 0 <= index < len(room.queue):
        room.current_index = index
        room.position = 0
        room.is_playing = True
        room.last_sync_at = time.time()
        return True
    return False


def seek_to(room: Room, position: float) -> None:
    """Seek to a specific position in the current track."""
    room.position = position
    room.last_sync_at = time.time()

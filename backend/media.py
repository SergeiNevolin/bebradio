"""Local media storage for downloaded audio tracks.

Handles downloading audio from YouTube via yt-dlp, serving files, and
cleaning up expired media that is no longer referenced by any room queue.
"""

import asyncio
import glob
import logging
import os
import time
from pathlib import Path
from typing import Optional

from config import MEDIA_DIR, MEDIA_MAX_SIZE, MEDIA_TTL

log = logging.getLogger(__name__)

_media_dir: Optional[Path] = None


def init_media_dir() -> None:
    """Create the media storage directory on startup."""
    global _media_dir
    _media_dir = Path(MEDIA_DIR)
    _media_dir.mkdir(parents=True, exist_ok=True)
    log.info("media directory: %s", _media_dir.resolve())


def get_media_dir() -> Path:
    assert _media_dir is not None, "Media directory not initialized"
    return _media_dir


def get_track_path(track_id: str) -> Path:
    """Return the expected path for a track's audio file.

    The exact filename is not known until download (yt-dlp picks the extension),
    so we search for ``{track_id}.*``.
    """
    pattern = os.path.join(str(get_media_dir()), f"{track_id}.*")
    matches = glob.glob(pattern)
    if matches:
        return Path(matches[0])
    # Fallback: assume .m4a (most common from yt-dlp bestaudio)
    return get_media_dir() / f"{track_id}.m4a"


def is_downloaded(track_id: str) -> bool:
    """Check whether a track's audio file exists on disk."""
    return get_track_path(track_id).exists()


def get_local_filename(track_id: str) -> Optional[str]:
    """Return the actual filename on disk for a track, or None."""
    pattern = os.path.join(str(get_media_dir()), f"{track_id}.*")
    matches = glob.glob(pattern)
    if matches:
        return os.path.basename(matches[0])
    return None


def download_track(source_url: str, track_id: str) -> bool:
    """Download audio from YouTube to the local media directory."""
    media = get_media_dir()
    media.mkdir(parents=True, exist_ok=True)
    output_template = str(media / f"{track_id}.%(ext)s")

    # Check if already downloaded
    if is_downloaded(track_id):
        return True

    from youtube import YT_DLP_COMMON, _run_ytdlp

    args = [
        *YT_DLP_COMMON,
        "-f", "bestaudio/best",
        "--no-playlist",
        "-o", output_template,
        source_url,
    ]

    result = _run_ytdlp(args, timeout=120)
    if not result:
        log.warning("download_track: failed to download %s", source_url)
        return False

    if not is_downloaded(track_id):
        log.warning("download_track: file not found after download for %s", source_url)
        return False

    log.info("download_track: downloaded %s -> %s", source_url, get_track_path(track_id).name)
    return True


def delete_track_file(track_id: str) -> bool:
    """Delete a track's audio file from disk. Returns True if a file was removed."""
    path = get_track_path(track_id)
    if path.exists():
        path.unlink()
        log.info("delete_track_file: removed %s", path.name)
        return True
    return False


def _get_referenced_video_ids() -> set[str]:
    """Collect all video IDs whose media files are still in a room queue."""
    from store import rooms

    ids: set[str] = set()
    for room in rooms.values():
        for track in room.queue:
            if track.video_id:
                ids.add(track.video_id)
    return ids


def _get_media_dir_size() -> int:
    """Return total size in bytes of all files in the media directory."""
    total = 0
    for path in get_media_dir().iterdir():
        if path.is_file():
            try:
                total += path.stat().st_size
            except OSError:
                pass
    return total


def _enforce_size_limit(referenced: set[str]) -> int:
    """Delete the oldest unreferenced files until total size <= MEDIA_MAX_SIZE.

    Returns the number of files deleted.
    """
    total = _get_media_dir_size()
    if total <= MEDIA_MAX_SIZE:
        return 0

    media = get_media_dir()
    candidates = []
    for path in media.iterdir():
        if not path.is_file():
            continue
        if path.suffix == ".part":
            continue
        track_id = path.stem
        if track_id in referenced:
            continue
        try:
            stat = path.stat()
            candidates.append((path, stat.st_size, stat.st_mtime))
        except OSError:
            continue

    # Oldest files first
    candidates.sort(key=lambda x: x[2])

    deleted = 0
    for path, size, _ in candidates:
        if total <= MEDIA_MAX_SIZE:
            break
        try:
            path.unlink()
            total -= size
            deleted += 1
        except OSError:
            pass

    if deleted:
        log.info("enforce_size_limit: deleted %d files to bring media under %d bytes", deleted, MEDIA_MAX_SIZE)
    return deleted


async def cleanup_expired_media() -> int:
    """Remove media files that are not in any room queue and are older than TTL.

    Also enforces MEDIA_MAX_SIZE by evicting the oldest unreferenced files
    when the total size exceeds the limit.

    Returns the number of files deleted.
    """
    media = get_media_dir()
    referenced = await asyncio.to_thread(_get_referenced_video_ids)
    deleted = 0

    for path in media.iterdir():
        if not path.is_file():
            continue
        # Skip partial downloads still in progress
        if path.suffix == ".part":
            continue

        track_id = path.stem

        if track_id in referenced:
            continue

        try:
            age = time.time() - path.stat().st_mtime
        except OSError:
            continue

        if age > MEDIA_TTL:
            try:
                path.unlink()
                deleted += 1
            except OSError:
                pass

    deleted += await asyncio.to_thread(_enforce_size_limit, referenced)

    if deleted:
        log.info("cleanup_expired_media: deleted %d files total", deleted)
    return deleted

"""Dispatch track lookups to the right music platform.

Everything that used to call :mod:`youtube` directly goes through here instead,
so a track carries the name of the platform it came from (``Track.source``) and
each platform keeps its own search / fetch / stream-refresh implementation.
"""

from typing import Optional

import vk
import youtube
from config import DEFAULT_SOURCE, SOURCES, SOURCE_VK as VK, SOURCE_YOUTUBE as YOUTUBE

__all__ = [
    "DEFAULT_SOURCE", "SOURCES", "VK", "YOUTUBE",
    "detect_source", "fetch_track", "normalize", "resolve", "resolve_stream", "search",
]


def normalize(source: Optional[str]) -> str:
    """Map a caller-supplied source name onto a known one."""
    name = (source or "").strip().lower()
    return name if name in SOURCES else DEFAULT_SOURCE


def detect_source(url: str) -> Optional[str]:
    """The platform a pasted URL unambiguously belongs to, else ``None``."""
    if vk.is_vk_url(url):
        return VK
    text = url or ""
    if "youtube.com" in text or "youtu.be" in text:
        return YOUTUBE
    return None


def resolve(url: str, source: Optional[str] = None) -> str:
    """The source to use for ``url``.

    A recognisable URL wins over the caller's stated source — someone who
    pastes a YouTube link while the VK tab is selected means the link. Anything
    unrecognised falls back to the stated source (and ultimately to YouTube,
    whose yt-dlp backend handles plenty of other sites).
    """
    return detect_source(url) or normalize(source)


def search(query: str, limit: int = 5, source: Optional[str] = None) -> list[dict]:
    """Search one platform. Results always carry a ``source`` field."""
    name = normalize(source)
    if name == VK:
        return vk.search_vk(query, limit)
    results = youtube.search_youtube(query, limit)
    for item in results:
        item.setdefault("source", YOUTUBE)
    return results


def fetch_track(url: str, source: Optional[str] = None) -> Optional[dict]:
    """Fetch metadata plus a playable stream URL for a single track."""
    name = resolve(url, source)
    info = vk.fetch_track(url) if name == VK else youtube.fetch_track(url)
    if info is not None:
        info.setdefault("source", name)
    return info


def resolve_stream(source_url: str, source: Optional[str] = None) -> Optional[dict]:
    """Re-resolve an expired stream URL for a track already in a queue."""
    name = resolve(source_url, source)
    return vk.resolve_stream(source_url) if name == VK else youtube.resolve_stream(source_url)

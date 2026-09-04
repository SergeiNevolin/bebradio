"""VK Music provider.

Tracks are looked up through the official VK API (``audio.search`` /
``audio.getById``), which needs a user access token with the ``audio`` scope in
the ``VK_TOKEN`` environment variable. Without a token every call here returns
"nothing found" rather than raising, so a deployment that only cares about
YouTube keeps working untouched.
"""

import json
import re
import time
import urllib.parse
import urllib.request
from typing import Optional

from config import VK_API_URL, VK_API_VERSION, VK_CONVERT_HLS, VK_STREAM_TTL, VK_TOKEN

# ``vk.com/audio-2001053608_78053608``, ``audio123_456_abcdef0`` or the bare
# ``-123_456`` id pair. The optional third group is the access key VK hands out
# for tracks that are not publicly addressable.
_AUDIO_ID_RE = re.compile(r"(?:^|/|audio)(-?\d+_\d+(?:_[0-9a-f]+)?)(?:$|[?&#])")

# A VK stream URL is an HLS playlist: ``.../<hash>/index.m3u8``, sometimes with
# an ``/audios`` segment in the path. A bare <audio> element cannot play HLS,
# but the same content is served as a plain MP3 at the sibling path, so we
# rewrite the playlist URL into that. Set ``VK_CONVERT_HLS=0`` to hand the
# playlist URL to the client untouched (e.g. once the player learns HLS).
_M3U8_RE = re.compile(r"/[0-9a-f]+(?:/audios)?/([0-9a-f]+)/index\.m3u8")

_TIMEOUT = 20


def is_vk_url(url: str) -> bool:
    """Whether ``url`` looks like something this provider can resolve."""
    text = (url or "").strip()
    if not text:
        return False
    if "vk.com" in text or "vk.ru" in text or text.startswith("vk:"):
        return True
    # A bare ``audio-123_456`` / ``-123_456`` reference.
    return bool(re.fullmatch(r"(?:audio)?-?\d+_\d+(?:_[0-9a-f]+)?", text))


def audio_id(url: str) -> str:
    """Extract the ``<owner_id>_<track_id>[_<access_key>]`` pair from a URL."""
    m = _AUDIO_ID_RE.search((url or "").strip())
    return m.group(1) if m else ""


def track_url(item: dict) -> str:
    """Canonical vk.com page URL for an ``audio`` API object."""
    ident = f"{item.get('owner_id')}_{item.get('id')}"
    key = item.get("access_key")
    if key:
        ident = f"{ident}_{key}"
    return f"https://vk.com/audio{ident}"


def _api(method: str, **params) -> Optional[object]:
    """Call a VK API method, returning its ``response`` payload or ``None``."""
    if not VK_TOKEN:
        return None
    query = {k: v for k, v in params.items() if v not in (None, "")}
    query["access_token"] = VK_TOKEN
    query["v"] = VK_API_VERSION
    url = f"{VK_API_URL}{method}?{urllib.parse.urlencode(query)}"
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        with urllib.request.urlopen(req, timeout=_TIMEOUT) as resp:
            data = json.loads(resp.read().decode("utf-8", "replace"))
    except Exception:
        return None
    if not isinstance(data, dict) or "error" in data:
        return None
    return data.get("response")


def playable_url(raw: str) -> str:
    """Turn a VK HLS playlist URL into a directly playable MP3 URL."""
    if not raw or not VK_CONVERT_HLS:
        return raw or ""
    converted, count = _M3U8_RE.subn(r"/\1.mp3", raw)
    return converted if count else raw


def _thumbnail(item: dict) -> str:
    thumb = (item.get("album") or {}).get("thumb") or {}
    return thumb.get("photo_300") or thumb.get("photo_135") or ""


def _to_result(item: dict) -> Optional[dict]:
    """Shape a VK ``audio`` object like a result from ``search_vk``."""
    if not item.get("url"):
        return None
    return {
        "id": f"{item.get('owner_id')}_{item.get('id')}",
        "title": item.get("title", "Unknown"),
        "artist": item.get("artist", "Unknown"),
        "thumbnail": _thumbnail(item),
        "duration": item.get("duration", 0),
        "url": track_url(item),
        "source": "vk",
    }


def search_vk(query: str, limit: int = 5) -> list[dict]:
    """Search VK Music and return results in the shared search-result shape."""
    response = _api("audio.search", q=query, count=limit, auto_complete=1)
    if not isinstance(response, dict):
        return []
    results = []
    for item in response.get("items") or []:
        shaped = _to_result(item) if isinstance(item, dict) else None
        if shaped:
            results.append(shaped)
    return results[:limit]


def _get_by_id(source_url: str) -> Optional[dict]:
    ident = audio_id(source_url)
    if not ident:
        return None
    response = _api("audio.getById", audios=ident)
    if not isinstance(response, list) or not response:
        return None
    item = response[0]
    return item if isinstance(item, dict) and item.get("url") else None


def resolve_stream(source_url: str) -> Optional[dict]:
    """Re-resolve just the playable stream URL for an already-known track."""
    item = _get_by_id(source_url)
    if item is None:
        return None
    return {
        "stream_url": playable_url(item["url"]),
        "expires_at": time.time() + VK_STREAM_TTL,
    }


def fetch_track(url: str) -> Optional[dict]:
    """Fetch metadata and a fresh stream URL for a VK track."""
    item = _get_by_id(url)
    if item is None:
        return None
    return {
        "title": item.get("title", "Unknown"),
        "artist": item.get("artist", "Unknown"),
        "thumbnail": _thumbnail(item),
        "duration": item.get("duration", 0),
        "stream_url": playable_url(item["url"]),
        "source_url": track_url(item),
        "expires_at": time.time() + VK_STREAM_TTL,
    }

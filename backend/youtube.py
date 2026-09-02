import json
import re
import subprocess
import time
from typing import Optional


YT_DLP_COMMON = ["yt-dlp", "--js-runtimes", "node"]

# Fallback validity when a resolved stream URL carries no ``expire`` param.
_DEFAULT_STREAM_TTL = 5 * 3600

_VIDEO_ID_RE = re.compile(r"(?:v=|youtu\.be/|/shorts/|/embed/)([\w-]{11})")
_EXPIRE_RE = re.compile(r"[?&/]expire[=/](\d+)")


def video_id(url: str) -> str:
    """Extract the 11-char YouTube video id from a watch/short/embed URL."""
    m = _VIDEO_ID_RE.search(url or "")
    return m.group(1) if m else ""


def parse_stream_expiry(stream_url: str) -> float:
    """Epoch second at which a resolved googlevideo URL stops working."""
    m = _EXPIRE_RE.search(stream_url or "")
    if m:
        return float(m.group(1))
    return time.time() + _DEFAULT_STREAM_TTL


def _resolve_stream_url(url: str) -> Optional[str]:
    result = subprocess.run(
        [*YT_DLP_COMMON, "-f", "bestaudio[ext=m4a]/bestaudio", "-g", "--no-playlist", url],
        capture_output=True,
        text=True,
        timeout=60,
    )
    if result.returncode != 0:
        return None
    return result.stdout.strip().split("\n")[0] or None


def resolve_stream(source_url: str) -> Optional[dict]:
    """Re-resolve just the playable stream URL for an already-known video."""
    try:
        stream_url = _resolve_stream_url(source_url)
        if not stream_url:
            return None
        return {"stream_url": stream_url, "expires_at": parse_stream_expiry(stream_url)}
    except Exception:
        return None


def fetch_track(url: str) -> Optional[dict]:
    """Fetch video info and a fresh stream URL from YouTube."""
    try:
        info_result = subprocess.run(
            [*YT_DLP_COMMON, "--dump-json", "--no-download", "--no-playlist", url],
            capture_output=True,
            text=True,
            timeout=60,
        )
        if info_result.returncode != 0:
            return None
        data = json.loads(info_result.stdout)

        stream_url = _resolve_stream_url(url)
        if not stream_url:
            return None

        return {
            "title": data.get("title", "Unknown"),
            "artist": data.get("uploader", data.get("channel", "Unknown")),
            "thumbnail": data.get("thumbnail", ""),
            "duration": data.get("duration", 0),
            "stream_url": stream_url,
            "source_url": data.get("webpage_url") or url,
            "expires_at": parse_stream_expiry(stream_url),
        }
    except Exception:
        return None


def fetch_related(source_url: str, limit: int) -> list[str]:
    """Return YouTube watch URLs from the Mix (radio) playlist of a video."""
    vid = video_id(source_url)
    if not vid:
        return []
    mix_url = f"https://www.youtube.com/watch?v={vid}&list=RD{vid}"
    try:
        result = subprocess.run(
            [
                *YT_DLP_COMMON,
                mix_url,
                "--flat-playlist", "--dump-json", "--no-warnings",
                "-I", f"1:{limit}",
            ],
            capture_output=True,
            text=True,
            timeout=45,
        )
        if result.returncode != 0:
            return []

        urls = []
        for line in result.stdout.strip().split("\n"):
            if not line:
                continue
            try:
                data = json.loads(line)
            except json.JSONDecodeError:
                continue
            rid = data.get("id", "")
            if rid and rid != vid:
                urls.append(f"https://www.youtube.com/watch?v={rid}")
        return urls
    except Exception:
        return []


def search_youtube(query: str, limit: int = 5) -> list[dict]:
    """Search YouTube and return a list of results."""
    try:
        result = subprocess.run(
            [
                *YT_DLP_COMMON,
                f"ytsearch{limit}:{query}",
                "--dump-json", "--no-download", "--flat-playlist",
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode != 0:
            return []

        items = []
        for line in result.stdout.strip().split("\n"):
            if not line:
                continue
            try:
                data = json.loads(line)
                vid = data.get("id", "")
                items.append({
                    "id": vid,
                    "title": data.get("title", "Unknown"),
                    "artist": data.get("uploader", data.get("channel", "Unknown")),
                    "thumbnail": data.get("thumbnail", f"https://i.ytimg.com/vi/{vid}/hqdefault.jpg"),
                    "duration": data.get("duration", 0),
                    "url": f"https://www.youtube.com/watch?v={vid}",
                })
            except json.JSONDecodeError:
                continue
        return items
    except Exception:
        return []

import json
import re
import subprocess
import time
import urllib.request
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


# --- Subtitles / karaoke lyrics ---

# Parsed cue lists, keyed by "<video_id>:<lang>". The timedtext URLs handed out
# by YouTube expire quickly, but the cues we extract from them do not, so we
# cache the parsed result rather than re-shelling out on every request.
_SUBS_CACHE: dict[str, dict] = {}
_SUBS_CACHE_MAX = 256

# Preferred subtitle languages when the caller does not ask for a specific one.
_LANG_PREFERENCE = ["en", "en-US", "en-GB", "en-orig"]
# Preferred caption formats, easiest to parse first.
_SUB_FORMATS = ["json3", "vtt", "srv1"]

_VTT_TS_RE = re.compile(
    r"(\d{2}):(\d{2}):(\d{2})[.,](\d{3})\s*-->\s*(\d{2}):(\d{2}):(\d{2})[.,](\d{3})"
)
_VTT_TAG_RE = re.compile(r"<[^>]+>")


def _pick_lang(tracks: dict, want: str) -> str:
    """Choose a language key from a yt-dlp ``subtitles`` / ``captions`` dict."""
    if not tracks:
        return ""
    if want and want in tracks:
        return want
    if want:
        for key in tracks:
            if key.split("-")[0] == want.split("-")[0]:
                return key
    for pref in _LANG_PREFERENCE:
        if pref in tracks:
            return pref
    for key in tracks:
        if key.split("-")[0] == "en":
            return key
    return next(iter(tracks))


def _pick_format(entries: list[dict]) -> Optional[dict]:
    by_ext = {e.get("ext"): e for e in entries if e.get("url")}
    for ext in _SUB_FORMATS:
        if ext in by_ext:
            return by_ext[ext]
    return next(iter(by_ext.values()), None)


def _clean_text(text: str) -> str:
    text = _VTT_TAG_RE.sub("", text)
    text = text.replace("&nbsp;", " ").replace("&#39;", "'")
    text = text.replace("&quot;", '"').replace("&amp;", "&")
    return " ".join(text.split()).strip()


def _parse_json3(raw: str) -> list[dict]:
    data = json.loads(raw)
    cues: list[dict] = []
    for event in data.get("events", []):
        segs = event.get("segs")
        if not segs:
            continue
        text = _clean_text("".join(s.get("utf8", "") for s in segs))
        if not text:
            continue
        start = event.get("tStartMs", 0) / 1000.0
        dur = event.get("dDurationMs", 0) / 1000.0
        if cues and abs(cues[-1]["start"] - start) < 0.05 and cues[-1]["text"] == text:
            continue
        cues.append({"start": round(start, 3), "dur": round(dur, 3), "text": text})
    return cues


def _parse_vtt(raw: str) -> list[dict]:
    cues: list[dict] = []
    blocks = re.split(r"\r?\n\r?\n", raw)
    for block in blocks:
        lines = [ln for ln in block.splitlines() if ln.strip()]
        ts_line = next((ln for ln in lines if "-->" in ln), None)
        if not ts_line:
            continue
        m = _VTT_TS_RE.search(ts_line)
        if not m:
            continue
        h1, m1, s1, ms1, h2, m2, s2, ms2 = (int(x) for x in m.groups())
        start = h1 * 3600 + m1 * 60 + s1 + ms1 / 1000.0
        end = h2 * 3600 + m2 * 60 + s2 + ms2 / 1000.0
        body = " ".join(lines[lines.index(ts_line) + 1:])
        text = _clean_text(body)
        if not text:
            continue
        if cues and cues[-1]["text"] == text:
            continue
        cues.append({
            "start": round(start, 3),
            "dur": round(max(end - start, 0.0), 3),
            "text": text,
        })
    return cues


def _download(url: str) -> Optional[str]:
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
        with urllib.request.urlopen(req, timeout=15) as resp:
            return resp.read().decode("utf-8", "replace")
    except Exception:
        return None


def fetch_subtitles(source_url: str, lang: str = "") -> dict:
    """Return timed caption cues for a YouTube video, for the karaoke view.

    ``{"lang": str, "auto": bool, "cues": [{"start", "dur", "text"}]}``. An empty
    ``cues`` list means the video has no usable captions (or the lookup failed).
    """
    vid = video_id(source_url)
    empty = {"lang": "", "auto": False, "cues": []}
    if not vid:
        return empty

    cache_key = f"{vid}:{lang}"
    if cache_key in _SUBS_CACHE:
        return _SUBS_CACHE[cache_key]

    result = empty
    try:
        # ``--dump-json`` already lists every caption track (manual and
        # auto-generated) with a downloadable URL; no need to write files.
        info = subprocess.run(
            [*YT_DLP_COMMON, "--dump-json", "--no-download", "--no-playlist",
             f"https://www.youtube.com/watch?v={vid}"],
            capture_output=True,
            text=True,
            timeout=60,
        )
        if info.returncode == 0:
            data = json.loads(info.stdout)
            manual = data.get("subtitles") or {}
            auto = data.get("automatic_captions") or {}
            want = lang or (data.get("language") or "")

            is_auto = False
            chosen_lang = _pick_lang(manual, want)
            entries = manual.get(chosen_lang) if chosen_lang else None
            if not entries:
                is_auto = True
                chosen_lang = _pick_lang(auto, want)
                entries = auto.get(chosen_lang) if chosen_lang else None

            entry = _pick_format(entries or [])
            if entry:
                raw = _download(entry["url"])
                if raw:
                    parser = _parse_json3 if entry.get("ext") == "json3" else _parse_vtt
                    try:
                        cues = parser(raw)
                    except Exception:
                        cues = []
                    if cues:
                        result = {"lang": chosen_lang, "auto": is_auto, "cues": cues}
    except Exception:
        result = empty

    if len(_SUBS_CACHE) >= _SUBS_CACHE_MAX:
        _SUBS_CACHE.pop(next(iter(_SUBS_CACHE)))
    _SUBS_CACHE[cache_key] = result
    return result


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

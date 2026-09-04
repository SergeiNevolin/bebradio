import json
import logging
import re
import subprocess
import time
import urllib.request
from typing import Optional

from config import BGUTIL_BASE_URL

log = logging.getLogger(__name__)

YT_DLP_COMMON = [
    "yt-dlp",
    "--remote-components", "ejs:github",
    "--extractor-args", f"youtubepot-bgutilhttp:base_url={BGUTIL_BASE_URL}",
]

_YT_DLP_MAX_RETRIES = 2
_YT_DLP_RETRY_DELAY = 2.0

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


def _run_ytdlp(args: list[str], timeout: int = 60) -> Optional[subprocess.CompletedProcess]:
    """Run yt-dlp with automatic retry on transient failures."""
    for attempt in range(_YT_DLP_MAX_RETRIES):
        try:
            result = subprocess.run(
                args,
                capture_output=True,
                text=True,
                timeout=timeout,
            )
            if result.returncode == 0:
                return result
            if attempt < _YT_DLP_MAX_RETRIES - 1:
                log.warning(
                    "yt-dlp exited with code %d (attempt %d/%d), retrying...",
                    result.returncode, attempt + 1, _YT_DLP_MAX_RETRIES,
                )
                if result.stderr:
                    log.warning("yt-dlp stderr: %s", result.stderr.strip()[:500])
                time.sleep(_YT_DLP_RETRY_DELAY * (attempt + 1))
            else:
                log.error(
                    "yt-dlp failed after %d attempts, exit code %d, args: %s",
                    _YT_DLP_MAX_RETRIES, result.returncode, args[1:4],
                )
                if result.stderr:
                    log.error("yt-dlp stderr: %s", result.stderr.strip()[:1000])
        except subprocess.TimeoutExpired:
            if attempt < _YT_DLP_MAX_RETRIES - 1:
                log.warning(
                    "yt-dlp timed out (attempt %d/%d), retrying...",
                    attempt + 1, _YT_DLP_MAX_RETRIES,
                )
                time.sleep(_YT_DLP_RETRY_DELAY * (attempt + 1))
            else:
                log.error("yt-dlp timed out after %d attempts, args: %s", _YT_DLP_MAX_RETRIES, args[1:4])
        except Exception:
            log.exception("yt-dlp raised unexpected exception")
            return None
    return None


def fetch_track(url: str) -> Optional[dict]:
    """Fetch video metadata from YouTube (no stream URL — files are downloaded locally)."""
    try:
        info_result = _run_ytdlp(
            [*YT_DLP_COMMON, "--dump-json", "--no-download", "--no-playlist", url],
        )
        if not info_result:
            log.warning("fetch_track: yt-dlp failed to get info for %s", url)
            return None
        data = json.loads(info_result.stdout)

        return {
            "title": data.get("title", "Unknown"),
            "artist": data.get("uploader", data.get("channel", "Unknown")),
            "thumbnail": data.get("thumbnail", ""),
            "duration": data.get("duration", 0),
            "source_url": data.get("webpage_url") or url,
        }
    except Exception:
        log.exception("fetch_track: unexpected error for %s", url)
        return None


def fetch_related(source_url: str, limit: int) -> list[str]:
    """Return YouTube watch URLs from the Mix (radio) playlist of a video."""
    vid = video_id(source_url)
    if not vid:
        return []
    mix_url = f"https://www.youtube.com/watch?v={vid}&list=RD{vid}"
    try:
        result = _run_ytdlp(
            [
                *YT_DLP_COMMON,
                mix_url,
                "--flat-playlist", "--dump-json", "--no-warnings",
                "-I", f"1:{limit}",
            ],
            timeout=45,
        )
        if not result:
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
        info = _run_ytdlp(
            [*YT_DLP_COMMON, "--dump-json", "--no-download", "--no-playlist",
             f"https://www.youtube.com/watch?v={vid}"],
        )
        if info:
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
        result = _run_ytdlp(
            [
                *YT_DLP_COMMON,
                f"ytsearch{limit}:{query}",
                "--dump-json", "--no-download", "--flat-playlist",
            ],
            timeout=30,
        )
        if not result:
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

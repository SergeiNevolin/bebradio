import hashlib
import json
import logging
import re
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Optional

from config import Settings

log = logging.getLogger(__name__)

_VIDEO_ID_RE = re.compile(r"(?:v=|youtu\.be/|/shorts/|/embed/)([\w-]{11})")
_LANGS = ["en", "en-US", "en-GB", "en-orig"]
_VTT_TS = re.compile(
    r"(\d{2}):(\d{2}):(\d{2})[.,](\d{3})\s*-->\s*"
    r"(\d{2}):(\d{2}):(\d{2})[.,](\d{3})"
)
_TAGS = re.compile(r"<[^>]+>")


class YouTubeProvider:
    name = "youtube"

    def __init__(self, settings: Settings) -> None:
        self._common_args = [
            "yt-dlp",
            "--remote-components", "ejs:github",
            "--extractor-args", f"youtubepot-bgutilhttp:base_url={settings.bgutil_base_url}",
        ]
        self._subtitle_cache: dict[str, dict] = {}

    @staticmethod
    def provider_item_id(url: str) -> str:
        match = _VIDEO_ID_RE.search(url or "")
        return match.group(1) if match else ""

    @classmethod
    def media_id(cls, provider_item_id: str) -> str:
        digest = hashlib.sha256(f"{cls.name}:{provider_item_id}".encode()).hexdigest()
        return f"media_{digest[:32]}"

    def _run(self, args: list[str], timeout: int = 60) -> Optional[subprocess.CompletedProcess]:
        for attempt in range(2):
            try:
                result = subprocess.run(args, capture_output=True, text=True, timeout=timeout)
                if result.returncode == 0:
                    return result
                if attempt == 0:
                    time.sleep(2)
            except (subprocess.TimeoutExpired, OSError):
                if attempt == 1:
                    log.exception("yt-dlp failed")
        return None

    def resolve(self, url: str) -> Optional[dict]:
        result = self._run([*self._common_args, "--dump-json", "--no-download", "--no-playlist", url])
        if not result:
            return None
        try:
            data = json.loads(result.stdout)
        except json.JSONDecodeError:
            return None
        provider_item_id = data.get("id") or self.provider_item_id(url)
        if not provider_item_id:
            return None
        return {
            "media_id": self.media_id(provider_item_id),
            "title": data.get("title", "Unknown"),
            "artist": data.get("uploader", data.get("channel", "Unknown")),
            "thumbnail": data.get("thumbnail", ""),
            "duration": data.get("duration", 0),
            "source_url": data.get("webpage_url") or url,
        }

    def search(self, query: str, limit: int) -> list[dict]:
        result = self._run([
            *self._common_args, f"ytsearch{limit}:{query}",
            "--dump-json", "--no-download", "--flat-playlist",
        ], 30)
        if not result:
            return []
        items = []
        for line in result.stdout.strip().splitlines():
            try:
                data = json.loads(line)
            except json.JSONDecodeError:
                continue
            provider_item_id = data.get("id", "")
            if not provider_item_id:
                continue
            media_id = self.media_id(provider_item_id)
            items.append({
                "id": media_id,
                "media_id": media_id,
                "title": data.get("title", "Unknown"),
                "artist": data.get("uploader", data.get("channel", "Unknown")),
                "thumbnail": data.get("thumbnail", f"https://i.ytimg.com/vi/{provider_item_id}/hqdefault.jpg"),
                "duration": data.get("duration", 0),
                "url": f"https://www.youtube.com/watch?v={provider_item_id}",
            })
        return items

    def related(self, source_url: str, limit: int) -> list[str]:
        provider_item_id = self.provider_item_id(source_url)
        if not provider_item_id:
            return []
        mix_url = f"https://www.youtube.com/watch?v={provider_item_id}&list=RD{provider_item_id}"
        result = self._run([
            *self._common_args, mix_url, "--flat-playlist", "--dump-json",
            "--no-warnings", "-I", f"1:{limit}",
        ], 45)
        if not result:
            return []
        urls = []
        for line in result.stdout.strip().splitlines():
            try:
                related_id = json.loads(line).get("id", "")
            except json.JSONDecodeError:
                continue
            if related_id and related_id != provider_item_id:
                urls.append(f"https://www.youtube.com/watch?v={related_id}")
        return urls

    def download(self, source_url: str, output_path: Path) -> bool:
        result = self._run([
            *self._common_args, "-f", "bestaudio/best", "--no-playlist",
            "-o", str(output_path.with_suffix(".%(ext)s")), source_url,
        ], 120)
        return bool(result)

    def captions(self, source_url: str, lang: str) -> dict:
        provider_item_id = self.provider_item_id(source_url)
        empty = {"lang": "", "auto": False, "cues": []}
        if not provider_item_id:
            return empty
        cache_key = f"{provider_item_id}:{lang}"
        if cache_key in self._subtitle_cache:
            return self._subtitle_cache[cache_key]

        result = empty
        info = self._run([
            *self._common_args, "--dump-json", "--no-download", "--no-playlist",
            f"https://www.youtube.com/watch?v={provider_item_id}",
        ])
        if info:
            try:
                data = json.loads(info.stdout)
                manual = data.get("subtitles") or {}
                automatic = data.get("automatic_captions") or {}
                tracks = manual or automatic
                chosen = lang if lang in tracks else next((item for item in _LANGS if item in tracks), next(iter(tracks), ""))
                entry = next((item for item in tracks.get(chosen, []) if item.get("ext") in ("vtt", "json3") and item.get("url")), None)
                if entry:
                    request = urllib.request.Request(entry["url"], headers={"User-Agent": "Mozilla/5.0"})
                    with urllib.request.urlopen(request, timeout=15) as response:
                        raw = response.read().decode("utf-8", "replace")
                    cues = self._parse_vtt(raw) if entry.get("ext") == "vtt" else []
                    result = {"lang": chosen, "auto": not bool(manual), "cues": cues}
            except urllib.error.HTTPError as exc:
                if exc.code == 429:
                    log.warning("captions rate-limited by YouTube for %s", provider_item_id)
                else:
                    log.warning("captions request failed for %s: HTTP %s", provider_item_id, exc.code)
            except urllib.error.URLError as exc:
                log.warning("captions request failed for %s: %s", provider_item_id, exc.reason)
            except Exception:
                log.exception("caption lookup failed")

        if len(self._subtitle_cache) >= 256:
            self._subtitle_cache.pop(next(iter(self._subtitle_cache)))
        self._subtitle_cache[cache_key] = result
        return result

    @staticmethod
    def _parse_vtt(raw: str) -> list[dict]:
        cues = []
        for block in re.split(r"\r?\n\r?\n", raw):
            lines = [line for line in block.splitlines() if line.strip()]
            timestamp = next((line for line in lines if "-->" in line), "")
            match = _VTT_TS.search(timestamp)
            if not match:
                continue
            values = [int(value) for value in match.groups()]
            start = values[0] * 3600 + values[1] * 60 + values[2] + values[3] / 1000
            end = values[4] * 3600 + values[5] * 60 + values[6] + values[7] / 1000
            text = _TAGS.sub("", " ".join(lines[lines.index(timestamp) + 1:]))
            text = " ".join(text.replace("&amp;", "&").replace("&#39;", "'").split())
            if text:
                cues.append({"start": round(start, 3), "dur": round(max(end - start, 0), 3), "text": text})
        return cues

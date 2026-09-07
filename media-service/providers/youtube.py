import hashlib
import json
import logging
import re
import subprocess
import time
from pathlib import Path
from typing import Optional

from config import Settings

log = logging.getLogger(__name__)

_VIDEO_ID_RE = re.compile(r"(?:v=|youtu\.be/|/shorts/|/embed/)([\w-]{11})")
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

    _SUB_LANGS = "en,en-orig,ru,ru-orig"

    def download(self, source_url: str, output_path: Path) -> bool:
        audio_tmpl = str(output_path.with_suffix(".%(ext)s"))
        if not self._run([
            *self._common_args, "-f", "bestaudio/best", "--no-playlist",
            "-o", audio_tmpl, source_url,
        ], 120):
            return False

        # Captions are best-effort and fetched in their own passes: a YouTube
        # rate-limit on subtitles must not fail the audio we already have.
        # Manual and ASR captions land in separate files
        # (media_<hash>.<lang>.vtt vs media_<hash>.auto.<lang>.vtt) so the reader
        # can prefer manual and label auto-generated captions honestly. Exact
        # language codes only -- "en.*" would also pull auto-translations. No
        # --convert-subs: ffmpeg is not in the image and YouTube serves vtt.
        auto_tmpl = f"subtitle:{output_path.with_suffix('')}.auto.%(ext)s"
        self._run([
            *self._common_args, "--skip-download", "--no-playlist", "--write-subs",
            "--sub-langs", self._SUB_LANGS, "--sub-format", "vtt/best",
            "-o", audio_tmpl, source_url,
        ], 60)
        self._run([
            *self._common_args, "--skip-download", "--no-playlist", "--write-auto-subs",
            "--sub-langs", self._SUB_LANGS, "--sub-format", "vtt/best",
            "-o", auto_tmpl, source_url,
        ], 60)
        return True

    @classmethod
    def parse_vtt_file(cls, path: Path) -> list[dict]:
        """Read and parse a .vtt written by `download()` into timed cues."""
        return cls._parse_vtt(path.read_text(encoding="utf-8", errors="replace"))

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

"""Persistent storage and ffmpeg transcoding for the ``/mashup`` section.

Unlike :mod:`storage`, this module never expires files by TTL: mashups are
user-owned uploads whose lifetime is decided by the Go backend (Postgres row).
The mashup directory is deliberately a *sibling* of ``media_dir`` so that
``MediaStorage.cleanup()`` / ``_enforce_size_limit()`` never walk into it.
"""

import asyncio
import json
import logging
import re
import subprocess
from dataclasses import dataclass
from pathlib import Path

from config import Settings

log = logging.getLogger(__name__)
_MASHUP_ID_RE = re.compile(r"[A-Za-z0-9_-]+")


class MashupError(Exception):
    """Raised when an upload fails validation or transcoding."""


@dataclass
class ProbeResult:
    duration: float
    title: str
    artist: str


def _ffmpeg_error(prefix: str, proc: subprocess.CompletedProcess) -> str:
    """Append the last few lines of ffmpeg's stderr so the failure is legible
    in logs and on the mashup card."""
    tail = [line.strip() for line in (proc.stderr or "").strip().splitlines() if line.strip()]
    detail = " | ".join(tail[-3:])
    return f"{prefix}: {detail}" if detail else prefix


class MashupStorage:
    def __init__(self, settings: Settings) -> None:
        self.settings = settings

    def init(self) -> None:
        self.settings.mashup_dir.mkdir(parents=True, exist_ok=True)

    @staticmethod
    def valid_id(media_id: str) -> bool:
        return bool(media_id and _MASHUP_ID_RE.fullmatch(media_id))

    def path(self, media_id: str) -> Path:
        return self.settings.mashup_dir / f"{media_id}.m4a"

    def cover_path(self, media_id: str) -> Path:
        return self.settings.mashup_dir / f"{media_id}.jpg"

    def part_path(self, media_id: str) -> Path:
        return self.settings.mashup_dir / f"{media_id}.part"

    def cover_part_path(self, media_id: str) -> Path:
        return self.settings.mashup_dir / f"{media_id}.cover.part"

    def is_ready(self, media_id: str) -> bool:
        return self.path(media_id).is_file()

    async def save_upload(self, media_id: str, upload) -> Path:
        """Stream the multipart body to ``<media_id>.part`` in fixed chunks.

        Aborts and unlinks the partial file the moment it exceeds
        ``mashup_max_size`` so a hostile client cannot fill the disk.
        """
        self.init()
        part = self.part_path(media_id)
        size = 0
        try:
            with part.open("wb") as handle:
                while True:
                    chunk = await upload.read(1024 * 1024)
                    if not chunk:
                        break
                    size += len(chunk)
                    if size > self.settings.mashup_max_size:
                        raise MashupError("uploaded file is too large")
                    handle.write(chunk)
        except MashupError:
            part.unlink(missing_ok=True)
            raise
        if size == 0:
            part.unlink(missing_ok=True)
            raise MashupError("uploaded file is empty")
        return part

    async def save_cover_upload(self, media_id: str, upload) -> Path:
        """Stream a user-supplied cover image to ``<media_id>.cover.part``.

        Mirrors :meth:`save_upload`: aborts and unlinks the moment the body
        exceeds ``mashup_cover_max_size`` so a hostile client cannot fill the disk.
        """
        self.init()
        part = self.cover_part_path(media_id)
        size = 0
        try:
            with part.open("wb") as handle:
                while True:
                    chunk = await upload.read(1024 * 1024)
                    if not chunk:
                        break
                    size += len(chunk)
                    if size > self.settings.mashup_cover_max_size:
                        raise MashupError("cover image is too large")
                    handle.write(chunk)
        except MashupError:
            part.unlink(missing_ok=True)
            raise
        if size == 0:
            part.unlink(missing_ok=True)
            raise MashupError("cover image is empty")
        return part

    def process_cover(self, media_id: str, src: Path) -> None:
        """Validate and normalise a cover image into ``<media_id>.jpg``.

        ffmpeg both rejects non-images (non-zero exit) and strips metadata while
        clamping the longest side to 1000px. Writes to a temp file next to the
        target and atomically renames it in.
        """
        tmp = self.settings.mashup_dir / f"{media_id}.jpg.tmp"
        proc = subprocess.run(
            [
                "ffmpeg", "-nostdin", "-y", "-i", str(src),
                "-an", "-vf",
                "scale='min(1000,iw)':'min(1000,ih)':force_original_aspect_ratio=decrease",
                "-frames:v", "1", "-f", "mjpeg", str(tmp),
            ],
            capture_output=True, text=True,
        )
        src.unlink(missing_ok=True)
        if proc.returncode != 0 or not tmp.is_file() or tmp.stat().st_size == 0:
            tmp.unlink(missing_ok=True)
            raise MashupError(_ffmpeg_error("cover processing failed", proc))
        tmp.rename(self.cover_path(media_id))

    def probe(self, src: Path) -> ProbeResult:
        """Reject non-audio and over-long files; read title/artist from tags."""
        proc = subprocess.run(
            [
                "ffprobe", "-v", "quiet", "-print_format", "json",
                "-show_format", "-show_streams", str(src),
            ],
            capture_output=True, text=True,
        )
        if proc.returncode != 0:
            raise MashupError("could not read the uploaded file")
        try:
            data = json.loads(proc.stdout or "{}")
        except json.JSONDecodeError as exc:
            raise MashupError("could not read the uploaded file") from exc

        streams = data.get("streams") or []
        if not any(stream.get("codec_type") == "audio" for stream in streams):
            raise MashupError("the file contains no audio stream")

        fmt = data.get("format") or {}
        try:
            duration = float(fmt.get("duration") or 0.0)
        except (TypeError, ValueError):
            duration = 0.0
        if duration > self.settings.mashup_max_duration:
            raise MashupError("the track is longer than allowed")

        tags = {str(k).lower(): v for k, v in (fmt.get("tags") or {}).items()}
        return ProbeResult(
            duration=duration,
            title=str(tags.get("title") or "").strip(),
            artist=str(tags.get("artist") or tags.get("album_artist") or "").strip(),
        )

    def transcode(self, media_id: str, src: Path) -> None:
        """Transcode to m4a/AAC 192k with EBU R128 loudness normalisation.

        Writes to a temp file next to the target and atomically renames it in.
        The output muxer is forced with ``-f ipod`` because the ``.tmp`` suffix
        gives ffmpeg no extension to guess an m4a container from.
        """
        tmp = self.settings.mashup_dir / f"{media_id}.m4a.tmp"
        proc = subprocess.run(
            [
                "ffmpeg", "-nostdin", "-y", "-i", str(src),
                "-vn", "-map", "0:a:0",
                "-c:a", "aac", "-b:a", "192k", "-ar", "44100",
                "-af", "loudnorm=I=-14:TP=-1.5:LRA=11",
                "-movflags", "+faststart",
                "-f", "ipod", str(tmp),
            ],
            capture_output=True, text=True,
        )
        if proc.returncode != 0 or not tmp.is_file() or tmp.stat().st_size == 0:
            tmp.unlink(missing_ok=True)
            raise MashupError(_ffmpeg_error("transcoding failed", proc))
        tmp.rename(self.path(media_id))

    def extract_cover(self, media_id: str, src: Path) -> bool:
        """Best-effort: pull an embedded cover into ``<media_id>.jpg``."""
        out = self.cover_path(media_id)
        proc = subprocess.run(
            [
                "ffmpeg", "-nostdin", "-y", "-i", str(src),
                "-an", "-map", "0:v?", "-c:v", "mjpeg", "-frames:v", "1",
                "-f", "mjpeg", str(out),
            ],
            capture_output=True, text=True,
        )
        if proc.returncode != 0 or not out.is_file() or out.stat().st_size == 0:
            out.unlink(missing_ok=True)
            return False
        return True

    def delete(self, media_id: str) -> None:
        for path in (
            self.path(media_id),
            self.cover_path(media_id),
            self.part_path(media_id),
            self.cover_part_path(media_id),
        ):
            path.unlink(missing_ok=True)

    def total_size(self) -> int:
        directory = self.settings.mashup_dir
        if not directory.is_dir():
            return 0
        return sum(path.stat().st_size for path in directory.iterdir() if path.is_file())


@dataclass
class JobState:
    status: str = "processing"
    duration: float = 0.0
    title: str = ""
    artist: str = ""
    has_cover: bool = False
    error: str = ""

    def as_dict(self) -> dict:
        return {
            "status": self.status,
            "duration": self.duration,
            "title": self.title,
            "artist": self.artist,
            "has_cover": self.has_cover,
            "error": self.error,
        }


class MashupJobs:
    """In-memory registry of running transcode jobs.

    The registry is lost on restart, so :meth:`status` falls back to the
    filesystem when a job is unknown: ``.m4a`` present -> ``ready``, ``.part``
    present -> ``failed`` (an orphaned job). The authoritative state lives in
    Postgres on the Go side.
    """

    def __init__(self, storage: MashupStorage, settings: Settings) -> None:
        self.storage = storage
        self._semaphore = asyncio.Semaphore(settings.mashup_max_jobs)
        self._jobs: dict[str, JobState] = {}
        self._tasks: set[asyncio.Task] = set()

    def __contains__(self, media_id: str) -> bool:
        return media_id in self._jobs

    def submit(self, media_id: str, src: Path) -> None:
        self._jobs[media_id] = JobState(status="processing")
        task = asyncio.create_task(self._run(media_id, src))
        self._tasks.add(task)
        task.add_done_callback(self._tasks.discard)

    async def _run(self, media_id: str, src: Path) -> None:
        async with self._semaphore:
            try:
                probe = await asyncio.to_thread(self.storage.probe, src)
                await asyncio.to_thread(self.storage.transcode, media_id, src)
                # A cover uploaded by the user while the job ran must win over
                # whatever art is embedded in the source file.
                if not self.storage.cover_path(media_id).is_file():
                    await asyncio.to_thread(self.storage.extract_cover, media_id, src)
                has_cover = self.storage.cover_path(media_id).is_file()
            except MashupError as exc:
                self._jobs[media_id] = JobState(status="failed", error=str(exc))
                log.warning("mashup %s failed: %s", media_id, exc)
                return
            except Exception:  # noqa: BLE001 - never let a job kill the loop
                self._jobs[media_id] = JobState(status="failed", error="internal error")
                log.exception("mashup %s crashed", media_id)
                return
            self._jobs[media_id] = JobState(
                status="ready",
                duration=probe.duration,
                title=probe.title,
                artist=probe.artist,
                has_cover=has_cover,
            )
            src.unlink(missing_ok=True)

    def status(self, media_id: str) -> JobState | None:
        job = self._jobs.get(media_id)
        if job is not None:
            return job
        if self.storage.path(media_id).is_file():
            return JobState(
                status="ready",
                has_cover=self.storage.cover_path(media_id).is_file(),
            )
        if self.storage.part_path(media_id).is_file():
            return JobState(status="failed", error="orphaned job")
        return None

    def discard(self, media_id: str) -> None:
        self._jobs.pop(media_id, None)

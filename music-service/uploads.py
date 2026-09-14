"""Persistent storage and ffmpeg transcoding for the ``/mashup`` section.

Unlike :mod:`storage`, this module never expires files by TTL: uploads are
user-owned uploads whose lifetime is decided by the Go backend (Postgres row).
Finished audio/covers live in MinIO under ``uploads/``; local disk only holds
in-progress uploads and ffmpeg scratch files inside ``work_dir``.
"""

import asyncio
import json
import logging
import re
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Iterator, Optional

from config import Settings
from s3store import S3Store

log = logging.getLogger(__name__)
_UPLOAD_ID_RE = re.compile(r"[A-Za-z0-9_-]+")
UPLOADS_PREFIX = "uploads/"


class UploadError(Exception):
    """Raised when an upload fails validation or transcoding."""


@dataclass
class ProbeResult:
    duration: float
    title: str
    artist: str


def _ffmpeg_error(prefix: str, proc: subprocess.CompletedProcess) -> str:
    """Append the last few lines of ffmpeg's stderr so the failure is legible
    in logs and on the track card."""
    tail = [line.strip() for line in (proc.stderr or "").strip().splitlines() if line.strip()]
    detail = " | ".join(tail[-3:])
    return f"{prefix}: {detail}" if detail else prefix


class UploadStorage:
    def __init__(self, settings: Settings, s3: Optional[S3Store] = None) -> None:
        self.settings = settings
        self._s3 = s3 if s3 is not None else S3Store.from_settings(settings)

    def init(self) -> None:
        (self.settings.work_dir / "uploads").mkdir(parents=True, exist_ok=True)
        self._s3.ensure_bucket()
        self._migrate_legacy_prefix()

    def _migrate_legacy_prefix(self) -> None:
        """One-way move of pre-unification ``mashups/`` objects to ``uploads/``.

        Idempotent: only missing targets are copied, sources are deleted after
        a successful copy, so a crash mid-migration is safe to re-run.
        """
        try:
            legacy = self._s3.list("mashups/")
        except Exception:
            log.exception("uploads migration: could not list legacy prefix")
            return
        for obj in legacy:
            target = UPLOADS_PREFIX + obj.key[len("mashups/"):]
            try:
                if not self._s3.exists(target):
                    self._s3.copy(obj.key, target)
                self._s3.delete(obj.key)
            except Exception:
                log.warning("uploads migration: could not move %s", obj.key)

    @staticmethod
    def valid_id(media_id: str) -> bool:
        return bool(media_id and _UPLOAD_ID_RE.fullmatch(media_id))

    @staticmethod
    def audio_key(media_id: str) -> str:
        return f"{UPLOADS_PREFIX}{media_id}.m4a"

    @staticmethod
    def cover_key(media_id: str) -> str:
        return f"{UPLOADS_PREFIX}{media_id}.jpg"

    def _stage_dir(self) -> Path:
        directory = self.settings.work_dir / "uploads"
        directory.mkdir(parents=True, exist_ok=True)
        return directory

    def stage_path(self, media_id: str) -> Path:
        return self._stage_dir() / f"{media_id}.part"

    def cover_stage_path(self, media_id: str) -> Path:
        return self._stage_dir() / f"{media_id}.cover.part"

    def exists(self, media_id: str) -> bool:
        return self._s3.exists(self.audio_key(media_id))

    def cover_exists(self, media_id: str) -> bool:
        return self._s3.exists(self.cover_key(media_id))

    # Back-compat alias: the audio file being present means "ready".
    def is_ready(self, media_id: str) -> bool:
        return self.exists(media_id)

    def audio_size(self, media_id: str) -> Optional[int]:
        return self._s3.size(self.audio_key(media_id))

    def audio_stream(
        self, media_id: str, start: int | None = None, end: int | None = None
    ) -> tuple[Iterator[bytes], int] | None:
        return self._s3.stream(self.audio_key(media_id), start, end)

    def read_cover(self, media_id: str) -> Optional[bytes]:
        return self._s3.get_bytes(self.cover_key(media_id))

    async def save_upload(self, media_id: str, upload) -> Path:
        """Stream the multipart body to a staging file in fixed chunks.

        Aborts and unlinks the partial file the moment it exceeds
        ``mashup_max_size`` so a hostile client cannot fill the disk.
        """
        part = self.stage_path(media_id)
        size = 0
        try:
            with part.open("wb") as handle:
                while True:
                    chunk = await upload.read(1024 * 1024)
                    if not chunk:
                        break
                    size += len(chunk)
                    if size > self.settings.mashup_max_size:
                        raise UploadError("uploaded file is too large")
                    handle.write(chunk)
        except UploadError:
            part.unlink(missing_ok=True)
            raise
        if size == 0:
            part.unlink(missing_ok=True)
            raise UploadError("uploaded file is empty")
        return part

    async def save_cover_upload(self, media_id: str, upload) -> Path:
        """Stream a user-supplied cover image to a staging file.

        Mirrors :meth:`save_upload`: aborts and unlinks the moment the body
        exceeds ``mashup_cover_max_size`` so a hostile client cannot fill the disk.
        """
        part = self.cover_stage_path(media_id)
        size = 0
        try:
            with part.open("wb") as handle:
                while True:
                    chunk = await upload.read(1024 * 1024)
                    if not chunk:
                        break
                    size += len(chunk)
                    if size > self.settings.mashup_cover_max_size:
                        raise UploadError("cover image is too large")
                    handle.write(chunk)
        except UploadError:
            part.unlink(missing_ok=True)
            raise
        if size == 0:
            part.unlink(missing_ok=True)
            raise UploadError("cover image is empty")
        return part

    def process_cover(self, media_id: str, src: Path) -> None:
        """Validate and normalise a cover image into ``uploads/<media_id>.jpg``.

        ffmpeg both rejects non-images (non-zero exit) and strips metadata while
        clamping the longest side to 1000px. Transcodes to a temp file next to
        the staging area, uploads it, then cleans up.
        """
        tmp = self._stage_dir() / f"{media_id}.jpg.tmp"
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
            raise UploadError(_ffmpeg_error("cover processing failed", proc))
        try:
            self._s3.put_file(self.cover_key(media_id), tmp)
        finally:
            tmp.unlink(missing_ok=True)

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
            raise UploadError("could not read the uploaded file")
        try:
            data = json.loads(proc.stdout or "{}")
        except json.JSONDecodeError as exc:
            raise UploadError("could not read the uploaded file") from exc

        streams = data.get("streams") or []
        if not any(stream.get("codec_type") == "audio" for stream in streams):
            raise UploadError("the file contains no audio stream")

        fmt = data.get("format") or {}
        try:
            duration = float(fmt.get("duration") or 0.0)
        except (TypeError, ValueError):
            duration = 0.0
        if duration > self.settings.mashup_max_duration:
            raise UploadError("the track is longer than allowed")

        tags = {str(k).lower(): v for k, v in (fmt.get("tags") or {}).items()}
        return ProbeResult(
            duration=duration,
            title=str(tags.get("title") or "").strip(),
            artist=str(tags.get("artist") or tags.get("album_artist") or "").strip(),
        )

    def transcode(self, media_id: str, src: Path) -> None:
        """Transcode to m4a/AAC 192k with EBU R128 loudness normalisation.

        Transcodes to a temp file in the staging area and uploads it to
        ``uploads/<media_id>.m4a``. The output muxer is forced with
        ``-f ipod`` because the ``.tmp`` suffix gives ffmpeg no extension to
        guess an m4a container from.
        """
        tmp = self._stage_dir() / f"{media_id}.m4a.tmp"
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
            raise UploadError(_ffmpeg_error("transcoding failed", proc))
        try:
            self._s3.put_file(self.audio_key(media_id), tmp)
        finally:
            tmp.unlink(missing_ok=True)

    def extract_cover(self, media_id: str, src: Path) -> bool:
        """Best-effort: pull an embedded cover into ``uploads/<media_id>.jpg``."""
        tmp = self._stage_dir() / f"{media_id}.jpg.tmp"
        proc = subprocess.run(
            [
                "ffmpeg", "-nostdin", "-y", "-i", str(src),
                "-an", "-map", "0:v?", "-c:v", "mjpeg", "-frames:v", "1",
                "-f", "mjpeg", str(tmp),
            ],
            capture_output=True, text=True,
        )
        if proc.returncode != 0 or not tmp.is_file() or tmp.stat().st_size == 0:
            tmp.unlink(missing_ok=True)
            return False
        try:
            self._s3.put_file(self.cover_key(media_id), tmp)
        finally:
            tmp.unlink(missing_ok=True)
        return True

    def delete(self, media_id: str) -> None:
        self._s3.delete(self.audio_key(media_id))
        self._s3.delete(self.cover_key(media_id))

    def total_size(self) -> int:
        try:
            return sum(obj.size for obj in self._s3.list(UPLOADS_PREFIX))
        except Exception:  # noqa: BLE001 - quota check must not break uploads
            log.exception("uploads total_size: could not list objects")
            return 0


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


class UploadJobs:
    """In-memory registry of running transcode jobs.

    The registry is lost on restart, so :meth:`status` falls back to the
    object store when a job is unknown: audio present -> ``ready``, staging
    ``.part`` present -> ``failed`` (an orphaned job). The authoritative state
    lives in Postgres on the Go side.
    """

    def __init__(self, storage: UploadStorage, settings: Settings) -> None:
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
                if not self.storage.cover_exists(media_id):
                    await asyncio.to_thread(self.storage.extract_cover, media_id, src)
                has_cover = self.storage.cover_exists(media_id)
            except UploadError as exc:
                self._jobs[media_id] = JobState(status="failed", error=str(exc))
                log.warning("upload %s failed: %s", media_id, exc)
                return
            except Exception:  # noqa: BLE001 - never let a job kill the loop
                self._jobs[media_id] = JobState(status="failed", error="internal error")
                log.exception("upload %s crashed", media_id)
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
        if self.storage.exists(media_id):
            return JobState(
                status="ready",
                has_cover=self.storage.cover_exists(media_id),
            )
        if self.storage.stage_path(media_id).is_file():
            return JobState(status="failed", error="orphaned job")
        return None

    def discard(self, media_id: str) -> None:
        self._jobs.pop(media_id, None)




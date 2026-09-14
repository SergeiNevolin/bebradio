"""Cache of YouTube audio + subtitles in MinIO (S3).

Local disk is only scratch space: yt-dlp downloads into ``work_dir`` and the
finished files are uploaded to ``tracks/`` in the bucket. TTL/size cleanup
walks S3 objects instead of the filesystem.
"""

import asyncio
import logging
import re
import time
from pathlib import Path
from typing import Callable, Iterator, Optional

from config import Settings
from s3store import S3Object, S3Store

log = logging.getLogger(__name__)
_MEDIA_ID_RE = re.compile(r"[A-Za-z0-9_-]+")
TRACKS_PREFIX = "tracks/"
# Audio containers yt-dlp may produce for `bestaudio`. Used to tell the track's
# audio file apart from sibling artefacts sharing its media_id prefix (e.g. the
# `media_<hash>.<lang>.vtt` subtitle file written alongside it).
_AUDIO_EXTS = frozenset({".flac", ".m4a", ".mp3", ".ogg", ".opus", ".wav", ".webm"})


class MediaStorage:
    def __init__(
        self,
        settings: Settings,
        downloader: Callable[[str, Path], bool],
        s3: Optional[S3Store] = None,
    ) -> None:
        self.settings = settings
        self._downloader = downloader
        self._s3 = s3 if s3 is not None else S3Store.from_settings(settings)
        self._locks: dict[str, asyncio.Lock] = {}
        self._locks_guard = asyncio.Lock()
        self._slots = asyncio.Semaphore(settings.max_downloads)
        self._referenced: set[str] = set()

    def init(self) -> None:
        (self.settings.work_dir / "tracks").mkdir(parents=True, exist_ok=True)
        self._s3.ensure_bucket()

    @staticmethod
    def valid_id(media_id: str) -> bool:
        return bool(media_id and _MEDIA_ID_RE.fullmatch(media_id))

    @staticmethod
    def _track_key(name: str) -> str:
        """`media_<hash>` for both `media_<hash>.m4a` and `media_<hash>.en.vtt`."""
        return name.split(".", 1)[0]

    def _objects(self, media_id: str) -> list[S3Object]:
        return self._s3.list(f"{TRACKS_PREFIX}{media_id}.")

    def _audio_object(self, media_id: str) -> Optional[S3Object]:
        candidates = [
            obj
            for obj in self._objects(media_id)
            if Path(obj.key).suffix.lower() in _AUDIO_EXTS
        ]
        return sorted(candidates, key=lambda obj: obj.key)[0] if candidates else None

    def audio_filename(self, media_id: str) -> str:
        obj = self._audio_object(media_id)
        return Path(obj.key).name if obj else f"{media_id}.m4a"

    def is_ready(self, media_id: str) -> bool:
        return self._audio_object(media_id) is not None

    def audio_size(self, media_id: str) -> Optional[int]:
        obj = self._audio_object(media_id)
        return obj.size if obj else None

    def audio_stream(
        self, media_id: str, start: int | None = None, end: int | None = None
    ) -> tuple[Iterator[bytes], int] | None:
        obj = self._audio_object(media_id)
        if obj is None:
            return None
        return self._s3.stream(obj.key, start, end)

    def captions_data(self, media_id: str, lang: str = "") -> tuple[str, str] | None:
        """Best subtitle as ``(object_name, vtt_text)`` or None.

        Prefers manual captions over auto-generated (`.auto.` in the name),
        then the requested language, then ru, then en, then whatever exists.
        """
        if not self.valid_id(media_id):
            return None
        vtts = [
            obj
            for obj in self._objects(media_id)
            if obj.key.lower().endswith(".vtt")
        ]
        if not vtts:
            return None

        def rank(obj: S3Object) -> tuple[bool, int]:
            name = obj.key.lower()
            if lang and f".{lang.lower()}." in name:
                lang_score = 0
            elif ".ru" in name:
                lang_score = 1
            elif ".en" in name:
                lang_score = 2
            else:
                lang_score = 3
            return (".auto." in name, lang_score)

        best = min(vtts, key=rank)
        text = self._s3.get_bytes(best.key)
        if text is None:
            return None
        return Path(best.key).name, text.decode("utf-8", errors="replace")

    async def ensure(self, source_url: str, media_id: str) -> bool:
        if not self.valid_id(media_id):
            return False
        async with self._locks_guard:
            lock = self._locks.setdefault(media_id, asyncio.Lock())
        async with lock, self._slots:
            if self.is_ready(media_id):
                return True
            return await asyncio.to_thread(self._download, source_url, media_id)

    def _download(self, source_url: str, media_id: str) -> bool:
        work = self.settings.work_dir / "tracks"
        work.mkdir(parents=True, exist_ok=True)
        # yt-dlp appends the real container extension to this stem, and drops
        # subtitles next to it; both are picked up below and uploaded to S3.
        target = work / f"{media_id}.m4a"
        if not self._downloader(source_url, target):
            self._drop_work_files(media_id)
            return False
        try:
            for path in sorted(work.glob(f"{media_id}.*")):
                if not path.is_file():
                    continue
                if path.suffix.lower() in _AUDIO_EXTS or path.suffix.lower() == ".vtt":
                    self._s3.put_file(f"{TRACKS_PREFIX}{path.name}", path)
        finally:
            self._drop_work_files(media_id)
        return self.is_ready(media_id)

    def _drop_work_files(self, media_id: str) -> None:
        work = self.settings.work_dir / "tracks"
        for path in work.glob(f"{media_id}.*"):
            try:
                path.unlink()
            except OSError:
                pass

    def set_references(self, media_ids: list[str]) -> None:
        self._referenced = {item for item in media_ids if self.valid_id(item)}

    async def cleanup(self) -> None:
        await asyncio.to_thread(self._cleanup_sync)

    def _cleanup_sync(self) -> None:
        now = time.time()
        try:
            objects = self._s3.list(TRACKS_PREFIX)
        except Exception:  # noqa: BLE001 - cleanup must never crash the loop
            log.exception("media cleanup: could not list tracks")
            return
        for obj in objects:
            name = Path(obj.key).name
            if self._track_key(name) in self._referenced:
                continue
            try:
                if now - obj.last_modified > self.settings.media_ttl:
                    self._s3.delete(obj.key)
            except Exception:  # noqa: BLE001 - best effort per object
                log.warning("could not remove expired media file %s", obj.key)
        self._enforce_size_limit()

    def _enforce_size_limit(self) -> None:
        try:
            objects = self._s3.list(TRACKS_PREFIX)
        except Exception:  # noqa: BLE001 - cleanup must never crash the loop
            log.exception("media cleanup: could not list tracks for eviction")
            return
        total = sum(obj.size for obj in objects)
        if total <= self.settings.media_max_size:
            return

        candidates = [
            obj for obj in objects if self._track_key(Path(obj.key).name) not in self._referenced
        ]
        for obj in sorted(candidates, key=lambda item: item.last_modified):
            if total <= self.settings.media_max_size:
                break
            try:
                self._s3.delete(obj.key)
                total -= obj.size
            except Exception:  # noqa: BLE001 - best effort per object
                log.warning("could not evict media file %s", obj.key)

    async def cleanup_loop(self) -> None:
        while True:
            await asyncio.sleep(self.settings.cleanup_interval)
            await self.cleanup()

    @property
    def referenced(self) -> set[str]:
        return self._referenced

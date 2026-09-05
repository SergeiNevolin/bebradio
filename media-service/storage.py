import asyncio
import logging
import re
import time
from pathlib import Path
from typing import Callable

from config import Settings

log = logging.getLogger(__name__)
_MEDIA_ID_RE = re.compile(r"[A-Za-z0-9_-]+")


class MediaStorage:
    def __init__(self, settings: Settings, downloader: Callable[[str, Path], bool]) -> None:
        self.settings = settings
        self._downloader = downloader
        self._locks: dict[str, asyncio.Lock] = {}
        self._locks_guard = asyncio.Lock()
        self._slots = asyncio.Semaphore(settings.max_downloads)
        self._referenced: set[str] = set()

    def init(self) -> None:
        self.settings.media_dir.mkdir(parents=True, exist_ok=True)

    @staticmethod
    def valid_id(media_id: str) -> bool:
        return bool(media_id and _MEDIA_ID_RE.fullmatch(media_id))

    def path(self, media_id: str) -> Path:
        matches = list(self.settings.media_dir.glob(f"{media_id}.*"))
        return matches[0] if matches else self.settings.media_dir / f"{media_id}.m4a"

    def is_ready(self, media_id: str) -> bool:
        return self.path(media_id).is_file()

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
        self.init()
        return self._downloader(source_url, self.path(media_id)) and self.is_ready(media_id)

    def set_references(self, media_ids: list[str]) -> None:
        self._referenced = {item for item in media_ids if self.valid_id(item)}

    async def cleanup(self) -> None:
        now = time.time()
        for path in self.settings.media_dir.iterdir():
            if not path.is_file() or path.suffix == ".part" or path.stem in self._referenced:
                continue
            try:
                if now - path.stat().st_mtime > self.settings.media_ttl:
                    path.unlink()
            except OSError:
                log.warning("could not remove expired media file %s", path)
        await asyncio.to_thread(self._enforce_size_limit)

    def _enforce_size_limit(self) -> None:
        files = [path for path in self.settings.media_dir.iterdir() if path.is_file() and path.suffix != ".part"]
        total = sum(path.stat().st_size for path in files)
        if total <= self.settings.media_max_size:
            return

        candidates = []
        for path in files:
            if path.stem in self._referenced:
                continue
            try:
                stat = path.stat()
            except OSError:
                continue
            candidates.append((path, stat.st_size, stat.st_mtime))

        for path, size, _ in sorted(candidates, key=lambda item: item[2]):
            if total <= self.settings.media_max_size:
                break
            try:
                path.unlink()
                total -= size
            except OSError:
                log.warning("could not evict media file %s", path)

    async def cleanup_loop(self) -> None:
        while True:
            await asyncio.sleep(self.settings.cleanup_interval)
            await self.cleanup()

    @property
    def referenced(self) -> set[str]:
        return self._referenced

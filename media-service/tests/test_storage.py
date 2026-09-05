import asyncio
import os
import time
from dataclasses import replace

import pytest


@pytest.mark.asyncio
async def test_ensure_downloads_once_for_same_media_id(settings):
    from storage import MediaStorage

    calls = []

    def downloader(source_url, output_path):
        calls.append((source_url, output_path))
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_bytes(b"audio")
        return True

    storage = MediaStorage(settings, downloader)

    results = await asyncio.gather(
        storage.ensure("source", "media_same"),
        storage.ensure("source", "media_same"),
    )

    assert results == [True, True]
    assert len(calls) == 1
    assert storage.path("media_same").read_bytes() == b"audio"


@pytest.mark.asyncio
async def test_ensure_rejects_path_traversal(settings):
    from storage import MediaStorage

    storage = MediaStorage(settings, lambda *_: True)

    assert await storage.ensure("source", "../escape") is False
    assert await storage.ensure("source", "media/id") is False


@pytest.mark.asyncio
async def test_cleanup_keeps_referenced_and_removes_expired(settings):
    from storage import MediaStorage

    settings.media_dir.mkdir(parents=True)
    stale = settings.media_dir / "media_stale.m4a"
    referenced = settings.media_dir / "media_ref.m4a"
    fresh = settings.media_dir / "media_fresh.m4a"
    for path in (stale, referenced, fresh):
        path.write_bytes(b"x")
    old_time = time.time() - settings.media_ttl - 1
    os.utime(stale, (old_time, old_time))
    os.utime(referenced, (old_time, old_time))

    storage = MediaStorage(settings, lambda *_: True)
    storage.set_references(["media_ref"])
    await storage.cleanup()

    assert not stale.exists()
    assert referenced.exists()
    assert fresh.exists()


@pytest.mark.asyncio
async def test_cleanup_evicts_oldest_unreferenced_when_size_exceeded(settings):
    from storage import MediaStorage

    settings = replace(settings, media_max_size=5)
    settings.media_dir.mkdir(parents=True)
    oldest = settings.media_dir / "media_old.m4a"
    newest = settings.media_dir / "media_new.m4a"
    oldest.write_bytes(b"1234")
    newest.write_bytes(b"5678")
    old_time = time.time() - 10
    os.utime(oldest, (old_time, old_time))

    storage = MediaStorage(settings, lambda *_: True)
    await storage.cleanup()

    assert not oldest.exists()
    assert newest.exists()

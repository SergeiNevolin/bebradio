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


def test_path_prefers_audio_over_sibling_vtt(settings):
    from storage import MediaStorage

    settings.media_dir.mkdir(parents=True)
    (settings.media_dir / "media_x.en.vtt").write_text("WEBVTT")
    (settings.media_dir / "media_x.m4a").write_bytes(b"audio")

    storage = MediaStorage(settings, lambda *_: True)

    assert storage.path("media_x").name == "media_x.m4a"
    assert storage.is_ready("media_x") is True


def test_is_ready_false_when_only_vtt_on_disk(settings):
    from storage import MediaStorage

    settings.media_dir.mkdir(parents=True)
    (settings.media_dir / "media_x.en.vtt").write_text("WEBVTT")

    storage = MediaStorage(settings, lambda *_: True)

    assert storage.is_ready("media_x") is False


def test_captions_path_language_priority(settings):
    from storage import MediaStorage

    settings.media_dir.mkdir(parents=True)
    (settings.media_dir / "media_x.en.vtt").write_text("WEBVTT")
    (settings.media_dir / "media_x.ru.vtt").write_text("WEBVTT")

    storage = MediaStorage(settings, lambda *_: True)

    assert storage.captions_path("media_x", "en").name == "media_x.en.vtt"
    assert storage.captions_path("media_x", "ru").name == "media_x.ru.vtt"
    # No language requested: ru outranks en.
    assert storage.captions_path("media_x").name == "media_x.ru.vtt"
    assert storage.captions_path("media_missing") is None
    assert storage.captions_path("../escape") is None


def test_captions_path_prefers_manual_over_auto(settings):
    from storage import MediaStorage

    settings.media_dir.mkdir(parents=True)
    # Manual only for en; auto only for ru.
    (settings.media_dir / "media_x.en.vtt").write_text("WEBVTT")
    (settings.media_dir / "media_x.auto.ru.vtt").write_text("WEBVTT")

    storage = MediaStorage(settings, lambda *_: True)

    # Manual beats auto even though ru would otherwise outrank en.
    assert storage.captions_path("media_x").name == "media_x.en.vtt"
    # Falls back to auto when that is all there is.
    (settings.media_dir / "media_x.en.vtt").unlink()
    assert storage.captions_path("media_x").name == "media_x.auto.ru.vtt"


@pytest.mark.asyncio
async def test_cleanup_keeps_vtt_while_audio_referenced(settings):
    from storage import MediaStorage

    settings.media_dir.mkdir(parents=True)
    audio = settings.media_dir / "media_ref.m4a"
    subs = settings.media_dir / "media_ref.en.vtt"
    audio.write_bytes(b"x")
    subs.write_text("WEBVTT")
    old_time = time.time() - settings.media_ttl - 1
    os.utime(audio, (old_time, old_time))
    os.utime(subs, (old_time, old_time))

    storage = MediaStorage(settings, lambda *_: True)
    storage.set_references(["media_ref"])
    await storage.cleanup()

    assert audio.exists()
    assert subs.exists()


@pytest.mark.asyncio
async def test_cleanup_removes_expired_vtt_with_its_audio(settings):
    from storage import MediaStorage

    settings.media_dir.mkdir(parents=True)
    audio = settings.media_dir / "media_stale.m4a"
    subs = settings.media_dir / "media_stale.en.vtt"
    audio.write_bytes(b"x")
    subs.write_text("WEBVTT")
    old_time = time.time() - settings.media_ttl - 1
    os.utime(audio, (old_time, old_time))
    os.utime(subs, (old_time, old_time))

    storage = MediaStorage(settings, lambda *_: True)
    await storage.cleanup()

    assert not audio.exists()
    assert not subs.exists()

import asyncio
from dataclasses import replace

import pytest


def _make_storage(settings, fake_s3, downloader=None):
    from storage import MediaStorage

    return MediaStorage(settings, downloader or (lambda *_: True), fake_s3)


@pytest.mark.asyncio
async def test_ensure_downloads_once_for_same_media_id(settings, fake_s3):
    calls = []

    def downloader(source_url, output_path):
        calls.append((source_url, output_path))
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_bytes(b"audio")
        return True

    storage = _make_storage(settings, fake_s3, downloader)

    results = await asyncio.gather(
        storage.ensure("source", "media_same"),
        storage.ensure("source", "media_same"),
    )

    assert results == [True, True]
    assert len(calls) == 1
    assert fake_s3.get_bytes("tracks/media_same.m4a") == b"audio"


@pytest.mark.asyncio
async def test_ensure_rejects_path_traversal(settings, fake_s3):
    storage = _make_storage(settings, fake_s3)

    assert await storage.ensure("source", "../escape") is False
    assert await storage.ensure("source", "media/id") is False


@pytest.mark.asyncio
async def test_cleanup_keeps_referenced_and_removes_expired(settings, fake_s3):
    storage = _make_storage(settings, fake_s3)
    for name in ("media_stale.m4a", "media_ref.m4a", "media_fresh.m4a"):
        fake_s3.put_bytes(f"tracks/{name}", b"x")
    fake_s3.backdate("tracks/media_stale.m4a", settings.media_ttl + 1)
    fake_s3.backdate("tracks/media_ref.m4a", settings.media_ttl + 1)

    storage.set_references(["media_ref"])
    await storage.cleanup()

    assert not fake_s3.exists("tracks/media_stale.m4a")
    assert fake_s3.exists("tracks/media_ref.m4a")
    assert fake_s3.exists("tracks/media_fresh.m4a")


@pytest.mark.asyncio
async def test_cleanup_evicts_oldest_unreferenced_when_size_exceeded(settings, fake_s3):
    settings = replace(settings, media_max_size=5)
    storage = _make_storage(settings, fake_s3)
    fake_s3.put_bytes("tracks/media_old.m4a", b"1234")
    fake_s3.backdate("tracks/media_old.m4a", 10)
    fake_s3.put_bytes("tracks/media_new.m4a", b"5678")

    await storage.cleanup()

    assert not fake_s3.exists("tracks/media_old.m4a")
    assert fake_s3.exists("tracks/media_new.m4a")


def test_audio_prefers_audio_over_sibling_vtt(settings, fake_s3):
    fake_s3.put_bytes("tracks/media_x.en.vtt", b"WEBVTT")
    fake_s3.put_bytes("tracks/media_x.m4a", b"audio")

    storage = _make_storage(settings, fake_s3)

    assert storage.audio_filename("media_x") == "media_x.m4a"
    assert storage.is_ready("media_x") is True


def test_is_ready_false_when_only_vtt_in_store(settings, fake_s3):
    fake_s3.put_bytes("tracks/media_x.en.vtt", b"WEBVTT")

    storage = _make_storage(settings, fake_s3)

    assert storage.is_ready("media_x") is False


def test_captions_data_language_priority(settings, fake_s3):
    fake_s3.put_bytes("tracks/media_x.en.vtt", b"WEBVTT")
    fake_s3.put_bytes("tracks/media_x.ru.vtt", b"WEBVTT")

    storage = _make_storage(settings, fake_s3)

    assert storage.captions_data("media_x", "en")[0] == "media_x.en.vtt"
    assert storage.captions_data("media_x", "ru")[0] == "media_x.ru.vtt"
    # No language requested: ru outranks en.
    assert storage.captions_data("media_x")[0] == "media_x.ru.vtt"
    assert storage.captions_data("media_missing") is None
    assert storage.captions_data("../escape") is None


def test_captions_data_prefers_manual_over_auto(settings, fake_s3):
    fake_s3.put_bytes("tracks/media_x.en.vtt", b"WEBVTT")
    fake_s3.put_bytes("tracks/media_x.auto.ru.vtt", b"WEBVTT")

    storage = _make_storage(settings, fake_s3)

    # Manual beats auto even though ru would otherwise outrank en.
    assert storage.captions_data("media_x")[0] == "media_x.en.vtt"
    # Falls back to auto when that is all there is.
    fake_s3.delete("tracks/media_x.en.vtt")
    assert storage.captions_data("media_x")[0] == "media_x.auto.ru.vtt"


@pytest.mark.asyncio
async def test_cleanup_keeps_vtt_while_audio_referenced(settings, fake_s3):
    fake_s3.put_bytes("tracks/media_ref.m4a", b"x")
    fake_s3.put_bytes("tracks/media_ref.en.vtt", b"WEBVTT")
    fake_s3.backdate("tracks/media_ref.m4a", settings.media_ttl + 1)
    fake_s3.backdate("tracks/media_ref.en.vtt", settings.media_ttl + 1)

    storage = _make_storage(settings, fake_s3)
    storage.set_references(["media_ref"])
    await storage.cleanup()

    assert fake_s3.exists("tracks/media_ref.m4a")
    assert fake_s3.exists("tracks/media_ref.en.vtt")


@pytest.mark.asyncio
async def test_cleanup_removes_expired_vtt_with_its_audio(settings, fake_s3):
    fake_s3.put_bytes("tracks/media_stale.m4a", b"x")
    fake_s3.put_bytes("tracks/media_stale.en.vtt", b"WEBVTT")
    fake_s3.backdate("tracks/media_stale.m4a", settings.media_ttl + 1)
    fake_s3.backdate("tracks/media_stale.en.vtt", settings.media_ttl + 1)

    storage = _make_storage(settings, fake_s3)
    await storage.cleanup()

    assert not fake_s3.exists("tracks/media_stale.m4a")
    assert not fake_s3.exists("tracks/media_stale.en.vtt")

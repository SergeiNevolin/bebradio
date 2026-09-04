import os
import time
import pytest
from pathlib import Path
import models
import store


def test_init_media_dir_creates_directory(tmp_path):
    import media
    test_dir = str(tmp_path / "test_media")
    media._media_dir = None
    old = media.MEDIA_DIR
    media.MEDIA_DIR = test_dir
    media.init_media_dir()
    assert Path(test_dir).exists()
    media.MEDIA_DIR = old


def test_get_track_path_returns_existing_file(tmp_path):
    import media
    media._media_dir = tmp_path
    (tmp_path / "abc123.m4a").touch()
    path = media.get_track_path("abc123")
    assert path.name == "abc123.m4a"
    assert path.exists()


def test_get_track_path_fallback_when_no_file(tmp_path):
    import media
    media._media_dir = tmp_path
    path = media.get_track_path("nonexistent")
    assert path.name == "nonexistent.m4a"


def test_get_track_path_prefers_actual_extension(tmp_path):
    import media
    media._media_dir = tmp_path
    (tmp_path / "abc123.mp3").touch()
    path = media.get_track_path("abc123")
    assert path.name == "abc123.mp3"


def test_is_downloaded_true(tmp_path):
    import media
    media._media_dir = tmp_path
    (tmp_path / "xyz.m4a").touch()
    assert media.is_downloaded("xyz") is True


def test_is_downloaded_false(tmp_path):
    import media
    media._media_dir = tmp_path
    assert media.is_downloaded("nope") is False


def test_get_local_filename_returns_name(tmp_path):
    import media
    media._media_dir = tmp_path
    (tmp_path / "abc123.m4a").touch()
    assert media.get_local_filename("abc123") == "abc123.m4a"


def test_get_local_filename_none_when_missing(tmp_path):
    import media
    media._media_dir = tmp_path
    assert media.get_local_filename("missing") is None


def test_delete_track_file_removes_file(tmp_path):
    import media
    media._media_dir = tmp_path
    (tmp_path / "del.m4a").touch()
    assert media.delete_track_file("del") is True
    assert not (tmp_path / "del.m4a").exists()


def test_delete_track_file_returns_false_when_missing(tmp_path):
    import media
    media._media_dir = tmp_path
    assert media.delete_track_file("nope") is False


def test_download_track_skips_if_already_downloaded(tmp_path, monkeypatch):
    import media
    import youtube
    media._media_dir = tmp_path
    (tmp_path / "abc.m4a").touch()
    called = False

    def _boom(*a, **k):
        nonlocal called
        called = True
        return None

    monkeypatch.setattr(youtube, "_run_ytdlp", _boom)
    assert media.download_track("https://youtu.be/x", "abc") is True
    assert called is False


@pytest.mark.asyncio
async def test_cleanup_expired_media_deletes_old_unreferenced(tmp_path):
    import media
    media._media_dir = tmp_path
    stale = tmp_path / "stale.m4a"
    stale.touch()
    os.utime(stale, (time.time() - 86400 * 10,) * 2)
    recent = tmp_path / "fresh.m4a"
    recent.touch()
    ref = tmp_path / "ref.m4a"
    ref.touch()
    os.utime(ref, (time.time() - 86400 * 10,) * 2)
    r = models.Room()
    r.queue = [models.Track(id="ref")]
    store.rooms["TEST"] = r
    deleted = await media.cleanup_expired_media()
    assert deleted == 1
    assert not stale.exists()
    assert recent.exists()
    assert ref.exists()
    store.rooms.pop("TEST", None)


@pytest.mark.asyncio
async def test_cleanup_expired_media_keeps_referenced_files(tmp_path):
    import media
    media._media_dir = tmp_path
    f = tmp_path / "track1.m4a"
    f.touch()
    os.utime(f, (time.time() - 86400 * 10,) * 2)
    r = models.Room()
    r.queue = [models.Track(id="track1")]
    store.rooms["T2"] = r
    deleted = await media.cleanup_expired_media()
    assert deleted == 0
    assert f.exists()
    store.rooms.pop("T2", None)


@pytest.mark.asyncio
async def test_cleanup_enforces_size_limit(tmp_path, monkeypatch):
    import media
    media._media_dir = tmp_path
    monkeypatch.setattr(media, "MEDIA_MAX_SIZE", 1000)

    # Create 3 unreferenced files of ~400 bytes each = 1200 total (over 1000 limit)
    for name in ("old1.m4a", "old2.m4a", "old3.m4a"):
        f = tmp_path / name
        f.write_bytes(b"x" * 400)
        os.utime(f, (time.time() - 3600,) * 2)  # 1 hour old

    deleted = await media.cleanup_expired_media()
    # At least 1 file should be deleted to bring total under 1000
    assert deleted >= 1
    total = sum(p.stat().st_size for p in tmp_path.iterdir() if p.is_file())
    assert total <= 1000


@pytest.mark.asyncio
async def test_cleanup_does_not_delete_referenced_when_over_limit(tmp_path, monkeypatch):
    import media
    media._media_dir = tmp_path
    monkeypatch.setattr(media, "MEDIA_MAX_SIZE", 500)

    # Create a referenced file (large) and an unreferenced file (small)
    ref = tmp_path / "ref.m4a"
    ref.write_bytes(b"r" * 400)
    os.utime(ref, (time.time() - 3600,) * 2)
    unref = tmp_path / "unref.m4a"
    unref.write_bytes(b"u" * 400)
    os.utime(unref, (time.time() - 3600,) * 2)

    r = models.Room()
    r.queue = [models.Track(id="ref")]
    store.rooms["SZ"] = r

    deleted = await media.cleanup_expired_media()
    assert deleted >= 1
    assert ref.exists()  # referenced file kept
    assert not unref.exists()  # unreferenced deleted
    store.rooms.pop("SZ", None)

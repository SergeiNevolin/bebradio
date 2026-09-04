import pytest
import models


@pytest.mark.asyncio
async def test_ensure_local_downloads_track(monkeypatch):
    import streams
    monkeypatch.setattr(streams.media, "download_track", lambda src, tid: True)
    monkeypatch.setattr(streams.media, "is_downloaded", lambda tid: True)
    monkeypatch.setattr(streams.media, "get_local_filename", lambda tid: f"{tid}.m4a")
    r = models.Room()
    t = models.Track(source_url="https://youtu.be/abcdefghijk", local_path="")
    r.queue = [t]
    changed = await streams.ensure_local(r, t)
    assert changed is True
    assert t.local_path == "abcdefghijk.m4a"


@pytest.mark.asyncio
async def test_ensure_local_skips_already_downloaded(monkeypatch):
    import streams
    called = False

    def _boom(src, tid):
        nonlocal called
        called = True
        return True

    monkeypatch.setattr(streams.media, "download_track", _boom)
    monkeypatch.setattr(streams.media, "is_downloaded", lambda tid: True)
    t = models.Track(source_url="https://youtu.be/abcdefghijk", local_path="abcdefghijk.m4a", url="/api/media/abcdefghijk")
    assert await streams.ensure_local(models.Room(), t) is False
    assert called is False


@pytest.mark.asyncio
async def test_ensure_local_ignores_track_without_source_url():
    import streams
    t = models.Track(source_url="", local_path="")
    assert await streams.ensure_local(models.Room(), t) is False


@pytest.mark.asyncio
async def test_ensure_local_ahead_downloads_current_and_next(monkeypatch):
    import streams
    downloaded = []
    monkeypatch.setattr(
        streams.media, "download_track",
        lambda src, tid: (downloaded.append(tid), True)[-1],
    )
    monkeypatch.setattr(streams.media, "is_downloaded", lambda tid: False)
    monkeypatch.setattr(streams.media, "get_local_filename", lambda tid: f"{tid}.m4a")
    r = models.Room()
    r.queue = [
        models.Track(source_url="https://youtu.be/aaaaaaaaaaa", local_path=""),
        models.Track(source_url="https://youtu.be/bbbbbbbbbbb", local_path=""),
        models.Track(source_url="https://youtu.be/ccccccccccc", local_path=""),
    ]
    r.current_index = 0
    changed = await streams.ensure_local_ahead(r)
    assert changed is True
    assert "aaaaaaaaaaa" in downloaded
    assert "bbbbbbbbbbb" in downloaded
    assert "ccccccccccc" not in downloaded


@pytest.mark.asyncio
async def test_ensure_local_ahead_without_next_track(monkeypatch):
    import streams
    downloaded = []
    monkeypatch.setattr(
        streams.media, "download_track",
        lambda src, tid: (downloaded.append(tid), True)[-1],
    )
    monkeypatch.setattr(streams.media, "is_downloaded", lambda tid: False)
    monkeypatch.setattr(streams.media, "get_local_filename", lambda tid: f"{tid}.m4a")
    r = models.Room()
    r.queue = [models.Track(source_url="https://youtu.be/aaaaaaaaaaa", local_path="")]
    r.current_index = 0
    assert await streams.ensure_local_ahead(r) is True
    assert downloaded == ["aaaaaaaaaaa"]


@pytest.mark.asyncio
async def test_ensure_local_falls_back_to_source_url_for_old_tracks(monkeypatch):
    """Tracks loaded from old DB rows without video_id still work."""
    import streams
    monkeypatch.setattr(streams.media, "download_track", lambda src, tid: True)
    monkeypatch.setattr(streams.media, "is_downloaded", lambda tid: True)
    monkeypatch.setattr(streams.media, "get_local_filename", lambda tid: f"{tid}.m4a")
    t = models.Track(source_url="https://youtu.be/xyz99999999", local_path="")
    assert t.video_id == ""
    changed = await streams.ensure_local(models.Room(), t)
    assert changed is True
    assert t.video_id == "xyz99999999"
    assert t.local_path == "xyz99999999.m4a"

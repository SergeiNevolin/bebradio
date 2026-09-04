import pytest
import models


@pytest.mark.asyncio
async def test_ensure_local_downloads_track(monkeypatch):
    import streams
    monkeypatch.setattr(streams.media, "download_track", lambda src, tid: True)
    monkeypatch.setattr(streams.media, "is_downloaded", lambda tid: True)
    monkeypatch.setattr(streams.media, "get_local_filename", lambda tid: f"{tid}.m4a")
    r = models.Room()
    t = models.Track(id="a", source_url="https://youtu.be/abc", local_path="")
    r.queue = [t]
    changed = await streams.ensure_local(r, t)
    assert changed is True
    assert t.local_path == "a.m4a"


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
    t = models.Track(id="a", source_url="https://youtu.be/abc", local_path="a.m4a", url="/api/media/a")
    assert await streams.ensure_local(models.Room(), t) is False
    assert called is False


@pytest.mark.asyncio
async def test_ensure_local_ignores_track_without_source_url():
    import streams
    t = models.Track(id="a", source_url="", local_path="")
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
        models.Track(id="a", source_url="https://youtu.be/aaa", local_path=""),
        models.Track(id="b", source_url="https://youtu.be/bbb", local_path=""),
        models.Track(id="c", source_url="https://youtu.be/ccc", local_path=""),
    ]
    r.current_index = 0
    changed = await streams.ensure_local_ahead(r)
    assert changed is True
    assert "a" in downloaded
    assert "b" in downloaded
    assert "c" not in downloaded


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
    r.queue = [models.Track(id="a", source_url="https://youtu.be/aaa", local_path="")]
    r.current_index = 0
    assert await streams.ensure_local_ahead(r) is True
    assert downloaded == ["a"]

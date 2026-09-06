import json
import time

import pytest
from httpx import ASGITransport, AsyncClient


class _FakeUpload:
    """Minimal stand-in for Starlette's UploadFile (async ``read(n)``)."""

    def __init__(self, data: bytes, chunk: int = 512) -> None:
        self._data = data
        self._chunk = chunk
        self._pos = 0

    async def read(self, size: int = -1) -> bytes:
        step = self._chunk if size in (-1, None) else min(size, self._chunk)
        piece = self._data[self._pos : self._pos + step]
        self._pos += len(piece)
        return piece


def _probe_json(*, audio=True, duration=42.0, tags=None) -> str:
    streams = [{"codec_type": "audio"}] if audio else [{"codec_type": "video"}]
    fmt = {"duration": str(duration)}
    if tags:
        fmt["tags"] = tags
    return json.dumps({"streams": streams, "format": fmt})


class _Completed:
    def __init__(self, returncode=0, stdout="", stderr=""):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr


def test_valid_id_rejects_traversal():
    from mashups import MashupStorage

    assert MashupStorage.valid_id("abc-123_DEF")
    assert not MashupStorage.valid_id("../escape")
    assert not MashupStorage.valid_id("a/b")
    assert not MashupStorage.valid_id("")


@pytest.mark.asyncio
async def test_save_upload_aborts_on_oversize(settings):
    from mashups import MashupError, MashupStorage

    storage = MashupStorage(settings)
    upload = _FakeUpload(b"x" * (settings.mashup_max_size + 10))

    with pytest.raises(MashupError):
        await storage.save_upload("big", upload)

    assert not storage.part_path("big").exists()


@pytest.mark.asyncio
async def test_save_upload_writes_part_file(settings):
    from mashups import MashupStorage

    storage = MashupStorage(settings)
    part = await storage.save_upload("ok", _FakeUpload(b"hello world"))

    assert part == storage.part_path("ok")
    assert part.read_bytes() == b"hello world"


def test_probe_rejects_file_without_audio_stream(settings, monkeypatch):
    from mashups import MashupError, MashupStorage

    monkeypatch.setattr(
        "mashups.subprocess.run",
        lambda *a, **k: _Completed(stdout=_probe_json(audio=False)),
    )
    storage = MashupStorage(settings)

    with pytest.raises(MashupError):
        storage.probe(settings.mashup_dir / "x")


def test_probe_rejects_overlong_track(settings, monkeypatch):
    from mashups import MashupError, MashupStorage

    monkeypatch.setattr(
        "mashups.subprocess.run",
        lambda *a, **k: _Completed(stdout=_probe_json(duration=settings.mashup_max_duration + 1)),
    )
    storage = MashupStorage(settings)

    with pytest.raises(MashupError):
        storage.probe(settings.mashup_dir / "x")


def test_probe_reads_tags(settings, monkeypatch):
    from mashups import MashupStorage

    monkeypatch.setattr(
        "mashups.subprocess.run",
        lambda *a, **k: _Completed(
            stdout=_probe_json(duration=7.5, tags={"TITLE": "Bootie", "artist": "DJ"})
        ),
    )
    result = MashupStorage(settings).probe(settings.mashup_dir / "x")

    assert result.duration == 7.5
    assert result.title == "Bootie"
    assert result.artist == "DJ"


def test_transcode_raises_when_ffmpeg_fails(settings, monkeypatch):
    from mashups import MashupError, MashupStorage

    monkeypatch.setattr("mashups.subprocess.run", lambda *a, **k: _Completed(returncode=1, stderr="boom"))
    storage = MashupStorage(settings)
    storage.init()

    with pytest.raises(MashupError):
        storage.transcode("x", settings.mashup_dir / "src.part")
    assert not (settings.mashup_dir / "x.m4a.tmp").exists()


def test_delete_removes_all_artifacts(settings):
    from mashups import MashupStorage

    storage = MashupStorage(settings)
    storage.init()
    for path in (storage.path("m"), storage.cover_path("m"), storage.part_path("m")):
        path.write_bytes(b"x")

    storage.delete("m")

    assert not storage.path("m").exists()
    assert not storage.cover_path("m").exists()
    assert not storage.part_path("m").exists()


def test_status_is_derived_from_filesystem_when_job_unknown(settings):
    from mashups import MashupJobs, MashupStorage

    storage = MashupStorage(settings)
    storage.init()
    jobs = MashupJobs(storage, settings)

    assert jobs.status("missing") is None

    storage.part_path("orphan").write_bytes(b"x")
    assert jobs.status("orphan").status == "failed"

    storage.path("done").write_bytes(b"x")
    assert jobs.status("done").status == "ready"


@pytest.mark.asyncio
async def test_media_cleanup_leaves_mashup_dir_untouched(settings):
    """MediaStorage TTL cleanup must never walk into the sibling mashup dir."""
    from mashups import MashupStorage
    from storage import MediaStorage

    settings.media_dir.mkdir(parents=True)
    mashups = MashupStorage(settings)
    mashups.init()
    keeper = mashups.path("keeper")
    keeper.write_bytes(b"x")
    stale = time.time() - settings.media_ttl - 1
    import os

    os.utime(keeper, (stale, stale))

    media = MediaStorage(settings, lambda *_: True)
    await media.cleanup()

    assert keeper.exists()


@pytest.fixture
def mashup_app(settings):
    from api import create_app
    from service import MediaService

    service = MediaService(settings)
    service.start()
    return create_app(service), service


@pytest.mark.asyncio
async def test_upload_endpoint_rejects_invalid_id(mashup_app):
    app, _ = mashup_app

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.post("/v1/mashups/..%2Fescape", files={"file": ("a.mp3", b"x")})

    assert response.status_code in (400, 404)


@pytest.mark.asyncio
async def test_mashup_media_endpoint_supports_range(mashup_app):
    app, service = mashup_app
    service.mashups.path("song").write_bytes(b"0123456789")

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        ranged = await client.get("/v1/mashups/song", headers={"Range": "bytes=2-5"})
        missing = await client.get("/v1/mashups/nope")

    assert ranged.status_code == 206
    assert ranged.content == b"2345"
    assert ranged.headers["content-range"] == "bytes 2-5/10"
    assert missing.status_code == 404


@pytest.mark.asyncio
async def test_mashup_delete_endpoint_removes_file(mashup_app):
    app, service = mashup_app
    service.mashups.path("gone").write_bytes(b"x")

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.delete("/v1/mashups/gone")

    assert response.status_code == 200
    assert response.json() == {"ok": True}
    assert not service.mashups.path("gone").exists()

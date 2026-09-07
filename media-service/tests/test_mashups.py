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


@pytest.mark.asyncio
async def test_save_cover_upload_aborts_on_oversize(settings):
    from mashups import MashupError, MashupStorage

    storage = MashupStorage(settings)
    upload = _FakeUpload(b"x" * (settings.mashup_cover_max_size + 10))

    with pytest.raises(MashupError):
        await storage.save_cover_upload("big", upload)

    assert not storage.cover_part_path("big").exists()


@pytest.mark.asyncio
async def test_save_cover_upload_rejects_empty(settings):
    from mashups import MashupError, MashupStorage

    storage = MashupStorage(settings)
    with pytest.raises(MashupError):
        await storage.save_cover_upload("empty", _FakeUpload(b""))


def test_process_cover_raises_when_ffmpeg_fails(settings, monkeypatch):
    from mashups import MashupError, MashupStorage

    monkeypatch.setattr("mashups.subprocess.run", lambda *a, **k: _Completed(returncode=1, stderr="not an image"))
    storage = MashupStorage(settings)
    storage.init()
    src = settings.mashup_dir / "x.cover.part"
    src.write_bytes(b"junk")

    with pytest.raises(MashupError) as excinfo:
        storage.process_cover("x", src)
    assert "not an image" in str(excinfo.value)
    assert not storage.cover_path("x").exists()
    assert not src.exists()  # the part file is always cleaned up


def test_process_cover_writes_jpg(settings, monkeypatch):
    from mashups import MashupStorage

    def fake_run(cmd, *a, **k):
        open(cmd[-1], "wb").write(b"\xff\xd8jpegdata")
        return _Completed(returncode=0)

    monkeypatch.setattr("mashups.subprocess.run", fake_run)
    storage = MashupStorage(settings)
    storage.init()
    src = settings.mashup_dir / "x.cover.part"
    src.write_bytes(b"png-bytes")

    storage.process_cover("x", src)

    assert storage.cover_path("x").read_bytes() == b"\xff\xd8jpegdata"
    assert not src.exists()


@pytest.mark.asyncio
async def test_job_keeps_a_user_supplied_cover(settings, monkeypatch):
    """A cover uploaded while the transcode runs must not be replaced by
    whatever art is embedded in the source file."""
    from mashups import MashupJobs, MashupStorage, ProbeResult

    storage = MashupStorage(settings)
    storage.init()
    storage.cover_path("mid").write_bytes(b"user-cover")

    monkeypatch.setattr(storage, "probe", lambda src: ProbeResult(duration=5.0, title="", artist=""))
    monkeypatch.setattr(storage, "transcode", lambda mid, src: storage.path(mid).write_bytes(b"m4a"))
    extract_called = False

    def _extract(mid, src):
        nonlocal extract_called
        extract_called = True
        return True

    monkeypatch.setattr(storage, "extract_cover", _extract)

    jobs = MashupJobs(storage, settings)
    src = storage.part_path("mid")
    src.write_bytes(b"src")
    await jobs._run("mid", src)

    assert not extract_called
    assert storage.cover_path("mid").read_bytes() == b"user-cover"
    assert jobs.status("mid").has_cover is True


def test_transcode_raises_when_ffmpeg_fails(settings, monkeypatch):
    from mashups import MashupError, MashupStorage

    monkeypatch.setattr("mashups.subprocess.run", lambda *a, **k: _Completed(returncode=1, stderr="boom"))
    storage = MashupStorage(settings)
    storage.init()

    with pytest.raises(MashupError) as excinfo:
        storage.transcode("x", settings.mashup_dir / "src.part")
    assert "boom" in str(excinfo.value)  # ffmpeg stderr is surfaced
    assert not (settings.mashup_dir / "x.m4a.tmp").exists()


def test_transcode_forces_output_format(settings, monkeypatch):
    """The temp file ends in .tmp, so ffmpeg must be told the muxer explicitly."""
    from mashups import MashupStorage

    seen = {}

    def fake_run(cmd, *a, **k):
        seen["cmd"] = cmd
        out = cmd[-1]
        open(out, "wb").write(b"x" * 10)
        return _Completed(returncode=0)

    monkeypatch.setattr("mashups.subprocess.run", fake_run)
    storage = MashupStorage(settings)
    storage.init()
    storage.transcode("x", settings.mashup_dir / "src.part")

    assert "-f" in seen["cmd"]
    assert storage.path("x").exists()


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
async def test_upload_cover_endpoint(mashup_app, monkeypatch):
    app, service = mashup_app

    def fake_run(cmd, *a, **k):
        open(cmd[-1], "wb").write(b"\xff\xd8jpeg")
        return _Completed(returncode=0)

    monkeypatch.setattr("mashups.subprocess.run", fake_run)

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.put("/v1/mashups/song/cover", files={"file": ("art.png", b"png-bytes")})

    assert response.status_code == 200
    assert response.json() == {"ok": True, "has_cover": True}
    assert service.mashups.cover_path("song").is_file()
    assert not service.mashups.cover_part_path("song").exists()


@pytest.mark.asyncio
async def test_mashup_delete_endpoint_removes_file(mashup_app):
    app, service = mashup_app
    service.mashups.path("gone").write_bytes(b"x")

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.delete("/v1/mashups/gone")

    assert response.status_code == 200
    assert response.json() == {"ok": True}
    assert not service.mashups.path("gone").exists()

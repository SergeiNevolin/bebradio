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
    from uploads import UploadStorage

    assert UploadStorage.valid_id("abc-123_DEF")
    assert not UploadStorage.valid_id("../escape")
    assert not UploadStorage.valid_id("a/b")
    assert not UploadStorage.valid_id("")


@pytest.mark.asyncio
async def test_save_upload_aborts_on_oversize(settings, fake_s3):
    from uploads import UploadError, UploadStorage

    storage = UploadStorage(settings, fake_s3)
    upload = _FakeUpload(b"x" * (settings.mashup_max_size + 10))

    with pytest.raises(UploadError):
        await storage.save_upload("big", upload)

    assert not storage.stage_path("big").exists()


@pytest.mark.asyncio
async def test_save_upload_writes_part_file(settings, fake_s3):
    from uploads import UploadStorage

    storage = UploadStorage(settings, fake_s3)
    part = await storage.save_upload("ok", _FakeUpload(b"hello world"))

    assert part == storage.stage_path("ok")
    assert part.read_bytes() == b"hello world"


def test_probe_rejects_file_without_audio_stream(settings, fake_s3, monkeypatch):
    from uploads import UploadError, UploadStorage

    monkeypatch.setattr(
        "uploads.subprocess.run",
        lambda *a, **k: _Completed(stdout=_probe_json(audio=False)),
    )
    storage = UploadStorage(settings, fake_s3)

    with pytest.raises(UploadError):
        storage.probe(settings.work_dir / "x")


def test_probe_rejects_overlong_track(settings, fake_s3, monkeypatch):
    from uploads import UploadError, UploadStorage

    monkeypatch.setattr(
        "uploads.subprocess.run",
        lambda *a, **k: _Completed(stdout=_probe_json(duration=settings.mashup_max_duration + 1)),
    )
    storage = UploadStorage(settings, fake_s3)

    with pytest.raises(UploadError):
        storage.probe(settings.work_dir / "x")


def test_probe_reads_tags(settings, fake_s3, monkeypatch):
    from uploads import UploadStorage

    monkeypatch.setattr(
        "uploads.subprocess.run",
        lambda *a, **k: _Completed(
            stdout=_probe_json(duration=7.5, tags={"TITLE": "Bootie", "artist": "DJ"})
        ),
    )
    result = UploadStorage(settings, fake_s3).probe(settings.work_dir / "x")

    assert result.duration == 7.5
    assert result.title == "Bootie"
    assert result.artist == "DJ"


@pytest.mark.asyncio
async def test_save_cover_upload_aborts_on_oversize(settings, fake_s3):
    from uploads import UploadError, UploadStorage

    storage = UploadStorage(settings, fake_s3)
    upload = _FakeUpload(b"x" * (settings.mashup_cover_max_size + 10))

    with pytest.raises(UploadError):
        await storage.save_cover_upload("big", upload)

    assert not storage.cover_stage_path("big").exists()


@pytest.mark.asyncio
async def test_save_cover_upload_rejects_empty(settings, fake_s3):
    from uploads import UploadError, UploadStorage

    storage = UploadStorage(settings, fake_s3)
    with pytest.raises(UploadError):
        await storage.save_cover_upload("empty", _FakeUpload(b""))


def test_process_cover_raises_when_ffmpeg_fails(settings, fake_s3, monkeypatch):
    from uploads import UploadError, UploadStorage

    monkeypatch.setattr("uploads.subprocess.run", lambda *a, **k: _Completed(returncode=1, stderr="not an image"))
    storage = UploadStorage(settings, fake_s3)
    storage.init()
    src = storage.cover_stage_path("x")
    src.write_bytes(b"junk")

    with pytest.raises(UploadError) as excinfo:
        storage.process_cover("x", src)
    assert "not an image" in str(excinfo.value)
    assert not fake_s3.exists("uploads/x.jpg")
    assert not src.exists()  # the staging file is always cleaned up


def test_process_cover_writes_jpg(settings, fake_s3, monkeypatch):
    from uploads import UploadStorage

    def fake_run(cmd, *a, **k):
        open(cmd[-1], "wb").write(b"\xff\xd8jpegdata")
        return _Completed(returncode=0)

    monkeypatch.setattr("uploads.subprocess.run", fake_run)
    storage = UploadStorage(settings, fake_s3)
    storage.init()
    src = storage.cover_stage_path("x")
    src.write_bytes(b"png-bytes")

    storage.process_cover("x", src)

    assert fake_s3.get_bytes("uploads/x.jpg") == b"\xff\xd8jpegdata"
    assert not src.exists()


@pytest.mark.asyncio
async def test_job_keeps_a_user_supplied_cover(settings, fake_s3, monkeypatch):
    """A cover uploaded while the transcode runs must not be replaced by
    whatever art is embedded in the source file."""
    from uploads import UploadJobs, UploadStorage, ProbeResult

    storage = UploadStorage(settings, fake_s3)
    storage.init()
    fake_s3.put_bytes("uploads/mid.jpg", b"user-cover")

    monkeypatch.setattr(storage, "probe", lambda src: ProbeResult(duration=5.0, title="", artist=""))
    monkeypatch.setattr(storage, "transcode", lambda mid, src: fake_s3.put_bytes("uploads/mid.m4a", b"m4a"))
    extract_called = False

    def _extract(mid, src):
        nonlocal extract_called
        extract_called = True
        return True

    monkeypatch.setattr(storage, "extract_cover", _extract)

    jobs = UploadJobs(storage, settings)
    src = storage.stage_path("mid")
    src.write_bytes(b"src")
    await jobs._run("mid", src)

    assert not extract_called
    assert fake_s3.get_bytes("uploads/mid.jpg") == b"user-cover"
    assert jobs.status("mid").has_cover is True


def test_transcode_raises_when_ffmpeg_fails(settings, fake_s3, monkeypatch):
    from uploads import UploadError, UploadStorage

    monkeypatch.setattr("uploads.subprocess.run", lambda *a, **k: _Completed(returncode=1, stderr="boom"))
    storage = UploadStorage(settings, fake_s3)
    storage.init()

    with pytest.raises(UploadError) as excinfo:
        storage.transcode("x", settings.work_dir / "src.part")
    assert "boom" in str(excinfo.value)  # ffmpeg stderr is surfaced
    assert not (settings.work_dir / "uploads" / "x.m4a.tmp").exists()


def test_transcode_forces_output_format(settings, fake_s3, monkeypatch):
    """The temp file ends in .tmp, so ffmpeg must be told the muxer explicitly."""
    from uploads import UploadStorage

    seen = {}

    def fake_run(cmd, *a, **k):
        seen["cmd"] = cmd
        out = cmd[-1]
        open(out, "wb").write(b"x" * 10)
        return _Completed(returncode=0)

    monkeypatch.setattr("uploads.subprocess.run", fake_run)
    storage = UploadStorage(settings, fake_s3)
    storage.init()
    storage.transcode("x", settings.work_dir / "src.part")

    assert "-f" in seen["cmd"]
    assert fake_s3.exists("uploads/x.m4a")


def test_delete_removes_all_artifacts(settings, fake_s3):
    from uploads import UploadStorage

    storage = UploadStorage(settings, fake_s3)
    storage.init()
    fake_s3.put_bytes("uploads/m.m4a", b"x")
    fake_s3.put_bytes("uploads/m.jpg", b"x")

    storage.delete("m")

    assert not fake_s3.exists("uploads/m.m4a")
    assert not fake_s3.exists("uploads/m.jpg")


def test_status_is_derived_from_store_when_job_unknown(settings, fake_s3):
    from uploads import UploadJobs, UploadStorage

    storage = UploadStorage(settings, fake_s3)
    storage.init()
    jobs = UploadJobs(storage, settings)

    assert jobs.status("missing") is None

    storage.stage_path("orphan").write_bytes(b"x")
    assert jobs.status("orphan").status == "failed"

    fake_s3.put_bytes("uploads/done.m4a", b"x")
    assert jobs.status("done").status == "ready"


@pytest.mark.asyncio
async def test_media_cleanup_leaves_upload_objects_untouched(settings, fake_s3):
    """MediaStorage TTL cleanup must never walk into the uploads prefix."""
    from storage import MediaStorage

    fake_s3.put_bytes("uploads/keeper.m4a", b"x")
    fake_s3.backdate("uploads/keeper.m4a", settings.media_ttl + 1)

    media = MediaStorage(settings, lambda *_: True, fake_s3)
    await media.cleanup()

    assert fake_s3.exists("uploads/keeper.m4a")


def test_init_moves_legacy_prefix(settings, fake_s3):
    from uploads import UploadStorage

    fake_s3.put_bytes("mashups/old.m4a", b"audio")
    fake_s3.put_bytes("mashups/old.jpg", b"cover")
    fake_s3.put_bytes("uploads/keep.m4a", b"keep")

    UploadStorage(settings, fake_s3).init()

    assert fake_s3.get_bytes("uploads/old.m4a") == b"audio"
    assert fake_s3.get_bytes("uploads/old.jpg") == b"cover"
    assert not fake_s3.exists("mashups/old.m4a")
    assert not fake_s3.exists("mashups/old.jpg")
    assert fake_s3.get_bytes("uploads/keep.m4a") == b"keep"


@pytest.fixture
def upload_app(settings, fake_s3):
    from api import create_app
    from service import MediaService

    service = MediaService(settings, fake_s3)
    service.start()
    return create_app(service), service


@pytest.mark.asyncio
async def test_upload_endpoint_accepts_file_without_id(upload_app):
    app, _ = upload_app

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.post("/v1/uploads", files={"file": ("a.mp3", b"x")})

    assert response.status_code == 202
    body = response.json()
    assert body["status"] == "processing"
    assert len(body["id"]) == 8
    assert len(body["media_id"]) == 16


@pytest.mark.asyncio
async def test_upload_media_endpoint_supports_range(upload_app, fake_s3):
    app, service = upload_app
    fake_s3.put_bytes("uploads/song.m4a", b"0123456789")

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        ranged = await client.get("/v1/uploads/song", headers={"Range": "bytes=2-5"})
        missing = await client.get("/v1/uploads/nope")

    assert ranged.status_code == 206
    assert ranged.content == b"2345"
    assert ranged.headers["content-range"] == "bytes 2-5/10"
    assert missing.status_code == 404


@pytest.mark.asyncio
async def test_upload_cover_endpoint(upload_app, fake_s3, monkeypatch):
    app, service = upload_app

    def fake_run(cmd, *a, **k):
        open(cmd[-1], "wb").write(b"\xff\xd8jpeg")
        return _Completed(returncode=0)

    monkeypatch.setattr("uploads.subprocess.run", fake_run)

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.put("/v1/uploads/song/cover", files={"file": ("art.png", b"png-bytes")})

    assert response.status_code == 200
    assert response.json() == {"ok": True, "has_cover": True}
    assert fake_s3.exists("uploads/song.jpg")
    assert not service.uploads.cover_stage_path("song").exists()


@pytest.mark.asyncio
async def test_upload_delete_endpoint_removes_file(upload_app, fake_s3):
    app, service = upload_app
    fake_s3.put_bytes("uploads/gone.m4a", b"x")

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.delete("/v1/uploads/gone")

    assert response.status_code == 200
    assert response.json() == {"ok": True}
    assert not fake_s3.exists("uploads/gone.m4a")




import pytest
from httpx import ASGITransport, AsyncClient


@pytest.fixture
def media_app(settings, fake_s3):
    from api import create_app
    from service import MediaService

    service = MediaService(settings, fake_s3)
    service.start()
    return create_app(service), service


@pytest.mark.asyncio
async def test_resolve_endpoint_returns_provider_result(media_app, monkeypatch):
    app, service = media_app

    async def resolve(url):
        return _resolve_result()

    monkeypatch.setattr(service, "resolve", resolve)

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.post("/v1/resolve", json={"url": "https://example.test/song"})

    assert response.status_code == 200
    assert response.json()["media_id"] == "media_test"


@pytest.mark.asyncio
async def test_resolve_endpoint_returns_bad_request_when_unresolved(media_app, monkeypatch):
    app, service = media_app

    async def resolve(url):
        return None

    monkeypatch.setattr(service, "resolve", resolve)

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.post("/v1/resolve", json={"url": "bad"})

    assert response.status_code == 400


@pytest.mark.asyncio
async def test_ensure_endpoint_returns_only_ready_ids(media_app, monkeypatch):
    app, service = media_app

    async def ensure(source_url, media_id):
        return media_id == "media_ready"

    monkeypatch.setattr(service, "ensure", ensure)

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.post("/v1/music/ensure", json={
            "items": [
                {"source_url": "source", "media_id": "media_ready"},
                {"source_url": "source", "media_id": "media_failed"},
            ],
        })

    assert response.status_code == 200
    assert response.json() == {"ready": ["media_ready"]}


@pytest.mark.asyncio
async def test_media_endpoint_supports_range_and_rejects_invalid_id(media_app, fake_s3):
    app, service = media_app
    fake_s3.put_bytes("tracks/media_audio.m4a", b"0123456789")

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        ranged = await client.get("/v1/music/media_audio", headers={"Range": "bytes=2-5"})
        invalid = await client.get("/v1/music/../escape")
        missing = await client.get("/v1/music/media_missing")

    assert ranged.status_code == 206
    assert ranged.content == b"2345"
    assert ranged.headers["content-range"] == "bytes 2-5/10"
    assert invalid.status_code in (400, 404)
    assert missing.status_code == 404


@pytest.mark.asyncio
async def test_media_captions_returns_cues_from_disk(media_app, fake_s3):
    app, service = media_app
    fake_s3.put_bytes(
        "tracks/media_song.en.vtt",
        b"WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nfirst line\n",
    )

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.get("/v1/music/media_song/captions")

    assert response.status_code == 200
    body = response.json()
    assert body["lang"] == "en"
    assert body["auto"] is False
    assert body["cues"] == [{"start": 1.0, "dur": 2.0, "text": "first line"}]


@pytest.mark.asyncio
async def test_media_captions_marks_auto_generated(media_app, fake_s3):
    app, service = media_app
    fake_s3.put_bytes(
        "tracks/media_song.auto.ru.vtt",
        "WEBVTT\n\n00:00:00.500 --> 00:00:02.500\nстрочка\n".encode(),
    )

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.get("/v1/music/media_song/captions")

    assert response.status_code == 200
    body = response.json()
    assert body["lang"] == "ru"
    assert body["auto"] is True


@pytest.mark.asyncio
async def test_media_captions_empty_when_no_vtt(media_app):
    app, service = media_app

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.get("/v1/music/media_nolyrics/captions")

    assert response.status_code == 200
    assert response.json() == {"lang": "", "auto": False, "cues": []}


@pytest.mark.asyncio
async def test_media_captions_rejects_invalid_id(media_app):
    app, _ = media_app

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.get("/v1/music/bad$id/captions")

    assert response.status_code == 400


@pytest.mark.asyncio
async def test_upload_mints_ids_with_the_data(media_app):
    app, _ = media_app

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        first = await client.post("/v1/uploads", files={"file": ("a.mp3", b"audio-1")})
        second = await client.post("/v1/uploads", files={"file": ("b.mp3", b"audio-2")})

    assert first.status_code == 202
    assert second.status_code == 202
    one, two = first.json(), second.json()
    assert one["status"] == "processing"
    assert len(one["id"]) == 8 and len(one["media_id"]) == 16
    assert one["id"] != two["id"] and one["media_id"] != two["media_id"]


def _resolve_result():
    return {
        "media_id": "media_test",
        "title": "Song",
        "artist": "Artist",
        "thumbnail": "",
        "duration": 10,
        "source_url": "https://example.test/song",
    }

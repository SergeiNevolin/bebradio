import pytest
import media


@pytest.mark.asyncio
async def test_serve_track_returns_audio_file(client, tmp_path):
    media._media_dir = tmp_path
    test_file = tmp_path / "test123.m4a"
    test_file.write_bytes(b"fake audio content")
    res = await client.get("/api/media/test123")
    assert res.status_code == 200
    assert res.content == b"fake audio content"
    assert "audio/mp4" in res.headers.get("content-type", "")
    assert res.headers.get("accept-ranges") == "bytes"


@pytest.mark.asyncio
async def test_serve_track_returns_404_when_missing(client, tmp_path):
    media._media_dir = tmp_path
    res = await client.get("/api/media/missing")
    assert res.status_code == 404


@pytest.mark.asyncio
async def test_serve_track_rejects_path_traversal(client, tmp_path):
    media._media_dir = tmp_path
    res = await client.get("/api/media/..%2F..%2Fetc%2Fpasswd")
    assert res.status_code in (400, 404)
    res = await client.get("/api/media/../../etc/passwd")
    assert res.status_code in (400, 404)


@pytest.mark.asyncio
async def test_serve_track_range_request(client, tmp_path):
    media._media_dir = tmp_path
    test_file = tmp_path / "range1.m4a"
    test_file.write_bytes(b"0123456789abcdef")
    res = await client.get("/api/media/range1", headers={"Range": "bytes=4-7"})
    assert res.status_code == 206
    assert res.content == b"4567"
    assert "bytes 4-7/16" in res.headers.get("content-range", "")


@pytest.mark.asyncio
async def test_serve_track_range_request_full(client, tmp_path):
    media._media_dir = tmp_path
    test_file = tmp_path / "full.m4a"
    test_file.write_bytes(b"hello")
    res = await client.get("/api/media/full", headers={"Range": "bytes=0-4"})
    assert res.status_code == 206
    assert res.content == b"hello"


@pytest.mark.asyncio
async def test_serve_track_range_out_of_range(client, tmp_path):
    media._media_dir = tmp_path
    test_file = tmp_path / "oor.m4a"
    test_file.write_bytes(b"abc")
    res = await client.get("/api/media/oor", headers={"Range": "bytes=10-20"})
    assert res.status_code == 416


@pytest.mark.asyncio
async def test_serve_track_mp3_content_type(client, tmp_path):
    media._media_dir = tmp_path
    test_file = tmp_path / "song123.mp3"
    test_file.write_bytes(b"fake mp3")
    res = await client.get("/api/media/song123")
    assert res.status_code == 200
    assert "audio/mpeg" in res.headers.get("content-type", "")


@pytest.mark.asyncio
async def test_serve_track_webm_content_type(client, tmp_path):
    media._media_dir = tmp_path
    test_file = tmp_path / "track456.webm"
    test_file.write_bytes(b"fake webm")
    res = await client.get("/api/media/track456")
    assert res.status_code == 200
    assert "audio/webm" in res.headers.get("content-type", "")

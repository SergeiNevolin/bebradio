import pytest
import json
from tests.conftest import register, auth_header
import models
import store


def test_parse_json3_extracts_timed_cues():
    import youtube
    raw = json.dumps({
        "events": [
            {"tStartMs": 0, "dDurationMs": 500},
            {"tStartMs": 1000, "dDurationMs": 2000, "segs": [{"utf8": "Hello "}, {"utf8": "world"}]},
            {"tStartMs": 3000, "dDurationMs": 2000, "segs": [{"utf8": "\n"}]},
            {"tStartMs": 3500, "dDurationMs": 1500, "segs": [{"utf8": "second line"}]},
        ]
    })
    cues = youtube._parse_json3(raw)
    assert [c["text"] for c in cues] == ["Hello world", "second line"]
    assert cues[0]["start"] == 1.0
    assert cues[0]["dur"] == 2.0


def test_parse_vtt_extracts_timed_cues():
    import youtube
    raw = (
        "WEBVTT\n\n"
        "00:00:01.000 --> 00:00:04.000\n"
        "Hello <c>world</c>\n\n"
        "00:00:04.000 --> 00:00:07.500\n"
        "second line\n"
    )
    cues = youtube._parse_vtt(raw)
    assert [c["text"] for c in cues] == ["Hello world", "second line"]
    assert cues[0]["start"] == 1.0
    assert cues[1]["dur"] == 3.5


def test_fetch_subtitles_prefers_manual_and_caches(monkeypatch):
    import youtube
    youtube._SUBS_CACHE.clear()
    calls = {"run": 0, "dl": 0}

    class _Result:
        returncode = 0
        stdout = json.dumps({
            "language": "en",
            "subtitles": {"en": [{"ext": "json3", "url": "http://sub/manual"}]},
            "automatic_captions": {"en": [{"ext": "json3", "url": "http://sub/auto"}]},
        })

    def _run(*a, **k):
        calls["run"] += 1
        return _Result()

    def _download(url):
        calls["dl"] += 1
        assert url == "http://sub/manual"
        return json.dumps({"events": [
            {"tStartMs": 0, "dDurationMs": 1000, "segs": [{"utf8": "line one"}]},
        ]})

    monkeypatch.setattr(youtube.subprocess, "run", _run)
    monkeypatch.setattr(youtube, "_download", _download)
    out = youtube.fetch_subtitles("https://youtu.be/dQw4w9WgXcQ")
    assert out["auto"] is False
    assert out["lang"] == "en"
    assert [c["text"] for c in out["cues"]] == ["line one"]
    again = youtube.fetch_subtitles("https://youtu.be/dQw4w9WgXcQ")
    assert again == out
    assert calls == {"run": 1, "dl": 1}


def test_fetch_subtitles_empty_when_no_captions(monkeypatch):
    import youtube
    youtube._SUBS_CACHE.clear()

    class _Result:
        returncode = 0
        stdout = json.dumps({"subtitles": {}, "automatic_captions": {}})

    monkeypatch.setattr(youtube.subprocess, "run", lambda *a, **k: _Result())
    monkeypatch.setattr(youtube, "_download", lambda url: pytest.fail("should not download"))
    out = youtube.fetch_subtitles("https://youtu.be/dQw4w9WgXcQ")
    assert out["cues"] == []


def test_fetch_subtitles_bad_url_returns_empty():
    import youtube
    assert youtube.fetch_subtitles("not a youtube url")["cues"] == []


@pytest.mark.asyncio
async def test_get_lyrics_endpoint(client, monkeypatch):
    import routes.rooms as rooms_module
    monkeypatch.setattr(
        rooms_module, "fetch_subtitles",
        lambda src, lang="": {
            "lang": "en", "auto": True,
            "cues": [{"start": 1.0, "dur": 2.0, "text": "sing along"}],
        },
    )
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "K"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id="a", source_url="https://youtu.be/dQw4w9WgXcQ")]
    res = await client.get(f"/api/rooms/{room_id}/lyrics")
    assert res.status_code == 200
    body = res.json()
    assert body["available"] is True
    assert body["auto"] is True
    assert body["track_id"] == "a"
    assert body["cues"][0]["text"] == "sing along"


@pytest.mark.asyncio
async def test_get_lyrics_no_track(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "K"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.get(f"/api/rooms/{room_id}/lyrics")
    assert res.status_code == 200
    assert res.json() == {"available": False, "track_id": None, "cues": []}


@pytest.mark.asyncio
async def test_get_lyrics_track_without_source_url(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "K"}, headers=auth_header(token))
    room_id = create.json()["id"]
    store.rooms[room_id].queue = [models.Track(id="x", source_url="")]
    res = await client.get(f"/api/rooms/{room_id}/lyrics")
    assert res.json() == {"available": False, "track_id": "x", "cues": []}

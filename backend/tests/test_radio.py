import pytest
from tests.conftest import register, auth_header
import models
import store


def test_needs_refill_requires_setting_and_seed():
    import radio
    r = models.Room()
    r.radio_seed_url = "https://youtu.be/abc"
    assert radio.needs_refill(r) is False
    r.auto_radio = True
    assert radio.needs_refill(r) is True
    r.queue = [models.Track(id=str(i)) for i in range(5)]
    assert radio.needs_refill(r) is False


@pytest.mark.asyncio
async def test_refill_appends_related_tracks(monkeypatch):
    import radio

    async def fake_related(seed, limit):
        return ["https://www.youtube.com/watch?v=rel0000000a",
                "https://www.youtube.com/watch?v=rel0000000b"]

    async def fake_fetch(url):
        return {
            "title": "T", "artist": "A", "thumbnail": "",
            "duration": 100, "source_url": url, "media_id": f"media_{url[-1]}",
        }

    monkeypatch.setattr(
        radio, "fetch_related",
        fake_related,
    )
    monkeypatch.setattr(radio, "fetch_track", fake_fetch)
    r = models.Room(auto_radio=True)
    r.radio_seed_url = "https://www.youtube.com/watch?v=seed000000a"
    added = await radio.refill(r)
    assert added is True
    assert len(r.queue) == 2
    assert all(t.added_by == "📻 Radio" for t in r.queue)
    assert r.is_playing is True


@pytest.mark.asyncio
async def test_refill_noop_when_disabled(monkeypatch):
    import radio

    async def fail_related(*args, **kwargs):
        raise AssertionError("called")

    monkeypatch.setattr(radio, "fetch_related", fail_related)
    r = models.Room(auto_radio=False)
    r.radio_seed_url = "https://youtu.be/abc"
    assert await radio.refill(r) is False


@pytest.mark.asyncio
async def test_update_room_settings_toggles_auto_radio(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "R"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={"auto_radio": True}, headers=auth_header(token))
    assert res.status_code == 200
    assert res.json()["auto_radio"] is True
    store.rooms.clear()
    res = await client.get(f"/api/rooms/{room_id}")
    assert res.json()["auto_radio"] is True


def test_room_settings_schema_accepts_auto_radio():
    from schemas import RoomSettingsRequest
    req = RoomSettingsRequest(auto_radio=True)
    assert req.auto_radio is True
    assert "auto_radio" in req.model_fields_set
    assert RoomSettingsRequest().auto_radio is None


@pytest.mark.asyncio
async def test_maybe_refill_spawns_task_only_when_needed(monkeypatch):
    import radio
    spawned = []
    monkeypatch.setattr(radio, "refill_and_broadcast", lambda room: None)
    monkeypatch.setattr(radio.asyncio, "create_task", lambda coro: spawned.append(coro))
    idle = models.Room(auto_radio=False)
    radio.maybe_refill(idle)
    assert spawned == []
    due = models.Room(auto_radio=True)
    due.radio_seed_url = "https://youtu.be/abc"
    radio.maybe_refill(due)
    assert len(spawned) == 1


def test_room_to_dict_exposes_radio_searching():
    r = models.Room()
    assert r.to_dict()["radio_searching"] is False
    r.radio_filling = True
    assert r.to_dict()["radio_searching"] is True


@pytest.mark.asyncio
async def test_refill_and_broadcast_announces_search_then_result(monkeypatch):
    import radio
    seen = []

    async def fake_broadcast(room_id, payload):
        seen.append(payload["radio_searching"])

    async def fake_save_tracks(room):
        return None

    monkeypatch.setattr(radio.manager, "broadcast", fake_broadcast)
    monkeypatch.setattr(store, "save_tracks", fake_save_tracks)
    async def fake_related(seed, limit):
        return ["https://www.youtube.com/watch?v=rel0000000a"]

    async def fake_fetch(url):
        return {"title": "T", "artist": "A", "thumbnail": "",
                "duration": 100, "source_url": url, "media_id": "media_related"}

    monkeypatch.setattr(radio, "fetch_related", fake_related)
    monkeypatch.setattr(radio, "fetch_track", fake_fetch)
    async def fake_ensure(room):
        room.current_track().url = "/api/media/media_related"
        room.current_track().local_path = "media_related.m4a"
        return True

    monkeypatch.setattr(radio, "ensure_room_media", fake_ensure)
    r = models.Room(auto_radio=True)
    r.radio_seed_url = "https://www.youtube.com/watch?v=seed000000a"
    await radio.refill_and_broadcast(r)
    assert seen[0] is True
    assert seen[-1] is False
    assert len(r.queue) == 1


@pytest.mark.asyncio
async def test_empty_radio_refill_prepares_first_track(monkeypatch):
    import radio

    async def fake_related(seed, limit):
        return ["https://www.youtube.com/watch?v=rel0000000a"]

    async def fake_fetch(url):
        return {
            "title": "Radio song", "artist": "A", "thumbnail": "",
            "duration": 100, "source_url": url, "media_id": "media_radio",
        }

    async def fake_ensure(track):
        track.url = "/api/media/media_radio"
        track.local_path = "media_radio.m4a"
        return True

    monkeypatch.setattr(radio, "fetch_related", fake_related)
    monkeypatch.setattr(radio, "fetch_track", fake_fetch)
    monkeypatch.setattr(radio, "ensure_track_ready", fake_ensure)

    async def fake_save_tracks(room):
        return None

    async def broadcast(*args):
        return None

    monkeypatch.setattr(radio.manager, "broadcast", broadcast)
    monkeypatch.setattr("store.save_tracks", fake_save_tracks)

    room = models.Room(auto_radio=True)
    room.radio_seed_url = "https://www.youtube.com/watch?v=seed000000a"
    await radio.refill_and_broadcast(room)

    assert room.current_index == 0
    assert room.is_playing is True
    assert room.queue[0].url == "/api/media/media_radio"


@pytest.mark.asyncio
async def test_refill_exception_resets_radio_filling(monkeypatch):
    """When _collect_tracks raises, radio_filling is reset via finally block."""
    import radio

    async def boom(*a, **k):
        raise RuntimeError("network error")

    monkeypatch.setattr(radio, "fetch_related", boom)
    r = models.Room(auto_radio=True)
    r.radio_seed_url = "https://www.youtube.com/watch?v=seed000000a"
    assert r.radio_filling is False
    with pytest.raises(RuntimeError):
        await radio.refill(r)
    assert r.radio_filling is False


@pytest.mark.asyncio
async def test_refill_skips_too_long_tracks(monkeypatch):
    import radio
    async def fake_related(seed, limit):
        return [
            "https://www.youtube.com/watch?v=shortvideoa",
            "https://www.youtube.com/watch?v=longvideobbb",
        ]
    monkeypatch.setattr(radio, "fetch_related", fake_related)
    calls = {"n": 0}

    async def fake_fetch(url):
        calls["n"] += 1
        if "longvideo" in url:
            return {"title": "Long", "duration": 7200, "source_url": url, "media_id": "media_long"}
        return {"title": "Short", "duration": 120, "source_url": url, "media_id": "media_short"}

    monkeypatch.setattr(radio, "fetch_track", fake_fetch)
    r = models.Room(auto_radio=True)
    r.radio_seed_url = "https://www.youtube.com/watch?v=seed000000a"
    added = await radio.refill(r)
    assert added is True
    assert len(r.queue) == 1
    assert r.queue[0].title == "Short"


@pytest.mark.asyncio
async def test_refill_noop_when_no_seed():
    import radio
    r = models.Room(auto_radio=True)
    r.radio_seed_url = ""
    assert await radio.refill(r) is False
    assert r.radio_filling is False

import pytest
import time as _t
from tests.conftest import register, auth_header
import models
import store


@pytest.mark.asyncio
async def test_playback_next(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    res = await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    assert res.status_code == 200
    assert len(room.queue) == 2
    assert room.current_index == 0


@pytest.mark.asyncio
async def test_playback_next_at_end(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id="0")]
    room.current_index = 0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    assert len(room.queue) == 0
    assert room.current_index == 0


@pytest.mark.asyncio
async def test_playback_prev(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    room.current_index = 2
    res = await client.post(f"/api/rooms/{room_id}/playback", json={"action": "prev"})
    assert res.status_code == 200
    assert room.current_index == 1


@pytest.mark.asyncio
async def test_playback_prev_at_start(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id="0")]
    room.current_index = 0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "prev"})
    assert room.current_index == 0


@pytest.mark.asyncio
async def test_playback_set_position(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "seek", "position": 42.5})
    assert store.rooms[room_id].position == 42.5


@pytest.mark.asyncio
async def test_playback_not_found(client):
    res = await client.post("/api/rooms/XXXXXX/playback", json={"action": "next"})
    assert res.status_code == 404


@pytest.mark.asyncio
async def test_playback_next_resets_position(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    room.position = 50.0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    assert room.position == 0.0


@pytest.mark.asyncio
async def test_playback_prev_resets_position(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    room.current_index = 2
    room.position = 50.0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "prev"})
    assert room.position == 0.0


@pytest.mark.asyncio
async def test_playback_seek_sets_position(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "seek", "position": 33.3})
    assert store.rooms[room_id].position == 33.3


@pytest.mark.asyncio
async def test_playback_jump_resets_position(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    room.position = 100.0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "jump", "index": 2})
    assert room.position == 0.0
    assert room.current_index == 2


@pytest.mark.asyncio
async def test_playback_next_persists_queue_to_db(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    await store.save_tracks(room)
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    store.rooms.clear()
    res = await client.get(f"/api/rooms/{room_id}")
    assert len(res.json()["queue"]) == 2


def test_go_next_with_out_of_range_index_does_not_raise():
    from playback import go_next
    r = models.Room()
    r.queue = [models.Track(id="a"), models.Track(id="b")]
    r.current_index = 9
    assert go_next(r) is True
    assert len(r.queue) == 1
    assert 0 <= r.current_index < len(r.queue)


def test_go_next_dedups_rapid_calls():
    from playback import go_next
    r = models.Room()
    r.queue = [models.Track(id="a"), models.Track(id="b"), models.Track(id="c")]
    assert go_next(r) is True
    assert go_next(r) is False
    assert len(r.queue) == 2


def test_go_next_allows_call_after_window():
    from playback import go_next
    r = models.Room()
    r.queue = [models.Track(id="a"), models.Track(id="b"), models.Track(id="c")]
    assert go_next(r) is True
    r.last_advance_at = _t.time() - 5
    assert go_next(r) is True
    assert len(r.queue) == 1


def test_go_next_records_radio_seed():
    from playback import go_next
    r = models.Room()
    r.queue = [
        models.Track(id="a", source_url="https://youtu.be/aaa"),
        models.Track(id="b", source_url="https://youtu.be/bbb"),
    ]
    go_next(r)
    assert r.radio_seed_url == "https://youtu.be/aaa"

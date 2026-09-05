import pytest
from tests.conftest import register, auth_header
import store


@pytest.mark.asyncio
async def test_create_room(client):
    token = await register(client)
    res = await client.post("/api/rooms", json={"name": "Test Room"}, headers=auth_header(token))
    assert res.status_code == 200
    data = res.json()
    assert data["name"] == "Test Room"
    assert len(data["id"]) == 6
    assert data["queue"] == []
    assert data["is_playing"] is False
    assert data["position"] == 0.0


@pytest.mark.asyncio
async def test_create_room_default_name(client):
    token = await register(client)
    res = await client.post("/api/rooms", json={}, headers=auth_header(token))
    assert res.status_code == 200
    assert res.json()["name"] == "My Room"


@pytest.mark.asyncio
async def test_create_room_no_auth(client):
    res = await client.post("/api/rooms", json={"name": "Test"})
    assert res.status_code == 401


@pytest.mark.asyncio
async def test_list_rooms_empty(client):
    res = await client.get("/api/rooms")
    assert res.status_code == 200
    assert res.json() == []


@pytest.mark.asyncio
async def test_list_rooms(client):
    token = await register(client)
    await client.post("/api/rooms", json={"name": "Room A"}, headers=auth_header(token))
    await client.post("/api/rooms", json={"name": "Room B"}, headers=auth_header(token))
    res = await client.get("/api/rooms")
    assert res.status_code == 200
    data = res.json()
    assert len(data) == 2
    names = {r["name"] for r in data}
    assert names == {"Room A", "Room B"}


@pytest.mark.asyncio
async def test_get_room(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "R1"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.get(f"/api/rooms/{room_id}")
    assert res.status_code == 200
    assert res.json()["id"] == room_id


@pytest.mark.asyncio
async def test_get_room_not_found(client):
    res = await client.get("/api/rooms/XXXXXX")
    assert res.status_code == 404


@pytest.mark.asyncio
async def test_join_room(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "J"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.post(f"/api/rooms/{room_id}/join", json={"username": "Alice"})
    assert res.status_code == 200
    assert res.json()["username"] == "Alice"


@pytest.mark.asyncio
async def test_join_room_not_found(client):
    res = await client.post("/api/rooms/XXXXXX/join", json={"username": "X"})
    assert res.status_code == 404


@pytest.mark.asyncio
async def test_room_settings_defaults(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "S"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.get(f"/api/rooms/{room_id}")
    assert res.json()["allow_anonymous_add"] is True
    assert res.json()["is_private"] is False
    assert res.json()["owner_id"]


@pytest.mark.asyncio
async def test_update_room_settings_owner(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "S"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={
        "allow_anonymous_add": False, "is_private": True
    }, headers=auth_header(token))
    assert res.status_code == 200
    assert res.json()["allow_anonymous_add"] is False
    assert res.json()["is_private"] is True


@pytest.mark.asyncio
async def test_update_room_settings_not_owner(client):
    token1 = await register(client, name="Owner", email="owner@test.com")
    token2 = await register(client, name="Other", email="other@test.com")
    create = await client.post("/api/rooms", json={"name": "S"}, headers=auth_header(token1))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={"is_private": True}, headers=auth_header(token2))
    assert res.status_code == 403


@pytest.mark.asyncio
async def test_update_room_settings_no_auth(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "S"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={"is_private": True})
    assert res.status_code == 401


@pytest.mark.asyncio
async def test_private_room_hidden_from_list(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "Secret"}, headers=auth_header(token))
    room_id = create.json()["id"]
    await client.patch(f"/api/rooms/{room_id}", json={"is_private": True}, headers=auth_header(token))
    res = await client.get("/api/rooms")
    assert len(res.json()) == 0


@pytest.mark.asyncio
async def test_private_room_still_accessible_by_id(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "Secret"}, headers=auth_header(token))
    room_id = create.json()["id"]
    await client.patch(f"/api/rooms/{room_id}", json={"is_private": True}, headers=auth_header(token))
    res = await client.get(f"/api/rooms/{room_id}")
    assert res.status_code == 200
    assert res.json()["is_private"] is True


@pytest.mark.asyncio
async def test_delete_room_owner(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "D"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.delete(f"/api/rooms/{room_id}", headers=auth_header(token))
    assert res.status_code == 200
    assert res.json()["ok"] is True
    assert room_id not in store.rooms
    assert (await client.get(f"/api/rooms/{room_id}")).status_code == 404


@pytest.mark.asyncio
async def test_delete_room_not_owner(client):
    owner = await register(client, name="Owner", email="owner@test.com")
    other = await register(client, name="Other", email="other@test.com")
    create = await client.post("/api/rooms", json={"name": "D"}, headers=auth_header(owner))
    room_id = create.json()["id"]
    res = await client.delete(f"/api/rooms/{room_id}", headers=auth_header(other))
    assert res.status_code == 403
    assert (await client.get(f"/api/rooms/{room_id}")).status_code == 200


@pytest.mark.asyncio
async def test_delete_room_no_auth(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "D"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.delete(f"/api/rooms/{room_id}")
    assert res.status_code == 401


@pytest.mark.asyncio
async def test_delete_room_not_found(client):
    token = await register(client)
    res = await client.delete("/api/rooms/XXXXXX", headers=auth_header(token))
    assert res.status_code == 404


@pytest.mark.asyncio
async def test_deleted_room_disappears_from_list(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "D"}, headers=auth_header(token))
    room_id = create.json()["id"]
    assert len((await client.get("/api/rooms")).json()) == 1
    await client.delete(f"/api/rooms/{room_id}", headers=auth_header(token))
    assert (await client.get("/api/rooms")).json() == []


@pytest.mark.asyncio
async def test_add_to_queue_rejects_too_long_video(client, monkeypatch):
    async def fake_fetch(url):
        return {"title": "Long", "artist": "A", "thumbnail": "", "duration": 7200, "source_url": url, "media_id": "media_long"}

    monkeypatch.setattr(
        "routes.rooms.fetch_track",
        fake_fetch,
    )
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "Q"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.post(f"/api/rooms/{room_id}/queue", json={"url": "https://youtu.be/long"})
    assert res.status_code == 400
    assert "too long" in res.json()["error"].lower()
    assert len(store.rooms[room_id].queue) == 0


@pytest.mark.asyncio
async def test_add_to_queue_accepts_short_video(client, monkeypatch):
    async def fake_fetch(url):
        return {"title": "Short", "artist": "A", "thumbnail": "", "duration": 180, "source_url": url, "media_id": "media_short"}

    monkeypatch.setattr(
        "routes.rooms.fetch_track",
        fake_fetch,
    )

    async def fake_ensure(track):
        track.local_path = "media_short.m4a"
        track.url = "/api/media/media_short"
        return True

    monkeypatch.setattr("routes.rooms.ensure_track_ready", fake_ensure)
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "Q"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.post(f"/api/rooms/{room_id}/queue", json={"url": "https://youtu.be/short"})
    assert res.status_code == 200
    assert len(store.rooms[room_id].queue) == 1

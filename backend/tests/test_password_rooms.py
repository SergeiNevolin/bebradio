import pytest
from tests.conftest import register, auth_header
import store


@pytest.mark.asyncio
async def test_create_room_with_password(client):
    token = await register(client)
    res = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=auth_header(token),
    )
    assert res.status_code == 200
    data = res.json()
    assert data["has_password"] is True
    assert data["access"]


@pytest.mark.asyncio
async def test_get_password_room_is_locked_without_access(client):
    token = await register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=auth_header(token),
    )
    room_id = create.json()["id"]
    store.rooms.clear()
    res = await client.get(f"/api/rooms/{room_id}")
    assert res.status_code == 200
    body = res.json()
    assert body["locked"] is True
    assert body["has_password"] is True
    assert "queue" not in body
    assert "owner_id" not in body


@pytest.mark.asyncio
async def test_owner_bypasses_room_password(client):
    token = await register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=auth_header(token),
    )
    room_id = create.json()["id"]
    store.rooms.clear()
    res = await client.get(f"/api/rooms/{room_id}", headers=auth_header(token))
    assert res.status_code == 200
    body = res.json()
    assert "locked" not in body
    assert body["queue"] == []
    assert body["access"]


@pytest.mark.asyncio
async def test_join_room_wrong_password(client):
    token = await register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=auth_header(token),
    )
    room_id = create.json()["id"]
    res = await client.post(f"/api/rooms/{room_id}/join", json={"username": "Bob", "password": "nope"})
    assert res.status_code == 403
    assert res.json()["needs_password"] is True


@pytest.mark.asyncio
async def test_join_room_missing_password(client):
    token = await register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=auth_header(token),
    )
    room_id = create.json()["id"]
    res = await client.post(f"/api/rooms/{room_id}/join", json={"username": "Bob"})
    assert res.status_code == 403


@pytest.mark.asyncio
async def test_join_room_correct_password_grants_access(client):
    token = await register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=auth_header(token),
    )
    room_id = create.json()["id"]
    store.rooms.clear()
    join = await client.post(f"/api/rooms/{room_id}/join", json={"username": "Bob", "password": "hunter2"})
    assert join.status_code == 200
    access = join.json()["access"]
    assert access
    res = await client.get(f"/api/rooms/{room_id}", params={"access": access})
    assert res.status_code == 200
    assert "locked" not in res.json()
    assert res.json()["queue"] == []


@pytest.mark.asyncio
async def test_get_room_rejects_access_token_from_other_room(client):
    token = await register(client)
    a = await client.post("/api/rooms", json={"name": "A", "password": "p"}, headers=auth_header(token))
    b = await client.post("/api/rooms", json={"name": "B", "password": "p"}, headers=auth_header(token))
    store.rooms.clear()
    res = await client.get(f"/api/rooms/{b.json()['id']}", params={"access": a.json()["access"]})
    assert res.json()["locked"] is True


@pytest.mark.asyncio
async def test_update_room_set_and_remove_password(client):
    token = await register(client)
    create = await client.post("/api/rooms", json={"name": "S"}, headers=auth_header(token))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={"password": "secret"}, headers=auth_header(token))
    assert res.json()["has_password"] is True
    res = await client.patch(f"/api/rooms/{room_id}", json={"is_private": True}, headers=auth_header(token))
    assert res.json()["has_password"] is True
    res = await client.patch(f"/api/rooms/{room_id}", json={"password": ""}, headers=auth_header(token))
    assert res.json()["has_password"] is False


@pytest.mark.asyncio
async def test_add_to_queue_forbidden_without_password_access(client):
    token = await register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=auth_header(token),
    )
    room_id = create.json()["id"]
    store.rooms.clear()
    res = await client.post(f"/api/rooms/{room_id}/queue", json={"url": "https://x"})
    assert res.status_code == 403
    assert res.json()["needs_password"] is True


@pytest.mark.asyncio
async def test_playback_forbidden_on_locked_room(client):
    token = await register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=auth_header(token),
    )
    room_id = create.json()["id"]
    store.rooms.clear()
    res = await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    assert res.status_code == 403

import pytest
import asyncio
import json
from httpx import AsyncClient, ASGITransport
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from db import Base, get_session, init_db, close_db
import models
import store
from main import app


@pytest.fixture(scope="session", autouse=True)
async def setup_db():
    await init_db("sqlite+aiosqlite:///:memory:")
    yield
    await close_db()


@pytest.fixture(autouse=True)
async def clear_state():
    store.rooms.clear()
    async with get_session() as session:
        await session.execute(text("DELETE FROM track_votes"))
        await session.execute(text("DELETE FROM chat_messages"))
        await session.execute(text("DELETE FROM tracks"))
        await session.execute(text("DELETE FROM rooms"))
        await session.execute(text("DELETE FROM users"))
        await session.commit()
    yield
    store.rooms.clear()


@pytest.fixture
def client(setup_db):
    transport = ASGITransport(app=app)
    return AsyncClient(transport=transport, base_url="http://testserver")


def _run(coro):
    return asyncio.get_event_loop().run_until_complete(coro)


async def _register(client, name="TestUser", email="test@test.com", password="pass123"):
    res = await client.post("/api/auth/register", json={
        "email": email, "username": name, "password": password
    })
    return res.json()["token"]


def _auth_header(token: str) -> dict:
    return {"Authorization": f"Bearer {token}"}


# --- Auth tests ---


@pytest.mark.asyncio
async def test_register(client):
    res = await client.post("/api/auth/register", json={
        "email": "a@b.com", "username": "Alice", "password": "secret"
    })
    assert res.status_code == 200
    data = res.json()
    assert "token" in data
    assert data["user"]["email"] == "a@b.com"
    assert data["user"]["username"] == "Alice"


@pytest.mark.asyncio
async def test_register_duplicate_email(client):
    await client.post("/api/auth/register", json={
        "email": "a@b.com", "username": "Alice", "password": "secret"
    })
    res = await client.post("/api/auth/register", json={
        "email": "a@b.com", "username": "Bob", "password": "other"
    })
    assert res.status_code == 409
    assert "already registered" in res.json()["error"]


@pytest.mark.asyncio
async def test_register_duplicate_username(client):
    await client.post("/api/auth/register", json={
        "email": "a@b.com", "username": "Alice", "password": "secret"
    })
    res = await client.post("/api/auth/register", json={
        "email": "c@d.com", "username": "Alice", "password": "other"
    })
    assert res.status_code == 409
    assert "already taken" in res.json()["error"]


@pytest.mark.asyncio
async def test_register_invalid_email(client):
    res = await client.post("/api/auth/register", json={
        "email": "not-an-email", "username": "Alice", "password": "secret"
    })
    assert res.status_code == 422


@pytest.mark.asyncio
async def test_register_short_username(client):
    res = await client.post("/api/auth/register", json={
        "email": "a@b.com", "username": "A", "password": "secret"
    })
    assert res.status_code == 422


@pytest.mark.asyncio
async def test_login(client):
    await client.post("/api/auth/register", json={
        "email": "a@b.com", "username": "Alice", "password": "secret"
    })
    res = await client.post("/api/auth/login", json={
        "email": "a@b.com", "password": "secret"
    })
    assert res.status_code == 200
    assert "token" in res.json()
    assert res.json()["user"]["username"] == "Alice"


@pytest.mark.asyncio
async def test_login_wrong_password(client):
    await client.post("/api/auth/register", json={
        "email": "a@b.com", "username": "Alice", "password": "secret"
    })
    res = await client.post("/api/auth/login", json={
        "email": "a@b.com", "password": "wrong"
    })
    assert res.status_code == 401


@pytest.mark.asyncio
async def test_login_nonexistent_user(client):
    res = await client.post("/api/auth/login", json={
        "email": "nobody@b.com", "password": "x"
    })
    assert res.status_code == 401


@pytest.mark.asyncio
async def test_me_authenticated(client):
    token = await _register(client)
    res = await client.get("/api/auth/me", headers=_auth_header(token))
    assert res.status_code == 200
    assert res.json()["user"]["username"] == "TestUser"


@pytest.mark.asyncio
async def test_me_no_token(client):
    res = await client.get("/api/auth/me")
    assert res.status_code == 401


@pytest.mark.asyncio
async def test_me_invalid_token(client):
    res = await client.get("/api/auth/me", headers={"Authorization": "Bearer invalid"})
    assert res.status_code == 401


# --- Room tests ---


@pytest.mark.asyncio
async def test_create_room(client):
    token = await _register(client)
    res = await client.post("/api/rooms", json={"name": "Test Room"}, headers=_auth_header(token))
    assert res.status_code == 200
    data = res.json()
    assert data["name"] == "Test Room"
    assert len(data["id"]) == 6
    assert data["queue"] == []
    assert data["is_playing"] is False
    assert data["position"] == 0.0


@pytest.mark.asyncio
async def test_create_room_default_name(client):
    token = await _register(client)
    res = await client.post("/api/rooms", json={}, headers=_auth_header(token))
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
    token = await _register(client)
    await client.post("/api/rooms", json={"name": "Room A"}, headers=_auth_header(token))
    await client.post("/api/rooms", json={"name": "Room B"}, headers=_auth_header(token))
    res = await client.get("/api/rooms")
    assert res.status_code == 200
    data = res.json()
    assert len(data) == 2
    names = {r["name"] for r in data}
    assert names == {"Room A", "Room B"}


@pytest.mark.asyncio
async def test_get_room(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "R1"}, headers=_auth_header(token))
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
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "J"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    res = await client.post(f"/api/rooms/{room_id}/join", json={"username": "Alice"})
    assert res.status_code == 200
    assert res.json()["username"] == "Alice"


@pytest.mark.asyncio
async def test_join_room_not_found(client):
    res = await client.post("/api/rooms/XXXXXX/join", json={"username": "X"})
    assert res.status_code == 404


# --- Queue tests ---


# --- Playback tests ---


@pytest.mark.asyncio
async def test_playback_next(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    res = await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    assert res.status_code == 200
    assert len(room.queue) == 2
    assert room.current_index == 0


@pytest.mark.asyncio
async def test_playback_next_at_end(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id="0")]
    room.current_index = 0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    assert len(room.queue) == 0
    assert room.current_index == 0


@pytest.mark.asyncio
async def test_playback_prev(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    room.current_index = 2
    res = await client.post(f"/api/rooms/{room_id}/playback", json={"action": "prev"})
    assert res.status_code == 200
    assert room.current_index == 1


@pytest.mark.asyncio
async def test_playback_prev_at_start(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id="0")]
    room.current_index = 0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "prev"})
    assert room.current_index == 0


@pytest.mark.asyncio
async def test_playback_set_position(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "seek", "position": 42.5})
    assert store.rooms[room_id].position == 42.5


@pytest.mark.asyncio
async def test_playback_not_found(client):
    res = await client.post("/api/rooms/XXXXXX/playback", json={"action": "next"})
    assert res.status_code == 404


# --- Model tests ---


def test_track_defaults():
    t = models.Track()
    assert len(t.id) == 8
    assert t.title == ""
    assert t.added_by == "Anonymous"
    assert t.added_at > 0


def test_track_to_dict():
    t = models.Track(id="abc123", title="Song", artist="Artist", url="http://x", duration=200, added_by="Bob")
    d = t.to_dict()
    assert d["id"] == "abc123"
    assert d["title"] == "Song"
    assert d["artist"] == "Artist"
    assert d["duration"] == 200
    assert d["added_by"] == "Bob"


def test_room_defaults():
    r = models.Room()
    assert len(r.id) == 6
    assert r.id == r.id.upper()
    assert r.queue == []
    assert r.is_playing is False


def test_room_current_track_empty():
    r = models.Room()
    assert r.current_track() is None


def test_room_current_track():
    r = models.Room()
    t = models.Track(id="abc")
    r.queue.append(t)
    assert r.current_track().id == "abc"


def test_room_current_track_out_of_bounds():
    r = models.Room()
    r.queue.append(models.Track())
    r.current_index = 5
    assert r.current_track() is None


def test_room_to_dict():
    r = models.Room(name="Test")
    r.queue.append(models.Track(id="t1", title="Song"))
    d = r.to_dict()
    assert d["name"] == "Test"
    assert len(d["queue"]) == 1
    assert d["current_track"]["id"] == "t1"
    assert d["user_count"] == 0


def test_room_to_dict_empty():
    r = models.Room()
    d = r.to_dict()
    assert d["current_track"] is None
    assert d["queue"] == []


# --- Sync tests ---


def test_room_get_current_position_paused():
    r = models.Room()
    r.position = 10.0
    r.is_playing = False
    assert r.get_current_position() == 10.0


def test_room_get_current_position_playing():
    import time
    r = models.Room()
    r.position = 10.0
    r.is_playing = True
    r.last_sync_at = time.time() - 5.0
    pos = r.get_current_position()
    assert 14.5 <= pos <= 15.5


def test_room_to_dict_position_playing():
    import time
    r = models.Room()
    r.position = 20.0
    r.is_playing = True
    r.last_sync_at = time.time() - 3.0
    d = r.to_dict()
    assert 22.5 <= d["position"] <= 23.5


def test_room_to_dict_position_paused():
    r = models.Room()
    r.position = 20.0
    r.is_playing = False
    d = r.to_dict()
    assert d["position"] == 20.0


@pytest.mark.asyncio
async def test_playback_next_resets_position(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    room.position = 50.0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    assert room.position == 0.0


@pytest.mark.asyncio
async def test_playback_prev_resets_position(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    room.current_index = 2
    room.position = 50.0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "prev"})
    assert room.position == 0.0


@pytest.mark.asyncio
async def test_playback_seek_sets_position(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "seek", "position": 33.3})
    assert store.rooms[room_id].position == 33.3


@pytest.mark.asyncio
async def test_playback_jump_resets_position(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    room = store.rooms[room_id]
    room.queue = [models.Track(id=str(i)) for i in range(3)]
    room.position = 100.0
    await client.post(f"/api/rooms/{room_id}/playback", json={"action": "jump", "index": 2})
    assert room.position == 0.0
    assert room.current_index == 2


# --- Room settings tests ---


@pytest.mark.asyncio
async def test_room_settings_defaults(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "S"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    res = await client.get(f"/api/rooms/{room_id}")
    assert res.json()["allow_anonymous_add"] is True
    assert res.json()["is_private"] is False
    assert res.json()["owner_id"]


@pytest.mark.asyncio
async def test_update_room_settings_owner(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "S"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={
        "allow_anonymous_add": False, "is_private": True
    }, headers=_auth_header(token))
    assert res.status_code == 200
    assert res.json()["allow_anonymous_add"] is False
    assert res.json()["is_private"] is True


@pytest.mark.asyncio
async def test_update_room_settings_not_owner(client):
    token1 = await _register(client, name="Owner", email="owner@test.com")
    token2 = await _register(client, name="Other", email="other@test.com")
    create = await client.post("/api/rooms", json={"name": "S"}, headers=_auth_header(token1))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={"is_private": True}, headers=_auth_header(token2))
    assert res.status_code == 403


@pytest.mark.asyncio
async def test_update_room_settings_no_auth(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "S"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={"is_private": True})
    assert res.status_code == 401


@pytest.mark.asyncio
async def test_private_room_hidden_from_list(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "Secret"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    await client.patch(f"/api/rooms/{room_id}", json={"is_private": True}, headers=_auth_header(token))
    res = await client.get("/api/rooms")
    assert len(res.json()) == 0


@pytest.mark.asyncio
async def test_private_room_still_accessible_by_id(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "Secret"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    await client.patch(f"/api/rooms/{room_id}", json={"is_private": True}, headers=_auth_header(token))
    res = await client.get(f"/api/rooms/{room_id}")
    assert res.status_code == 200
    assert res.json()["is_private"] is True


# --- Room deletion tests ---


@pytest.mark.asyncio
async def test_delete_room_owner(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "D"}, headers=_auth_header(token))
    room_id = create.json()["id"]

    res = await client.delete(f"/api/rooms/{room_id}", headers=_auth_header(token))
    assert res.status_code == 200
    assert res.json()["ok"] is True

    assert room_id not in store.rooms
    assert (await client.get(f"/api/rooms/{room_id}")).status_code == 404


@pytest.mark.asyncio
async def test_delete_room_not_owner(client):
    owner = await _register(client, name="Owner", email="owner@test.com")
    other = await _register(client, name="Other", email="other@test.com")
    create = await client.post("/api/rooms", json={"name": "D"}, headers=_auth_header(owner))
    room_id = create.json()["id"]

    res = await client.delete(f"/api/rooms/{room_id}", headers=_auth_header(other))
    assert res.status_code == 403
    assert (await client.get(f"/api/rooms/{room_id}")).status_code == 200


@pytest.mark.asyncio
async def test_delete_room_no_auth(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "D"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    res = await client.delete(f"/api/rooms/{room_id}")
    assert res.status_code == 401


@pytest.mark.asyncio
async def test_delete_room_not_found(client):
    token = await _register(client)
    res = await client.delete("/api/rooms/XXXXXX", headers=_auth_header(token))
    assert res.status_code == 404


@pytest.mark.asyncio
async def test_deleted_room_disappears_from_list(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "D"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    assert len((await client.get("/api/rooms")).json()) == 1

    await client.delete(f"/api/rooms/{room_id}", headers=_auth_header(token))
    assert (await client.get("/api/rooms")).json() == []


# --- Chat tests ---


def test_chat_message_model():
    msg = models.ChatMessage(user_id="u1", username="Alice", text="Hello!")
    d = msg.to_dict()
    assert d["user_id"] == "u1"
    assert d["username"] == "Alice"
    assert d["text"] == "Hello!"
    assert "id" in d
    assert "created_at" in d


def test_room_messages_default_empty():
    r = models.Room()
    assert r.messages == []


# --- Vote tests ---


def test_track_vote_model():
    v = models.TrackVote(user_id="u1", track_id="t1", vote=1)
    assert v.user_id == "u1"
    assert v.vote == 1


def test_room_votes_default_empty():
    r = models.Room()
    assert r.votes == []


def test_get_track_votes():
    r = models.Room()
    r.votes.append(models.TrackVote(user_id="u1", track_id="t1", vote=1))
    r.votes.append(models.TrackVote(user_id="u2", track_id="t1", vote=1))
    r.votes.append(models.TrackVote(user_id="u3", track_id="t1", vote=-1))
    votes = r.get_track_votes("t1")
    assert votes["likes"] == 2
    assert votes["dislikes"] == 1


def test_get_track_votes_empty():
    r = models.Room()
    votes = r.get_track_votes("nonexistent")
    assert votes["likes"] == 0
    assert votes["dislikes"] == 0


def test_skip_votes_default_empty():
    r = models.Room()
    assert r.skip_votes == set()


def test_room_to_dict_includes_skip_voters():
    r = models.Room()
    r.skip_votes.add("u1")
    r.skip_votes.add("u2")
    d = r.to_dict()
    assert "skip_voters" in d
    assert len(d["skip_voters"]) == 2


# --- Password-protected room tests ---


def test_room_to_dict_has_password_flag():
    r = models.Room()
    assert r.to_dict()["has_password"] is False
    r.password_hash = "hashed"
    assert r.to_dict()["has_password"] is True


def test_room_to_dict_never_leaks_password_hash():
    r = models.Room(password_hash="super-secret-hash")
    assert "password_hash" not in r.to_dict()


@pytest.mark.asyncio
async def test_create_room_with_password(client):
    token = await _register(client)
    res = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=_auth_header(token),
    )
    assert res.status_code == 200
    data = res.json()
    assert data["has_password"] is True
    assert data["access"]


@pytest.mark.asyncio
async def test_get_password_room_is_locked_without_access(client):
    token = await _register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=_auth_header(token),
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
    token = await _register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=_auth_header(token),
    )
    room_id = create.json()["id"]
    store.rooms.clear()
    res = await client.get(f"/api/rooms/{room_id}", headers=_auth_header(token))
    assert res.status_code == 200
    body = res.json()
    assert "locked" not in body
    assert body["queue"] == []
    assert body["access"]


@pytest.mark.asyncio
async def test_join_room_wrong_password(client):
    token = await _register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=_auth_header(token),
    )
    room_id = create.json()["id"]
    res = await client.post(f"/api/rooms/{room_id}/join", json={"username": "Bob", "password": "nope"})
    assert res.status_code == 403
    assert res.json()["needs_password"] is True


@pytest.mark.asyncio
async def test_join_room_missing_password(client):
    token = await _register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=_auth_header(token),
    )
    room_id = create.json()["id"]
    res = await client.post(f"/api/rooms/{room_id}/join", json={"username": "Bob"})
    assert res.status_code == 403


@pytest.mark.asyncio
async def test_join_room_correct_password_grants_access(client):
    token = await _register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=_auth_header(token),
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
    token = await _register(client)
    a = await client.post("/api/rooms", json={"name": "A", "password": "p"}, headers=_auth_header(token))
    b = await client.post("/api/rooms", json={"name": "B", "password": "p"}, headers=_auth_header(token))
    store.rooms.clear()
    res = await client.get(f"/api/rooms/{b.json()['id']}", params={"access": a.json()["access"]})
    assert res.json()["locked"] is True


@pytest.mark.asyncio
async def test_update_room_set_and_remove_password(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "S"}, headers=_auth_header(token))
    room_id = create.json()["id"]

    res = await client.patch(f"/api/rooms/{room_id}", json={"password": "secret"}, headers=_auth_header(token))
    assert res.json()["has_password"] is True

    # An unrelated settings change must not drop the password.
    res = await client.patch(f"/api/rooms/{room_id}", json={"is_private": True}, headers=_auth_header(token))
    assert res.json()["has_password"] is True

    res = await client.patch(f"/api/rooms/{room_id}", json={"password": ""}, headers=_auth_header(token))
    assert res.json()["has_password"] is False


@pytest.mark.asyncio
async def test_add_to_queue_forbidden_without_password_access(client):
    token = await _register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=_auth_header(token),
    )
    room_id = create.json()["id"]
    store.rooms.clear()
    res = await client.post(f"/api/rooms/{room_id}/queue", json={"url": "https://x"})
    assert res.status_code == 403
    assert res.json()["needs_password"] is True


@pytest.mark.asyncio
async def test_playback_forbidden_on_locked_room(client):
    token = await _register(client)
    create = await client.post(
        "/api/rooms", json={"name": "Locked", "password": "hunter2"},
        headers=_auth_header(token),
    )
    room_id = create.json()["id"]
    store.rooms.clear()
    res = await client.post(f"/api/rooms/{room_id}/playback", json={"action": "next"})
    assert res.status_code == 403


# --- Playback persistence (regression) ---


@pytest.mark.asyncio
async def test_playback_next_persists_queue_to_db(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "P"}, headers=_auth_header(token))
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


# --- Stream URL refresh ---


def test_video_id_extraction():
    from youtube import video_id
    assert video_id("https://www.youtube.com/watch?v=dQw4w9WgXcQ") == "dQw4w9WgXcQ"
    assert video_id("https://youtu.be/dQw4w9WgXcQ") == "dQw4w9WgXcQ"
    assert video_id("https://www.youtube.com/shorts/dQw4w9WgXcQ") == "dQw4w9WgXcQ"
    assert video_id("not a url") == ""


def test_parse_stream_expiry_reads_expire_param():
    from youtube import parse_stream_expiry
    assert parse_stream_expiry("https://r1.googlevideo.com/videoplayback?expire=1700000000&x=y") == 1700000000.0


def test_parse_stream_expiry_falls_back_when_absent():
    import time as _t
    from youtube import parse_stream_expiry
    assert parse_stream_expiry("https://example.com/stream.m4a") > _t.time()


@pytest.mark.asyncio
async def test_ensure_fresh_reresolves_expired_stream(monkeypatch):
    import streams
    monkeypatch.setattr(
        streams, "resolve_stream",
        lambda src: {"stream_url": "http://fresh", "expires_at": 9_999_999_999.0},
    )
    r = models.Room()
    t = models.Track(id="a", url="http://stale", source_url="https://youtu.be/abc", stream_expires_at=1.0)
    r.queue = [t]
    changed = await streams.ensure_fresh(r, t)
    assert changed is True
    assert t.url == "http://fresh"
    assert t.stream_expires_at == 9_999_999_999.0


@pytest.mark.asyncio
async def test_ensure_fresh_skips_still_valid_stream(monkeypatch):
    import time as _t
    import streams
    called = False

    def _boom(src):
        nonlocal called
        called = True
        return None

    monkeypatch.setattr(streams, "resolve_stream", _boom)
    t = models.Track(id="a", url="http://ok", source_url="https://youtu.be/abc",
                     stream_expires_at=_t.time() + 3600)
    assert await streams.ensure_fresh(models.Room(), t) is False
    assert called is False


@pytest.mark.asyncio
async def test_ensure_fresh_ignores_track_without_source_url():
    import streams
    t = models.Track(id="a", url="http://x", source_url="", stream_expires_at=1.0)
    assert await streams.ensure_fresh(models.Room(), t) is False


# --- Server-side auto-advance de-dup ---


def test_go_next_dedups_rapid_calls():
    from playback import go_next
    r = models.Room()
    r.queue = [models.Track(id="a"), models.Track(id="b"), models.Track(id="c")]
    assert go_next(r) is True
    # Immediate second call is inside the de-dup window and is ignored.
    assert go_next(r) is False
    assert len(r.queue) == 2


def test_go_next_allows_call_after_window():
    import time as _t
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


# --- Presence / listeners ---


def test_room_to_dict_exposes_listeners_and_auto_radio():
    r = models.Room()
    d = r.to_dict()
    assert d["listeners"] == []
    assert d["auto_radio"] is False


def test_listeners_dedup_by_identity():
    r = models.Room()
    r.presence = {
        "ws1": {"id": "u1", "name": "Alice"},
        "ws2": {"id": "u1", "name": "Alice"},
        "ws3": {"id": "anon:1", "name": "Guest"},
    }
    listeners = r.listeners()
    assert {l["id"] for l in listeners} == {"u1", "anon:1"}
    assert r.to_dict()["user_count"] == 2


# --- Auto-radio ---


def test_needs_refill_requires_setting_and_seed():
    import radio
    r = models.Room()
    r.radio_seed_url = "https://youtu.be/abc"
    assert radio.needs_refill(r) is False  # auto_radio off
    r.auto_radio = True
    assert radio.needs_refill(r) is True
    r.queue = [models.Track(id=str(i)) for i in range(5)]
    assert radio.needs_refill(r) is False  # queue not low


@pytest.mark.asyncio
async def test_refill_appends_related_tracks(monkeypatch):
    import radio
    monkeypatch.setattr(
        radio, "fetch_related",
        lambda seed, limit: ["https://www.youtube.com/watch?v=rel0000000a",
                             "https://www.youtube.com/watch?v=rel0000000b"],
    )
    monkeypatch.setattr(
        radio, "fetch_track",
        lambda url: {
            "title": "T", "artist": "A", "stream_url": "http://s", "thumbnail": "",
            "duration": 100, "source_url": url, "expires_at": 9_999_999_999.0,
        },
    )
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
    monkeypatch.setattr(radio, "fetch_related", lambda *a, **k: (_ for _ in ()).throw(AssertionError("called")))
    r = models.Room(auto_radio=False)
    r.radio_seed_url = "https://youtu.be/abc"
    assert await radio.refill(r) is False


# --- Room settings: auto_radio ---


@pytest.mark.asyncio
async def test_update_room_settings_toggles_auto_radio(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "R"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    res = await client.patch(f"/api/rooms/{room_id}", json={"auto_radio": True}, headers=_auth_header(token))
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


# --- Track.from_youtube / radio.maybe_refill ---


def test_track_from_youtube_maps_info_dict():
    info = {
        "title": "Song", "artist": "Band", "stream_url": "http://s",
        "thumbnail": "http://t", "duration": 123,
        "source_url": "https://youtu.be/abc", "expires_at": 42.0,
    }
    t = models.Track.from_youtube(info, added_by="Alice")
    assert (t.title, t.artist, t.url, t.thumbnail, t.duration) == (
        "Song", "Band", "http://s", "http://t", 123,
    )
    assert t.added_by == "Alice"
    assert t.source_url == "https://youtu.be/abc"
    assert t.stream_expires_at == 42.0


def test_track_from_youtube_tolerates_missing_optional_fields():
    t = models.Track.from_youtube({"stream_url": "http://s"}, added_by="📻 Radio")
    assert t.url == "http://s"
    assert t.title == "Unknown"
    assert t.source_url == ""
    assert t.stream_expires_at == 0.0


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


# --- Karaoke / subtitles ---


def test_parse_json3_extracts_timed_cues():
    import youtube
    raw = json.dumps({
        "events": [
            {"tStartMs": 0, "dDurationMs": 500},  # no segs -> skipped
            {"tStartMs": 1000, "dDurationMs": 2000, "segs": [{"utf8": "Hello "}, {"utf8": "world"}]},
            {"tStartMs": 3000, "dDurationMs": 2000, "segs": [{"utf8": "\n"}]},  # blank -> skipped
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

    # Second call is served from cache -> no extra subprocess / download.
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
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "K"}, headers=_auth_header(token))
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
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "K"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    res = await client.get(f"/api/rooms/{room_id}/lyrics")
    assert res.status_code == 200
    assert res.json() == {"available": False, "track_id": None, "cues": []}


@pytest.mark.asyncio
async def test_get_lyrics_track_without_source_url(client):
    token = await _register(client)
    create = await client.post("/api/rooms", json={"name": "K"}, headers=_auth_header(token))
    room_id = create.json()["id"]
    store.rooms[room_id].queue = [models.Track(id="x", source_url="")]
    res = await client.get(f"/api/rooms/{room_id}/lyrics")
    assert res.json() == {"available": False, "track_id": "x", "cues": []}

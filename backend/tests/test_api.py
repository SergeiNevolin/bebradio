import pytest
import asyncio
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

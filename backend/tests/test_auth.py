import pytest
from tests.conftest import register, auth_header


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
    token = await register(client)
    res = await client.get("/api/auth/me", headers=auth_header(token))
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

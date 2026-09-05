import pytest
from httpx import AsyncClient, ASGITransport
from sqlalchemy import text

from db import get_session, init_db, close_db
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


async def register(client, name="TestUser", email="test@test.com", password="pass123"):
    res = await client.post("/api/auth/register", json={
        "email": email, "username": name, "password": password
    })
    return res.json()["token"]


def auth_header(token: str) -> dict:
    return {"Authorization": f"Bearer {token}"}

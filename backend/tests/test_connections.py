import pytest
from unittest.mock import AsyncMock, MagicMock
from connections import ConnectionManager


@pytest.mark.asyncio
async def test_broadcast_does_not_crash_on_dead_connections():
    """Regression: broadcast must snapshot the set before iterating to avoid
    RuntimeError from set mutation during iteration."""
    mgr = ConnectionManager()
    room = "R1"

    ws_ok = AsyncMock()
    ws_dead = AsyncMock()
    ws_dead.send_text.side_effect = ConnectionError("disconnected")

    mgr.connect(room, ws_ok)
    mgr.connect(room, ws_dead)

    # Should not raise RuntimeError
    await mgr.broadcast(room, {"hello": True})
    ws_ok.send_text.assert_awaited_once()
    assert mgr.get_count(room) == 1


@pytest.mark.asyncio
async def test_broadcast_to_empty_room():
    mgr = ConnectionManager()
    await mgr.broadcast("NONEXISTENT", {"data": 1})
    assert mgr.get_count("NONEXISTENT") == 0


@pytest.mark.asyncio
async def test_disconnect_cleans_up_empty_room():
    mgr = ConnectionManager()
    ws = AsyncMock()
    mgr.connect("R1", ws)
    assert mgr.get_count("R1") == 1
    mgr.disconnect("R1", ws)
    assert "R1" not in mgr._connections


@pytest.mark.asyncio
async def test_concurrent_broadcasts():
    """Two broadcasts on the same room must not interfere."""
    import asyncio
    mgr = ConnectionManager()
    room = "R2"

    sent = []

    class FakeWS:
        def __init__(self):
            self.id = id(self)

        async def send_text(self, data):
            sent.append(data)
            await asyncio.sleep(0)  # yield to event loop

    ws1 = FakeWS()
    ws2 = FakeWS()
    mgr.connect(room, ws1)
    mgr.connect(room, ws2)

    await asyncio.gather(
        mgr.broadcast(room, {"msg": "a"}),
        mgr.broadcast(room, {"msg": "b"}),
    )
    # Each broadcast reaches both websockets = 4 messages total, no crash
    assert len(sent) == 4

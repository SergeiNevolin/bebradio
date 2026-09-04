import asyncio
import json
from fastapi import WebSocket


class ConnectionManager:
    """Manages WebSocket connections per room."""

    def __init__(self):
        self._connections: dict[str, set[WebSocket]] = {}

    def connect(self, room_id: str, websocket: WebSocket) -> None:
        if room_id not in self._connections:
            self._connections[room_id] = set()
        self._connections[room_id].add(websocket)

    def disconnect(self, room_id: str, websocket: WebSocket) -> None:
        if room_id in self._connections:
            self._connections[room_id].discard(websocket)
            if not self._connections[room_id]:
                del self._connections[room_id]

    async def broadcast(self, room_id: str, state: dict) -> None:
        """Send state to all connected clients in a room concurrently."""
        conns = self._connections.get(room_id)
        if not conns:
            return

        data = json.dumps(state)

        async def _send(ws: WebSocket) -> WebSocket | None:
            try:
                await ws.send_text(data)
                return None
            except Exception:
                return ws

        results = await asyncio.gather(*[_send(ws) for ws in list(conns)])
        dead = {ws for ws in results if ws is not None}
        if dead:
            conns -= dead

    def get_count(self, room_id: str) -> int:
        """Get number of connected clients in a room."""
        return len(self._connections.get(room_id, set()))


manager = ConnectionManager()

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
        """Send state to all connected clients in a room."""
        if room_id not in self._connections:
            return

        data = json.dumps(state)
        dead: set[WebSocket] = set()

        for ws in self._connections[room_id]:
            try:
                await ws.send_text(data)
            except Exception:
                dead.add(ws)

        self._connections[room_id] -= dead

    def get_count(self, room_id: str) -> int:
        """Get number of connected clients in a room."""
        return len(self._connections.get(room_id, set()))


manager = ConnectionManager()

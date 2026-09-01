import json

from fastapi import APIRouter, WebSocket, WebSocketDisconnect

from auth import verify_room_token
from config import MAX_CHAT_MESSAGES
from connections import manager
from models import ChatMessage, TrackVote
from playback import go_next, go_prev, jump_to, seek_to
from store import get_or_load_room, rooms, save_message, save_tracks, save_votes

router = APIRouter()


@router.websocket("/ws/{room_id}")
async def websocket_endpoint(websocket: WebSocket, room_id: str, access: str | None = None):
    room_id = room_id.upper()
    await websocket.accept()

    room = await get_or_load_room(room_id)
    if room is None:
        await websocket.send_text(json.dumps({"error": "Room not found"}))
        await websocket.close()
        return

    if room.password_hash and not verify_room_token(access, room_id):
        await websocket.send_text(json.dumps({"error": "Password required", "locked": True}))
        await websocket.close()
        return

    manager.connect(room_id, websocket)
    await websocket.send_text(json.dumps(room.to_dict()))

    try:
        while True:
            data = await websocket.receive_text()
            msg = json.loads(data)

            if room_id not in rooms:
                continue

            room = rooms[room_id]
            action = msg.get("action")

            user_id = msg.get("user_id", "")
            if user_id and websocket not in room.users:
                room.users[websocket] = user_id

            queue_changed = False
            if action == "next":
                queue_changed = go_next(room)
            elif action == "prev":
                go_prev(room)
            elif action == "jump":
                jump_to(room, msg.get("index", 0))
            elif action == "seek":
                seek_to(room, msg.get("position", 0))
            elif action == "sync":
                seek_to(room, msg.get("position", 0))
            elif action == "chat":
                await _handle_chat(room, room_id, msg)
                continue
            elif action == "vote":
                queue_changed = await _handle_vote(room, msg)
            elif action == "skip_vote":
                queue_changed = await _handle_skip_vote(room, room_id, msg)
            elif action == "clear_skip_votes":
                room.skip_votes.clear()

            if queue_changed:
                await save_tracks(room)

            await manager.broadcast(room_id, room.to_dict())

    except WebSocketDisconnect:
        if room_id in rooms:
            rooms[room_id].users.pop(websocket, None)
        manager.disconnect(room_id, websocket)


async def _handle_chat(room, room_id: str, msg: dict):
    text = msg.get("text", "").strip()
    user_id = msg.get("user_id", "")
    username = msg.get("username", "Anonymous")
    if not text:
        return

    chat_msg = ChatMessage(user_id=user_id, username=username, text=text)
    room.messages.append(chat_msg)
    if len(room.messages) > MAX_CHAT_MESSAGES:
        room.messages = room.messages[-MAX_CHAT_MESSAGES:]

    await save_message(room_id, chat_msg)
    await manager.broadcast(room_id, {
        "type": "chat",
        "message": chat_msg.to_dict(),
    })


async def _handle_vote(room, msg: dict) -> bool:
    user_id = msg.get("user_id", "")
    track_id = msg.get("track_id", "")
    vote_val = msg.get("vote", 0)

    if not user_id or not track_id:
        return False

    room.votes = [
        v for v in room.votes
        if not (v.user_id == user_id and v.track_id == track_id)
    ]
    if vote_val in (1, -1):
        room.votes.append(TrackVote(user_id=user_id, track_id=track_id, vote=vote_val))

    await save_votes(room)

    current_track = room.current_track()
    if current_track and current_track.id == track_id:
        track_votes = room.get_track_votes(track_id)
        if track_votes["dislikes"] > track_votes["likes"]:
            changed = go_next(room)
            room.skip_votes.clear()
            return changed
    return False


async def _handle_skip_vote(room, room_id: str, msg: dict) -> bool:
    user_id = msg.get("user_id", "")
    if not user_id:
        return False

    if user_id in room.skip_votes:
        room.skip_votes.discard(user_id)
    else:
        room.skip_votes.add(user_id)

    listeners = max(manager.get_count(room_id), 2)
    if len(room.skip_votes) >= listeners // 2:
        changed = go_next(room)
        room.skip_votes.clear()
        return changed
    return False

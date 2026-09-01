import asyncio
import time
import uuid

from fastapi import APIRouter, Depends
from fastapi.responses import JSONResponse

from auth import get_current_user, require_user
from connections import manager
from models import Room, Track
from playback import go_next, go_prev, jump_to, seek_to
from schemas import (
    AddTrackRequest,
    CreateRoomRequest,
    JoinRequest,
    PlaybackRequest,
    RoomSettingsRequest,
)
from store import get_or_load_room, list_public_rooms, rooms, save_room, save_tracks
from youtube import fetch_track

router = APIRouter(prefix="/api")


@router.post("/rooms")
async def create_room(req: CreateRoomRequest, user=Depends(require_user)):
    room_id = str(uuid.uuid4())[:6].upper()
    room = Room(id=room_id, name=req.name, owner_id=user.id)
    rooms[room_id] = room
    await save_room(room)
    return room.to_dict()


@router.get("/rooms")
async def list_rooms():
    return await list_public_rooms()


@router.get("/rooms/{room_id}")
async def get_room(room_id: str):
    room = await get_or_load_room(room_id)
    if room is None:
        return JSONResponse(status_code=404, content={"error": "Room not found"})
    return room.to_dict()


@router.patch("/rooms/{room_id}")
async def update_room_settings(
    room_id: str,
    req: RoomSettingsRequest,
    user=Depends(require_user),
):
    room = await get_or_load_room(room_id)
    if room is None:
        return JSONResponse(status_code=404, content={"error": "Room not found"})
    if room.owner_id != user.id:
        return JSONResponse(status_code=403, content={"error": "Only the room owner can change settings"})
    if req.allow_anonymous_add is not None:
        room.allow_anonymous_add = req.allow_anonymous_add
    if req.is_private is not None:
        room.is_private = req.is_private
    await save_room(room)
    await manager.broadcast(room.id, room.to_dict())
    return room.to_dict()


@router.post("/rooms/{room_id}/join")
async def join_room(room_id: str, req: JoinRequest):
    room = await get_or_load_room(room_id)
    if room is None:
        return JSONResponse(status_code=404, content={"error": "Room not found"})
    return {"room": room.to_dict(), "username": req.username}


@router.post("/rooms/{room_id}/queue")
async def add_to_queue(
    room_id: str,
    req: AddTrackRequest,
    user=Depends(get_current_user),
):
    room = await get_or_load_room(room_id)
    if room is None:
        return JSONResponse(status_code=404, content={"error": "Room not found"})

    if user is None and not room.allow_anonymous_add:
        return JSONResponse(
            status_code=403,
            content={"error": "Anonymous users cannot add tracks to this room"},
        )

    result = await asyncio.to_thread(fetch_track, req.url)
    if not result:
        return JSONResponse(status_code=400, content={"error": "Could not fetch video info"})

    added_by = user.username if user else req.added_by or "Anonymous"
    track = Track(
        title=result["title"],
        artist=result["artist"],
        url=result["stream_url"],
        thumbnail=result["thumbnail"],
        duration=result["duration"],
        added_by=added_by,
    )
    room.queue.append(track)

    if len(room.queue) == 1:
        room.is_playing = True
        room.position = 0
        room.last_sync_at = time.time()

    await save_tracks(room)
    await manager.broadcast(room.id, room.to_dict())
    return track.to_dict()


@router.post("/rooms/{room_id}/playback")
async def update_playback(room_id: str, req: PlaybackRequest):
    room = await get_or_load_room(room_id)
    if room is None:
        return JSONResponse(status_code=404, content={"error": "Room not found"})

    if req.action == "next":
        go_next(room)
    elif req.action == "prev":
        go_prev(room)
    elif req.action == "jump" and req.index is not None:
        jump_to(room, req.index)
    elif req.action == "seek" and req.position is not None:
        seek_to(room, req.position)

    await manager.broadcast(room.id, room.to_dict())
    return room.to_dict()

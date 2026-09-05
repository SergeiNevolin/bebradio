from typing import Optional

import asyncio
from sqlalchemy import select, delete

from db import (
    ChatMessageModel,
    RoomModel,
    TrackModel,
    TrackVoteModel,
    get_session,
)
from models import ChatMessage, Room, Track, TrackVote
from media_client import fetch_track, update_media_references


rooms: dict[str, Room] = {}
# Per-room lock to prevent concurrent save_tracks from racing.
_room_locks: dict[str, asyncio.Lock] = {}
_room_locks_lock = asyncio.Lock()


async def _get_room_lock(room_id: str) -> asyncio.Lock:
    async with _room_locks_lock:
        if room_id not in _room_locks:
            _room_locks[room_id] = asyncio.Lock()
        return _room_locks[room_id]


async def load_room(room_id: str) -> Optional[Room]:
    async with get_session() as session:
        rm = await session.get(RoomModel, room_id)
        if rm is None:
            return None

        result = await session.execute(
            select(TrackModel)
            .where(TrackModel.room_id == room_id)
            .order_by(TrackModel.position_index)
        )
        tracks = [
            Track(
                id=t.id,
                title=t.title,
                artist=t.artist,
                url=t.url,
                thumbnail=t.thumbnail,
                duration=t.duration,
                added_by=t.added_by,
                source_url=t.source_url or "",
                local_path=t.local_path or "",
                media_id=t.media_id or "",
            )
            for t in result.scalars().all()
        ]

        result = await session.execute(
            select(ChatMessageModel)
            .where(ChatMessageModel.room_id == room_id)
            .order_by(ChatMessageModel.created_at)
        )
        messages = [
            ChatMessage(
                id=m.id,
                user_id=m.user_id,
                username=m.username,
                text=m.text,
                created_at=m.created_at,
            )
            for m in result.scalars().all()
        ]

        result = await session.execute(
            select(TrackVoteModel).where(TrackVoteModel.room_id == room_id)
        )
        votes = [
            TrackVote(user_id=v.user_id, track_id=v.track_id, vote=v.vote)
            for v in result.scalars().all()
        ]

        room = Room(
            id=rm.id,
            name=rm.name,
            owner_id=rm.owner_id,
            queue=tracks,
            allow_anonymous_add=rm.allow_anonymous_add,
            is_private=rm.is_private,
            password_hash=rm.password_hash,
            auto_radio=bool(rm.auto_radio),
            messages=messages,
            votes=votes,
        )
        migrated = False
        for track in room.queue:
            if not track.media_id and track.source_url:
                info = await fetch_track(track.source_url)
                if info:
                    track.media_id = info["media_id"]
                    migrated = True
        if migrated:
            await save_tracks(room)
        return room


async def get_or_load_room(room_id: str) -> Optional[Room]:
    room_id = room_id.upper()
    if room_id in rooms:
        return rooms[room_id]
    room = await load_room(room_id)
    if room:
        rooms[room_id] = room
        # Restore track URLs from local_path for any tracks that have files.
        for track in room.queue:
            if track.local_path and track.media_id and not track.url:
                key = track.media_id
                track.url = f"/api/media/{key}"
    return room


async def save_room(room: Room) -> None:
    async with get_session() as session:
        rm = await session.get(RoomModel, room.id)
        if rm is None:
            rm = RoomModel(
                id=room.id,
                name=room.name,
                owner_id=room.owner_id,
                allow_anonymous_add=room.allow_anonymous_add,
                is_private=room.is_private,
                password_hash=room.password_hash,
                auto_radio=room.auto_radio,
            )
            session.add(rm)
        else:
            rm.name = room.name
            rm.allow_anonymous_add = room.allow_anonymous_add
            rm.is_private = room.is_private
            rm.password_hash = room.password_hash
            rm.auto_radio = room.auto_radio
        await session.commit()


async def save_tracks(room: Room) -> None:
    lock = await _get_room_lock(room.id)
    async with lock:
        async with get_session() as session:
            await session.execute(
                delete(TrackModel).where(TrackModel.room_id == room.id)
            )
            if room.queue:
                await session.execute(
                    TrackModel.__table__.insert(),
                    [
                        {
                            "id": t.id,
                            "room_id": room.id,
                            "title": t.title,
                            "artist": t.artist,
                            "url": t.url,
                            "thumbnail": t.thumbnail,
                            "duration": t.duration,
                            "added_by": t.added_by,
                            "position_index": i,
                            "source_url": t.source_url,
                            "local_path": t.local_path,
                            "media_id": t.media_id,
                        }
                        for i, t in enumerate(room.queue)
                    ],
                )
            await session.commit()
    await update_media_references([
        track.media_id for track in room.queue if track.media_id
    ])


async def save_message(room_id: str, msg: ChatMessage) -> None:
    async with get_session() as session:
        session.add(ChatMessageModel(
            id=msg.id,
            room_id=room_id,
            user_id=msg.user_id,
            username=msg.username,
            text=msg.text,
            created_at=msg.created_at,
        ))
        await session.commit()


async def save_votes(room: Room) -> None:
    async with get_session() as session:
        await session.execute(
            delete(TrackVoteModel).where(TrackVoteModel.room_id == room.id)
        )
        if room.votes:
            await session.execute(
                TrackVoteModel.__table__.insert(),
                [
                    {"room_id": room.id, "user_id": v.user_id, "track_id": v.track_id, "vote": v.vote}
                    for v in room.votes
                ],
            )
        await session.commit()


async def delete_room_from_db(room_id: str) -> None:
    async with get_session() as session:
        await session.execute(delete(TrackVoteModel).where(TrackVoteModel.room_id == room_id))
        await session.execute(delete(ChatMessageModel).where(ChatMessageModel.room_id == room_id))
        await session.execute(delete(TrackModel).where(TrackModel.room_id == room_id))
        rm = await session.get(RoomModel, room_id)
        if rm:
            await session.delete(rm)
        await session.commit()


async def list_public_rooms() -> list[dict]:
    async with get_session() as session:
        result = await session.execute(
            select(RoomModel).where(RoomModel.is_private == False)
        )
        rooms_list = []
        for rm in result.scalars().all():
            if rm.id in rooms:
                r = rooms[rm.id]
                rooms_list.append({
                    "id": r.id,
                    "name": r.name,
                    "user_count": len(set(r.users.values())),
                    "track_count": len(r.queue),
                    "is_playing": r.is_playing,
                    "has_password": bool(r.password_hash),
                })
            else:
                track_count_result = await session.execute(
                    select(TrackModel).where(TrackModel.room_id == rm.id)
                )
                track_count = len(track_count_result.scalars().all())
                rooms_list.append({
                    "id": rm.id,
                    "name": rm.name,
                    "user_count": 0,
                    "track_count": track_count,
                    "is_playing": False,
                    "has_password": bool(rm.password_hash),
                })
        return rooms_list

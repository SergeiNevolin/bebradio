import time
import uuid
from dataclasses import dataclass, field
from typing import Optional

from config import MAX_CHAT_MESSAGES


@dataclass
class Track:
    id: str = field(default_factory=lambda: str(uuid.uuid4())[:8])
    title: str = ""
    artist: str = ""
    url: str = ""
    thumbnail: str = ""
    duration: int = 0
    added_by: str = "Anonymous"
    added_at: float = field(default_factory=time.time)
    source_url: str = ""
    local_path: str = ""
    # YouTube video ID — used as the media file key on disk so the same video
    # is only downloaded once even if it appears in multiple queues.
    video_id: str = ""

    @classmethod
    def from_youtube(cls, info: dict, added_by: str) -> "Track":
        """Build a queue track from a ``youtube.fetch_track`` result dict."""
        from youtube import video_id as extract_vid
        vid = extract_vid(info.get("source_url", ""))
        return cls(
            title=info.get("title", "Unknown"),
            artist=info.get("artist", "Unknown"),
            thumbnail=info.get("thumbnail", ""),
            duration=info.get("duration", 0),
            added_by=added_by,
            source_url=info.get("source_url", ""),
            video_id=vid,
        )

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "title": self.title,
            "artist": self.artist,
            "url": self.url,
            "thumbnail": self.thumbnail,
            "duration": self.duration,
            "added_by": self.added_by,
        }


@dataclass
class ChatMessage:
    id: str = field(default_factory=lambda: str(uuid.uuid4())[:8])
    user_id: str = ""
    username: str = ""
    text: str = ""
    created_at: float = field(default_factory=time.time)

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "user_id": self.user_id,
            "username": self.username,
            "text": self.text,
            "created_at": self.created_at,
        }


@dataclass
class TrackVote:
    user_id: str = ""
    track_id: str = ""
    vote: int = 0


@dataclass
class Room:
    id: str = field(default_factory=lambda: str(uuid.uuid4())[:6].upper())
    name: str = ""
    owner_id: str = ""
    queue: list[Track] = field(default_factory=list)
    current_index: int = 0
    is_playing: bool = False
    position: float = 0.0
    last_sync_at: float = field(default_factory=time.time)
    created_at: float = field(default_factory=time.time)
    users: dict = field(default_factory=dict)
    allow_anonymous_add: bool = True
    is_private: bool = False
    password_hash: Optional[str] = None
    messages: list[ChatMessage] = field(default_factory=list)
    votes: list[TrackVote] = field(default_factory=list)
    skip_votes: set[str] = field(default_factory=set)
    auto_radio: bool = False

    # --- runtime-only state (never persisted) ---
    # WebSocket -> {"id", "name"} for everyone currently connected.
    presence: dict = field(default_factory=dict)
    # Epoch of the last successful ``go_next``; guards against double-skips.
    last_advance_at: float = 0.0
    # ``source_url`` of the most recently played track, used to seed auto-radio.
    radio_seed_url: str = ""
    # Video ids already queued by auto-radio this session, to avoid repeats.
    radio_seen: set[str] = field(default_factory=set)
    # True while a background auto-radio refill is in flight.
    radio_filling: bool = False

    def current_track(self) -> Optional[Track]:
        if self.queue and 0 <= self.current_index < len(self.queue):
            return self.queue[self.current_index]
        return None

    def get_current_position(self) -> float:
        if self.is_playing:
            elapsed = time.time() - self.last_sync_at
            return self.position + elapsed
        return self.position

    def get_track_votes(self, track_id: str) -> dict:
        likes = sum(1 for v in self.votes if v.track_id == track_id and v.vote == 1)
        dislikes = sum(1 for v in self.votes if v.track_id == track_id and v.vote == -1)
        return {"likes": likes, "dislikes": dislikes}

    def listeners(self) -> list[dict]:
        """Distinct people currently connected, newest identity wins.

        Logged-in users are de-duplicated by their user id; each anonymous
        connection counts once.
        """
        seen: dict[str, str] = {}
        for info in self.presence.values():
            seen[info["id"]] = info["name"]
        return [{"id": uid, "name": name} for uid, name in seen.items()]

    def to_dict(self) -> dict:
        track = self.current_track()
        track_votes = self.get_track_votes(track.id) if track else {"likes": 0, "dislikes": 0}
        queue_with_votes = []
        for t in self.queue:
            tv = self.get_track_votes(t.id)
            entry = t.to_dict()
            entry["likes"] = tv["likes"]
            entry["dislikes"] = tv["dislikes"]
            queue_with_votes.append(entry)
        listeners = self.listeners()
        return {
            "id": self.id,
            "name": self.name,
            "owner_id": self.owner_id,
            "queue": queue_with_votes,
            "current_index": self.current_index,
            "is_playing": self.is_playing,
            "position": self.get_current_position(),
            "current_track": track.to_dict() if track else None,
            "user_count": len(listeners) or len(set(self.users.values())),
            "listeners": listeners,
            "allow_anonymous_add": self.allow_anonymous_add,
            "is_private": self.is_private,
            "auto_radio": self.auto_radio,
            "radio_searching": self.radio_filling,
            "has_password": bool(self.password_hash),
            "track_votes": track_votes,
            "skip_voters": list(self.skip_votes),
            "messages": [m.to_dict() for m in self.messages[-MAX_CHAT_MESSAGES:]],
        }

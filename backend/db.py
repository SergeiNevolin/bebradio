from sqlalchemy import (
    Boolean,
    Column,
    Float,
    ForeignKey,
    Integer,
    String,
    Text,
    func,
    text,
)
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy.orm import DeclarativeBase, relationship

from config import DATABASE_URL


class Base(DeclarativeBase):
    pass


class UserModel(Base):
    __tablename__ = "users"

    id = Column(String(8), primary_key=True)
    email = Column(String(255), unique=True, nullable=False, index=True)
    username = Column(String(30), unique=True, nullable=False)
    password_hash = Column(String(255), nullable=False)
    bio = Column(Text, default="")
    avatar_url = Column(Text, default="")
    created_at = Column(Float, server_default=func.extract("epoch", func.now()))


class RoomModel(Base):
    __tablename__ = "rooms"

    id = Column(String(6), primary_key=True)
    name = Column(String(255), nullable=False, default="My Room")
    owner_id = Column(String(8), nullable=False)
    allow_anonymous_add = Column(Boolean, default=True)
    is_private = Column(Boolean, default=False)
    password_hash = Column(String(255), nullable=True)
    auto_radio = Column(Boolean, default=False)
    created_at = Column(Float, server_default=func.extract("epoch", func.now()))

    tracks = relationship("TrackModel", back_populates="room", cascade="all, delete-orphan")
    messages = relationship("ChatMessageModel", back_populates="room", cascade="all, delete-orphan")
    votes = relationship("TrackVoteModel", back_populates="room", cascade="all, delete-orphan")


class TrackModel(Base):
    __tablename__ = "tracks"

    id = Column(String(8), primary_key=True)
    room_id = Column(String(6), ForeignKey("rooms.id", ondelete="CASCADE"), nullable=False)
    title = Column(String(500), default="")
    artist = Column(String(500), default="")
    url = Column(Text, default="")
    thumbnail = Column(Text, default="")
    duration = Column(Integer, default=0)
    added_by = Column(String(30), default="Anonymous")
    position_index = Column(Integer, nullable=False, default=0)
    source_url = Column(Text, default="")
    local_path = Column(Text, default="")
    video_id = Column(String(11), default="")
    added_at = Column(Float, server_default=func.extract("epoch", func.now()))

    room = relationship("RoomModel", back_populates="tracks")


class ChatMessageModel(Base):
    __tablename__ = "chat_messages"

    id = Column(String(8), primary_key=True)
    room_id = Column(String(6), ForeignKey("rooms.id", ondelete="CASCADE"), nullable=False)
    user_id = Column(String(8), default="")
    username = Column(String(30), default="")
    text = Column(Text, default="")
    created_at = Column(Float, server_default=func.extract("epoch", func.now()))

    room = relationship("RoomModel", back_populates="messages")


class TrackVoteModel(Base):
    __tablename__ = "track_votes"

    id = Column(Integer, primary_key=True, autoincrement=True)
    room_id = Column(String(6), ForeignKey("rooms.id", ondelete="CASCADE"), nullable=False)
    user_id = Column(String(8), nullable=False)
    track_id = Column(String(8), nullable=False)
    vote = Column(Integer, nullable=False, default=0)

    room = relationship("RoomModel", back_populates="votes")


_engine = None
_session_factory = None


async def init_db(database_url: str | None = None):
    global _engine, _session_factory
    url = database_url or DATABASE_URL
    _engine = create_async_engine(url, echo=False)
    _session_factory = async_sessionmaker(_engine, expire_on_commit=False)
    async with _engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
        await _run_migrations(conn)


# Columns added after the initial schema. ``create_all`` never alters existing
# tables, so on an already-provisioned database (production Postgres) we add the
# missing columns by hand. Postgres supports ``ADD COLUMN IF NOT EXISTS``; on
# SQLite the column is always present because tests start from a fresh database.
_MIGRATIONS = [
    "ALTER TABLE rooms ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255)",
    "ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT DEFAULT ''",
    "ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT DEFAULT ''",
    "ALTER TABLE rooms ADD COLUMN IF NOT EXISTS auto_radio BOOLEAN DEFAULT FALSE",
    "ALTER TABLE tracks ADD COLUMN IF NOT EXISTS local_path TEXT DEFAULT ''",
    "ALTER TABLE tracks ADD COLUMN IF NOT EXISTS video_id VARCHAR(11) DEFAULT ''",
]


async def _run_migrations(conn):
    if conn.dialect.name != "postgresql":
        return
    for statement in _MIGRATIONS:
        await conn.execute(text(statement))


async def close_db():
    global _engine
    if _engine:
        await _engine.dispose()


def get_session() -> AsyncSession:
    assert _session_factory is not None, "Database not initialized"
    return _session_factory()

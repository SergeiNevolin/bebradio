import time

import bcrypt
import jwt
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from sqlalchemy import select

from config import JWT_ALGORITHM, JWT_EXPIRE_HOURS, SECRET_KEY
from db import UserModel, get_session

security = HTTPBearer(auto_error=False)


def hash_password(password: str) -> str:
    return bcrypt.hashpw(password.encode(), bcrypt.gensalt()).decode()


def verify_password(password: str, password_hash: str) -> bool:
    return bcrypt.checkpw(password.encode(), password_hash.encode())


def create_token(user_id: str) -> str:
    payload = {
        "sub": user_id,
        "exp": time.time() + JWT_EXPIRE_HOURS * 3600,
    }
    return jwt.encode(payload, SECRET_KEY, algorithm=JWT_ALGORITHM)


def decode_token(token: str) -> str | None:
    try:
        payload = jwt.decode(token, SECRET_KEY, algorithms=[JWT_ALGORITHM])
        return payload.get("sub")
    except (jwt.ExpiredSignatureError, jwt.InvalidTokenError):
        return None


def create_room_token(room_id: str) -> str:
    """Grant access to a password-protected room after the password was verified."""
    payload = {
        "room": room_id,
        "scope": "room_access",
        "exp": time.time() + JWT_EXPIRE_HOURS * 3600,
    }
    return jwt.encode(payload, SECRET_KEY, algorithm=JWT_ALGORITHM)


def verify_room_token(token: str | None, room_id: str) -> bool:
    if not token:
        return False
    try:
        payload = jwt.decode(token, SECRET_KEY, algorithms=[JWT_ALGORITHM])
    except (jwt.ExpiredSignatureError, jwt.InvalidTokenError):
        return False
    return payload.get("scope") == "room_access" and payload.get("room") == room_id


async def find_user_by_id(user_id: str) -> UserModel | None:
    async with get_session() as session:
        result = await session.execute(select(UserModel).where(UserModel.id == user_id))
        return result.scalar_one_or_none()


async def find_user_by_email(email: str) -> UserModel | None:
    async with get_session() as session:
        result = await session.execute(select(UserModel).where(UserModel.email == email))
        return result.scalar_one_or_none()


async def find_user_by_username(username: str) -> UserModel | None:
    async with get_session() as session:
        result = await session.execute(select(UserModel).where(UserModel.username == username))
        return result.scalar_one_or_none()


async def get_current_user(
    credentials: HTTPAuthorizationCredentials | None = Depends(security),
):
    if credentials is None:
        return None
    user_id = decode_token(credentials.credentials)
    if user_id is None:
        return None
    return await find_user_by_id(user_id)


async def require_user(
    credentials: HTTPAuthorizationCredentials | None = Depends(security),
):
    if credentials is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Not authenticated",
        )
    user_id = decode_token(credentials.credentials)
    if user_id is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or expired token",
        )
    user = await find_user_by_id(user_id)
    if user is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="User not found",
        )
    return user

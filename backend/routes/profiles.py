from fastapi import APIRouter, Depends
from fastapi.responses import JSONResponse

from auth import get_current_user, require_user
from db import UserModel, get_session
from sqlalchemy import select
from schemas import UpdateProfileRequest

router = APIRouter(prefix="/api/users")


def _user_dict(user: UserModel, include_email: bool = False) -> dict:
    data = {
        "id": user.id,
        "username": user.username,
        "bio": user.bio or "",
        "avatar_url": user.avatar_url or "",
        "created_at": user.created_at,
    }
    if include_email:
        data["email"] = user.email
    return data


@router.get("/me")
async def get_me(user=Depends(require_user)):
    return {"user": _user_dict(user, include_email=True)}


@router.put("/me")
async def update_me(req: UpdateProfileRequest, user=Depends(require_user)):
    async with get_session() as session:
        result = await session.execute(select(UserModel).where(UserModel.id == user.id))
        db_user = result.scalar_one()
        if req.bio is not None:
            db_user.bio = req.bio
        if req.avatar_url is not None:
            db_user.avatar_url = req.avatar_url
        await session.commit()
        await session.refresh(db_user)
    return {"user": _user_dict(db_user, include_email=True)}


@router.get("/{user_id}")
async def get_user(user_id: str):
    async with get_session() as session:
        result = await session.execute(select(UserModel).where(UserModel.id == user_id))
        user = result.scalar_one_or_none()
    if user is None:
        return JSONResponse(status_code=404, content={"error": "User not found"})
    return {"user": _user_dict(user)}

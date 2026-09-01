import uuid

from fastapi import APIRouter, Depends
from fastapi.responses import JSONResponse

from auth import (
    create_token,
    find_user_by_email,
    find_user_by_username,
    get_current_user,
    hash_password,
    require_user,
    verify_password,
)
from db import UserModel, get_session
from schemas import LoginRequest, RegisterRequest

router = APIRouter(prefix="/api/auth")


@router.post("/register")
async def register(req: RegisterRequest):
    existing = await find_user_by_email(req.email)
    if existing:
        return JSONResponse(
            status_code=409, content={"error": "Email already registered"}
        )
    existing_username = await find_user_by_username(req.username)
    if existing_username:
        return JSONResponse(
            status_code=409, content={"error": "Username already taken"}
        )

    user_id = str(uuid.uuid4())[:8]
    async with get_session() as session:
        user = UserModel(
            id=user_id,
            email=req.email,
            username=req.username,
            password_hash=hash_password(req.password),
        )
        session.add(user)
        await session.commit()

    token = create_token(user_id)
    return {"token": token, "user": {"id": user_id, "email": req.email, "username": req.username}}


@router.post("/login")
async def login(req: LoginRequest):
    user = await find_user_by_email(req.email)
    if user is None:
        return JSONResponse(
            status_code=401, content={"error": "Invalid email or password"}
        )
    if not verify_password(req.password, user.password_hash):
        return JSONResponse(
            status_code=401, content={"error": "Invalid email or password"}
        )
    token = create_token(user.id)
    return {"token": token, "user": {"id": user.id, "email": user.email, "username": user.username}}


@router.get("/me")
async def me(user=Depends(get_current_user)):
    if user is None:
        return JSONResponse(status_code=401, content={"error": "Not authenticated"})
    return {"user": {"id": user.id, "email": user.email, "username": user.username}}

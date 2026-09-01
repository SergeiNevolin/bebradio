import asyncio

from fastapi import APIRouter

from schemas import SearchRequest
from youtube import search_youtube

router = APIRouter(prefix="/api")


@router.post("/search")
async def search(req: SearchRequest):
    if not req.query.strip():
        return []
    return await asyncio.to_thread(search_youtube, req.query.strip(), req.limit)

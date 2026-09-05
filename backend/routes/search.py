from fastapi import APIRouter, Depends, Request
from fastapi.responses import JSONResponse

from config import RATE_LIMIT_SEARCH, RATE_LIMIT_WINDOW
from ratelimit import rate_limit
from schemas import SearchRequest
from media_client import search_youtube

router = APIRouter(prefix="/api")

_search_limit = rate_limit(RATE_LIMIT_SEARCH, RATE_LIMIT_WINDOW)


@router.post("/search")
async def search(
    req: SearchRequest,
    _limit=Depends(_search_limit),
):
    if _limit is not None:
        return _limit
    if not req.query.strip():
        return []
    return await search_youtube(req.query.strip(), req.limit)

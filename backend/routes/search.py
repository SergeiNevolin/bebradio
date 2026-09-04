import asyncio

from fastapi import APIRouter

import providers
from schemas import SearchRequest

router = APIRouter(prefix="/api")


@router.get("/search/sources")
async def search_sources():
    """The platforms the search box can target, in display order."""
    return {"sources": list(providers.SOURCES), "default": providers.DEFAULT_SOURCE}


@router.post("/search")
async def search(req: SearchRequest):
    if not req.query.strip():
        return []
    return await asyncio.to_thread(
        providers.search, req.query.strip(), req.limit, providers.normalize(req.source)
    )

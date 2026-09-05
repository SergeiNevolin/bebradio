"""Streaming endpoint for serving downloaded audio files."""

import logging
from fastapi import APIRouter, HTTPException, Request
from fastapi.responses import Response

from media_client import MediaServiceError, media_content

log = logging.getLogger(__name__)

router = APIRouter()

@router.get("/api/media/{track_id}")
async def serve_track(track_id: str, request: Request):
    """Compatibility gateway; production proxies this path directly."""
    if not track_id or any(c not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-" for c in track_id):
        raise HTTPException(status_code=400, detail="Invalid track ID")
    try:
        response = await media_content(track_id, request.headers.get("range"))
    except MediaServiceError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    headers = {
        key: value for key, value in response.headers.items()
        if key.lower() in {"content-type", "content-length", "content-range", "accept-ranges", "cache-control"}
    }
    return Response(content=response.content, status_code=response.status_code, headers=headers)

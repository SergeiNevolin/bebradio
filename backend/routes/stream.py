"""Streaming endpoint for serving downloaded audio files."""

import logging
import mimetypes
import os
from pathlib import Path

from fastapi import APIRouter, HTTPException, Request
from fastapi.responses import FileResponse, Response, StreamingResponse

from media import get_track_path

log = logging.getLogger(__name__)

router = APIRouter()

# Supported audio MIME types by extension
_MIME_MAP = {
    ".m4a": "audio/mp4",
    ".mp3": "audio/mpeg",
    ".opus": "audio/opus",
    ".ogg": "audio/ogg",
    ".webm": "audio/webm",
    ".wav": "audio/wav",
    ".flac": "audio/flac",
}


def _guess_mime(path: Path) -> str:
    ext = path.suffix.lower()
    if ext in _MIME_MAP:
        return _MIME_MAP[ext]
    mime, _ = mimetypes.guess_type(str(path))
    return mime or "application/octet-stream"


@router.get("/api/media/{track_id}")
async def serve_track(track_id: str, request: Request):
    """Serve a downloaded audio file.

    Supports Range requests for seeking in the browser's audio player.
    """
    # Sanitize track_id to prevent path traversal
    safe_id = "".join(c for c in track_id if c.isalnum() or c in "-_")
    if not safe_id or safe_id != track_id:
        raise HTTPException(status_code=400, detail="Invalid track ID")

    path = get_track_path(safe_id)
    if not path.exists():
        raise HTTPException(status_code=404, detail="Track not found")

    content_type = _guess_mime(path)
    file_size = path.stat().st_size

    # Handle Range requests for audio seeking
    range_header = request.headers.get("range")
    if range_header:
        try:
            # Parse "bytes=start-end"
            range_spec = range_header.replace("bytes=", "")
            parts = range_spec.split("-")
            start = int(parts[0]) if parts[0] else 0
            end = int(parts[1]) if parts[1] else file_size - 1

            if start >= file_size or end >= file_size or start > end:
                return Response(
                    status_code=416,
                    headers={
                        "Content-Range": f"bytes */{file_size}",
                    },
                )

            content_length = end - start + 1

            def iter_range():
                with open(path, "rb") as f:
                    f.seek(start)
                    remaining = content_length
                    chunk_size = 64 * 1024
                    while remaining > 0:
                        read_size = min(chunk_size, remaining)
                        data = f.read(read_size)
                        if not data:
                            break
                        remaining -= len(data)
                        yield data

            return StreamingResponse(
                iter_range(),
                status_code=206,
                media_type=content_type,
                headers={
                    "Content-Range": f"bytes {start}-{end}/{file_size}",
                    "Content-Length": str(content_length),
                    "Accept-Ranges": "bytes",
                    "Cache-Control": "public, max-age=3600",
                },
            )
        except (ValueError, IndexError):
            pass

    return FileResponse(
        path,
        media_type=content_type,
        headers={
            "Accept-Ranges": "bytes",
            "Cache-Control": "public, max-age=3600",
        },
    )

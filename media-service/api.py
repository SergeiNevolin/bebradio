import asyncio
import mimetypes
import re
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import APIRouter, FastAPI, HTTPException, Request
from fastapi.responses import FileResponse, Response, StreamingResponse

from schemas import (
    DownloadRequest,
    EnsureRequest,
    QueryRequest,
    ReferencesRequest,
    RelatedRequest,
    ResolveRequest,
)
from service import MediaService

_AUDIO_MIME = {
    ".flac": "audio/flac",
    ".m4a": "audio/mp4",
    ".mp3": "audio/mpeg",
    ".ogg": "audio/ogg",
    ".opus": "audio/opus",
    ".wav": "audio/wav",
    ".webm": "audio/webm",
}


def create_app(service: MediaService) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app: FastAPI):
        service.start()
        cleanup_task = asyncio.create_task(service.storage.cleanup_loop())
        try:
            yield
        finally:
            cleanup_task.cancel()
            try:
                await cleanup_task
            except asyncio.CancelledError:
                pass

    app = FastAPI(title="bebradio media service", lifespan=lifespan)
    router = APIRouter(prefix="/v1")

    @router.post("/search")
    async def search(request: QueryRequest):
        return await service.search(request.query.strip(), request.limit)

    @router.post("/resolve")
    async def resolve(request: ResolveRequest):
        result = await service.resolve(request.url)
        if not result:
            raise HTTPException(status_code=400, detail="Could not resolve media URL")
        return result

    @router.post("/media/download")
    async def download(request: DownloadRequest):
        if not service.storage.valid_id(request.media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        ready = await service.ensure(request.url, request.media_id)
        if not ready:
            raise HTTPException(status_code=502, detail="Media download failed")
        return {
            "media_id": request.media_id,
            "filename": service.storage.path(request.media_id).name,
            "status": "ready",
        }

    @router.post("/media/ensure")
    async def ensure(request: EnsureRequest):
        async def prepare(item) -> str | None:
            if not service.storage.valid_id(item.media_id):
                return None
            return item.media_id if await service.ensure(item.source_url, item.media_id) else None

        ready = await asyncio.gather(*(prepare(item) for item in request.items))
        return {"ready": [media_id for media_id in ready if media_id]}

    @router.post("/radio/related")
    async def related(request: RelatedRequest):
        return await service.related(request.source_url, request.limit)

    @router.post("/media/references")
    async def references(request: ReferencesRequest):
        service.set_references(request.media_ids)
        return {"count": len(service.storage.referenced)}

    @router.get("/captions")
    async def captions(source_url: str, lang: str = ""):
        return await service.captions(source_url, lang)

    @router.get("/media/{media_id}")
    async def media(media_id: str, request: Request):
        if not service.storage.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        path = service.storage.path(media_id)
        if not path.is_file():
            raise HTTPException(status_code=404, detail="Track not found")
        return _file_response(path, request)

    app.include_router(router)
    return app


def _file_response(path: Path, request: Request):
    media_type = _AUDIO_MIME.get(path.suffix.lower()) or mimetypes.guess_type(str(path))[0] or "application/octet-stream"
    size = path.stat().st_size
    common_headers = {"Accept-Ranges": "bytes", "Cache-Control": "public, max-age=3600"}
    range_header = request.headers.get("range")
    if not range_header:
        return FileResponse(path, media_type=media_type, headers=common_headers)

    try:
        start_text, end_text = range_header.replace("bytes=", "").split("-", 1)
        start = int(start_text) if start_text else 0
        end = int(end_text) if end_text else size - 1
    except ValueError:
        return FileResponse(path, media_type=media_type, headers=common_headers)

    if start < 0 or end < start or end >= size:
        return Response(status_code=416, headers={"Content-Range": f"bytes */{size}"})

    length = end - start + 1

    def chunks():
        with path.open("rb") as file:
            file.seek(start)
            remaining = length
            while remaining:
                data = file.read(min(64 * 1024, remaining))
                if not data:
                    break
                remaining -= len(data)
                yield data

    headers = {
        **common_headers,
        "Content-Range": f"bytes {start}-{end}/{size}",
        "Content-Length": str(length),
    }
    return StreamingResponse(chunks(), status_code=206, media_type=media_type, headers=headers)

import asyncio
import logging
import mimetypes
import re
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import APIRouter, FastAPI, File, HTTPException, Request, UploadFile
from fastapi.responses import FileResponse, Response, StreamingResponse

from mashups import MashupError
from schemas import (
    DownloadRequest,
    EnsureRequest,
    QueryRequest,
    ReferencesRequest,
    RelatedRequest,
    ResolveRequest,
)
from service import MediaService

log = logging.getLogger(__name__)

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
        ready_ids = [media_id for media_id in ready if media_id]
        log.info("media ensure requested=%d ready=%d", len(request.items), len(ready_ids))
        return {"ready": ready_ids}

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

    @router.post("/mashups/{media_id}", status_code=202)
    async def upload_mashup(media_id: str, file: UploadFile = File(...)):
        if not service.mashups.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        if service.mashups.is_ready(media_id) or media_id in service.mashup_jobs:
            raise HTTPException(status_code=409, detail="Mashup already exists")
        if service.mashups.total_size() >= service.settings.mashup_total_limit:
            raise HTTPException(status_code=507, detail="Mashup storage is full")
        try:
            part = await service.mashups.save_upload(media_id, file)
        except MashupError as exc:
            raise HTTPException(status_code=413, detail=str(exc))
        service.mashup_jobs.submit(media_id, part)
        return {"media_id": media_id, "status": "processing"}

    @router.get("/mashups/{media_id}/status")
    async def mashup_status(media_id: str):
        if not service.mashups.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        state = service.mashup_jobs.status(media_id)
        if state is None:
            raise HTTPException(status_code=404, detail="Mashup not found")
        return state.as_dict()

    @router.get("/mashups/{media_id}/cover")
    async def mashup_cover(media_id: str):
        if not service.mashups.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        path = service.mashups.cover_path(media_id)
        if not path.is_file():
            raise HTTPException(status_code=404, detail="Cover not found")
        return FileResponse(
            path,
            media_type="image/jpeg",
            headers={"Cache-Control": "public, max-age=86400"},
        )

    @router.get("/mashups/{media_id}")
    async def mashup_media(media_id: str, request: Request):
        if not service.mashups.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        path = service.mashups.path(media_id)
        if not path.is_file():
            raise HTTPException(status_code=404, detail="Mashup not found")
        return _file_response(path, request)

    @router.delete("/mashups/{media_id}")
    async def delete_mashup(media_id: str):
        if not service.mashups.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        service.mashups.delete(media_id)
        service.mashup_jobs.discard(media_id)
        return {"ok": True}

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

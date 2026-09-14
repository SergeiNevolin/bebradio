import asyncio
import logging
import mimetypes
import re
import secrets
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import APIRouter, FastAPI, File, HTTPException, Request, UploadFile
from fastapi.responses import Response, StreamingResponse

from uploads import UploadError
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

    app = FastAPI(title="bebradio music service", lifespan=lifespan)
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

    @router.post("/music/download")
    async def download(request: DownloadRequest):
        if not service.storage.valid_id(request.media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        ready = await service.ensure(request.url, request.media_id)
        if not ready:
            raise HTTPException(status_code=502, detail="Media download failed")
        return {
            "media_id": request.media_id,
            "filename": service.storage.audio_filename(request.media_id),
            "status": "ready",
        }

    @router.post("/music/ensure")
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

    @router.post("/music/references")
    async def references(request: ReferencesRequest):
        service.set_references(request.media_ids)
        return {"count": len(service.storage.referenced)}

    @router.get("/music/{media_id}/captions")
    async def media_captions(media_id: str, lang: str = ""):
        if not service.storage.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        return await service.captions_from_disk(media_id, lang)

    @router.get("/music/{media_id}")
    async def media(media_id: str, request: Request):
        if not service.storage.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        size = await asyncio.to_thread(service.storage.audio_size, media_id)
        if size is None:
            raise HTTPException(status_code=404, detail="Track not found")

        def _open(start: int | None, end: int | None):
            result = service.storage.audio_stream(media_id, start, end)
            if result is None:
                raise HTTPException(status_code=404, detail="Track not found")
            return result[0]

        return _stream_response(service.storage.audio_filename(media_id), size, request, _open)

    @router.post("/uploads", status_code=202)
    async def upload_track(file: UploadFile = File(...)):
        """Accept a user upload, minting both its track id and media id.

        Identity always arrives with the data: callers never invent ids.
        """
        media_id = track_id = ""
        for _ in range(3):
            candidate_media = secrets.token_hex(8)
            candidate_track = secrets.token_hex(4)
            if not service.uploads.exists(candidate_media) and candidate_media not in service.upload_jobs:
                media_id, track_id = candidate_media, candidate_track
                break
        if not media_id:
            raise HTTPException(status_code=503, detail="Could not allocate track id")
        if service.uploads.total_size() >= service.settings.mashup_total_limit:
            raise HTTPException(status_code=507, detail="Upload storage is full")
        try:
            part = await service.uploads.save_upload(media_id, file)
        except UploadError as exc:
            raise HTTPException(status_code=413, detail=str(exc))
        service.upload_jobs.submit(media_id, part)
        return {"id": track_id, "media_id": media_id, "status": "processing"}

    @router.get("/uploads/{media_id}/status")
    async def upload_status(media_id: str):
        if not service.uploads.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        state = service.upload_jobs.status(media_id)
        if state is None:
            raise HTTPException(status_code=404, detail="Track not found")
        return state.as_dict()

    @router.get("/uploads/{media_id}/cover")
    async def track_cover(media_id: str):
        if not service.uploads.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        data = await asyncio.to_thread(service.uploads.read_cover, media_id)
        if data is None:
            raise HTTPException(status_code=404, detail="Cover not found")
        return Response(
            data,
            media_type="image/jpeg",
            headers={"Cache-Control": "public, max-age=86400"},
        )

    @router.put("/uploads/{media_id}/cover")
    async def upload_track_cover(media_id: str, file: UploadFile = File(...)):
        if not service.uploads.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        try:
            part = await service.uploads.save_cover_upload(media_id, file)
            await asyncio.to_thread(service.uploads.process_cover, media_id, part)
        except UploadError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
        return {"ok": True, "has_cover": True}

    @router.get("/uploads/{media_id}")
    async def track_audio(media_id: str, request: Request):
        if not service.uploads.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        size = await asyncio.to_thread(service.uploads.audio_size, media_id)
        if size is None:
            raise HTTPException(status_code=404, detail="Track not found")

        def _open(start: int | None, end: int | None):
            result = service.uploads.audio_stream(media_id, start, end)
            if result is None:
                raise HTTPException(status_code=404, detail="Track not found")
            return result[0]

        return _stream_response(f"{media_id}.m4a", size, request, _open)

    @router.delete("/uploads/{media_id}")
    async def delete_track(media_id: str):
        if not service.uploads.valid_id(media_id):
            raise HTTPException(status_code=400, detail="Invalid media ID")
        service.uploads.delete(media_id)
        service.upload_jobs.discard(media_id)
        return {"ok": True}

    app.include_router(router)
    return app


def _stream_response(filename: str, size: int, request: Request, opener):
    """Build a full or Range (206) audio response streamed from object storage.

    ``opener(start, end)`` returns an iterator of bytes; ``(None, None)`` means
    the whole object.
    """
    media_type = (
        _AUDIO_MIME.get(Path(filename).suffix.lower())
        or mimetypes.guess_type(filename)[0]
        or "application/octet-stream"
    )
    common_headers = {"Accept-Ranges": "bytes", "Cache-Control": "public, max-age=3600"}
    range_header = request.headers.get("range")
    if not range_header:
        return StreamingResponse(
            opener(None, None), media_type=media_type,
            headers={**common_headers, "Content-Length": str(size)},
        )

    try:
        start_text, end_text = range_header.replace("bytes=", "").split("-", 1)
        start = int(start_text) if start_text else 0
        end = int(end_text) if end_text else size - 1
    except ValueError:
        return StreamingResponse(
            opener(None, None), media_type=media_type,
            headers={**common_headers, "Content-Length": str(size)},
        )

    if start < 0 or end < start or end >= size:
        return Response(status_code=416, headers={"Content-Range": f"bytes */{size}"})

    headers = {
        **common_headers,
        "Content-Range": f"bytes {start}-{end}/{size}",
        "Content-Length": str(end - start + 1),
    }
    return StreamingResponse(opener(start, end), status_code=206, media_type=media_type, headers=headers)





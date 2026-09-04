import asyncio
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from autoadvance import run_auto_advance
from db import init_db, close_db
from media import init_media_dir, cleanup_expired_media
from routes import api_router, ws_router

MEDIA_CLEANUP_INTERVAL = 3600  # 1 hour


async def run_media_cleanup() -> None:
    """Periodically remove expired media files."""
    while True:
        await asyncio.sleep(MEDIA_CLEANUP_INTERVAL)
        try:
            await cleanup_expired_media()
        except asyncio.CancelledError:
            raise
        except Exception:
            pass


@asynccontextmanager
async def lifespan(app: FastAPI):
    test_db = os.getenv("TESTING")
    if test_db:
        await init_db("sqlite+aiosqlite:///:memory:")
    else:
        await init_db()

    init_media_dir()

    advance_task = asyncio.create_task(run_auto_advance())
    cleanup_task = asyncio.create_task(run_media_cleanup())
    try:
        yield
    finally:
        advance_task.cancel()
        cleanup_task.cancel()
        try:
            await advance_task
        except asyncio.CancelledError:
            pass
        try:
            await cleanup_task
        except asyncio.CancelledError:
            pass
        await close_db()


app = FastAPI(title="bebradio", lifespan=lifespan)

ALLOWED_ORIGINS = os.getenv("CORS_ORIGINS", "http://localhost:3000").split(",")

app.add_middleware(
    CORSMiddleware,
    allow_origins=ALLOWED_ORIGINS,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(api_router)
app.include_router(ws_router)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)

import os
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from db import init_db, close_db
from routes import api_router, ws_router


@asynccontextmanager
async def lifespan(app: FastAPI):
    test_db = os.getenv("TESTING")
    if test_db:
        await init_db("sqlite+aiosqlite:///:memory:")
    else:
        await init_db()
    yield
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

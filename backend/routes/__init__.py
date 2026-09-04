from fastapi import APIRouter

from routes.auth import router as auth_router
from routes.rooms import router as rooms_router
from routes.search import router as search_router
from routes.profiles import router as profiles_router
from routes.stream import router as stream_router
from routes.ws import router as ws_router

api_router = APIRouter()
api_router.include_router(auth_router)
api_router.include_router(rooms_router)
api_router.include_router(search_router)
api_router.include_router(profiles_router)
api_router.include_router(stream_router)

__all__ = ["api_router", "ws_router"]

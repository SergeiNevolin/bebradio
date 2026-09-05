import asyncio

from config import Settings
from providers.youtube import YouTubeProvider
from storage import MediaStorage


class MediaService:
    def __init__(self, settings: Settings) -> None:
        self.settings = settings
        self.youtube = YouTubeProvider(settings)
        self.storage = MediaStorage(settings, self.youtube.download)

    def start(self) -> None:
        self.storage.init()

    async def search(self, query: str, limit: int) -> list[dict]:
        return await asyncio.to_thread(self.youtube.search, query, limit)

    async def resolve(self, url: str) -> dict | None:
        return await asyncio.to_thread(self.youtube.resolve, url)

    async def related(self, source_url: str, limit: int) -> list[str]:
        return await asyncio.to_thread(self.youtube.related, source_url, limit)

    async def captions(self, source_url: str, lang: str) -> dict:
        return await asyncio.to_thread(self.youtube.captions, source_url, lang)

    async def ensure(self, source_url: str, media_id: str) -> bool:
        return await self.storage.ensure(source_url, media_id)

    def set_references(self, media_ids: list[str]) -> None:
        self.storage.set_references(media_ids)

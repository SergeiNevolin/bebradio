from dataclasses import dataclass
import os
from pathlib import Path


@dataclass(frozen=True)
class Settings:
    media_dir: Path
    media_ttl: int
    media_max_size: int
    max_downloads: int
    bgutil_base_url: str
    cleanup_interval: int = 3600

    @classmethod
    def from_env(cls) -> "Settings":
        return cls(
            media_dir=Path(os.getenv("MEDIA_DIR", "/app/media/tracks")),
            media_ttl=int(os.getenv("MEDIA_TTL", str(4 * 3600))),
            media_max_size=int(os.getenv("MEDIA_MAX_SIZE", str(10 * 1024 * 1024 * 1024))),
            max_downloads=int(os.getenv("MAX_DOWNLOADS", "3")),
            bgutil_base_url=os.getenv("BGUTIL_BASE_URL", "http://127.0.0.1:4416"),
        )

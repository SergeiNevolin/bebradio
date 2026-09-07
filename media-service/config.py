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
    mashup_dir: Path = Path("/app/media/mashups")
    mashup_max_size: int = 60 * 1024 * 1024
    mashup_max_duration: int = 900
    mashup_total_limit: int = 20 * 1024 * 1024 * 1024
    mashup_max_jobs: int = 2
    mashup_cover_max_size: int = 5 * 1024 * 1024

    @classmethod
    def from_env(cls) -> "Settings":
        return cls(
            media_dir=Path(os.getenv("MEDIA_DIR", "/app/media/tracks")),
            media_ttl=int(os.getenv("MEDIA_TTL", str(4 * 3600))),
            media_max_size=int(os.getenv("MEDIA_MAX_SIZE", str(10 * 1024 * 1024 * 1024))),
            max_downloads=int(os.getenv("MAX_DOWNLOADS", "3")),
            bgutil_base_url=os.getenv("BGUTIL_BASE_URL", "http://127.0.0.1:4416"),
            mashup_dir=Path(os.getenv("MASHUP_DIR", "/app/media/mashups")),
            mashup_max_size=int(os.getenv("MASHUP_MAX_SIZE", str(60 * 1024 * 1024))),
            mashup_max_duration=int(os.getenv("MASHUP_MAX_DURATION", "900")),
            mashup_total_limit=int(os.getenv("MASHUP_TOTAL_LIMIT", str(20 * 1024 * 1024 * 1024))),
            mashup_max_jobs=int(os.getenv("MASHUP_MAX_JOBS", "2")),
            mashup_cover_max_size=int(os.getenv("MASHUP_COVER_MAX_SIZE", str(5 * 1024 * 1024))),
        )

from dataclasses import dataclass
import os
from pathlib import Path


def _getenv(primary: str, fallback_key: str, default: str) -> str:
    return os.getenv(primary) or os.getenv(fallback_key) or default


@dataclass(frozen=True)
class Settings:
    s3_endpoint: str
    s3_access_key: str
    s3_secret_key: str
    s3_bucket: str
    s3_region: str
    work_dir: Path
    media_ttl: int
    media_max_size: int
    max_downloads: int
    bgutil_base_url: str
    cleanup_interval: int = 3600
    mashup_max_size: int = 60 * 1024 * 1024
    mashup_max_duration: int = 900
    mashup_total_limit: int = 20 * 1024 * 1024 * 1024
    mashup_max_jobs: int = 2
    mashup_cover_max_size: int = 5 * 1024 * 1024

    @classmethod
    def from_env(cls) -> "Settings":
        return cls(
            s3_endpoint=os.getenv("S3_ENDPOINT", "http://localhost:9000"),
            s3_access_key=os.getenv("S3_ACCESS_KEY", "minioadmin"),
            s3_secret_key=os.getenv("S3_SECRET_KEY", "minioadmin"),
            s3_bucket=os.getenv("S3_BUCKET", "music"),
            s3_region=os.getenv("S3_REGION", "us-east-1"),
            work_dir=Path(os.getenv("WORK_DIR", "/tmp/music-work")),
            media_ttl=int(_getenv("MUSIC_TTL", "MEDIA_TTL", str(4 * 3600))),
            media_max_size=int(_getenv("MUSIC_MAX_SIZE", "MEDIA_MAX_SIZE", str(10 * 1024 * 1024 * 1024))),
            max_downloads=int(os.getenv("MAX_DOWNLOADS", "3")),
            bgutil_base_url=os.getenv("BGUTIL_BASE_URL", "http://127.0.0.1:4416"),
            mashup_max_size=int(os.getenv("MASHUP_MAX_SIZE", str(60 * 1024 * 1024))),
            mashup_max_duration=int(os.getenv("MASHUP_MAX_DURATION", "900")),
            mashup_total_limit=int(os.getenv("MASHUP_TOTAL_LIMIT", str(20 * 1024 * 1024 * 1024))),
            mashup_max_jobs=int(os.getenv("MASHUP_MAX_JOBS", "2")),
            mashup_cover_max_size=int(os.getenv("MASHUP_COVER_MAX_SIZE", str(5 * 1024 * 1024))),
        )

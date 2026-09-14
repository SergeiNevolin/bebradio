import sys
import time
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parents[1]))


class FakeS3:
    """In-memory stand-in for :class:`s3store.S3Store`."""

    def __init__(self) -> None:
        self.objects: dict[str, dict] = {}

    def ensure_bucket(self) -> None:
        return None

    def exists(self, key: str) -> bool:
        return key in self.objects

    def size(self, key: str):
        obj = self.objects.get(key)
        return len(obj["data"]) if obj else None

    def put_file(self, key: str, path: Path) -> None:
        self.put_bytes(key, Path(path).read_bytes())

    def put_bytes(self, key: str, data: bytes) -> None:
        self.objects[key] = {"data": bytes(data), "last_modified": time.time()}

    def copy(self, src_key: str, dst_key: str) -> None:
        obj = self.objects[src_key]
        self.objects[dst_key] = {"data": obj["data"], "last_modified": obj["last_modified"]}

    def get_bytes(self, key: str):
        obj = self.objects.get(key)
        return obj["data"] if obj else None

    def get_range(self, key: str, start: int, end: int):
        obj = self.objects.get(key)
        if obj is None:
            return None
        return obj["data"][start : end + 1], len(obj["data"])

    def stream(self, key: str, start=None, end=None):
        obj = self.objects.get(key)
        if obj is None:
            return None
        data = obj["data"]
        total = len(data)
        if start is None:
            chunk = data
        else:
            chunk = data[start : (end + 1) if end is not None else None]

        def _iter():
            yield chunk

        return _iter(), total

    def delete(self, key: str) -> None:
        self.objects.pop(key, None)

    def list(self, prefix: str):
        from s3store import S3Object

        return sorted(
            (
                S3Object(key=key, size=len(obj["data"]), last_modified=obj["last_modified"])
                for key, obj in self.objects.items()
                if key.startswith(prefix)
            ),
            key=lambda item: item.key,
        )

    def backdate(self, key: str, age_seconds: float) -> None:
        self.objects[key]["last_modified"] = time.time() - age_seconds


@pytest.fixture
def fake_s3():
    return FakeS3()


@pytest.fixture
def settings(tmp_path):
    from config import Settings

    return Settings(
        s3_endpoint="http://minio:9000",
        s3_access_key="test",
        s3_secret_key="test",
        s3_bucket="music",
        s3_region="us-east-1",
        work_dir=tmp_path / "work",
        media_ttl=3600,
        media_max_size=1024,
        max_downloads=2,
        bgutil_base_url="http://provider:4416",
        mashup_max_size=1024,
        mashup_max_duration=10,
        mashup_total_limit=4096,
        mashup_max_jobs=2,
        mashup_cover_max_size=512,
    )

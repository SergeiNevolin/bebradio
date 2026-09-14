"""Persistent object storage for all music files, backed by MinIO (S3 API).

Layout inside the bucket::

    tracks/<media_id>.<ext>                 - cached YouTube audio (.m4a/.webm/...)
    tracks/<media_id>[.auto].<lang>.vtt     - subtitles next to the audio
    mashups/<media_id>.m4a                  - transcoded user uploads
    mashups/<media_id>.jpg                  - mashup covers

Local disk is only scratch space (yt-dlp downloads, ffmpeg transcoding);
everything durable lives in the bucket. ``S3Store`` is the boto3
implementation used in production; tests inject an in-memory fake exposing
the same methods.
"""

from __future__ import annotations

import logging
import re
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Iterator, Optional

log = logging.getLogger(__name__)

_CONTENT_RANGE_RE = re.compile(r"bytes \d+-\d+/(\d+)")


@dataclass
class S3Object:
    key: str
    size: int
    last_modified: float  # epoch seconds


class S3Store:
    def __init__(
        self,
        endpoint: str,
        access_key: str,
        secret_key: str,
        bucket: str,
        region: str = "us-east-1",
        client=None,
    ) -> None:
        self.bucket = bucket
        if client is not None:
            self._client = client
            return
        import boto3
        from botocore.config import Config as BotoConfig

        self._client = boto3.client(
            "s3",
            endpoint_url=endpoint,
            aws_access_key_id=access_key,
            aws_secret_access_key=secret_key,
            region_name=region,
            use_ssl=endpoint.startswith("https://"),
            config=BotoConfig(s3={"addressing_style": "path"}),
        )

    @classmethod
    def from_settings(cls, settings) -> "S3Store":
        return cls(
            endpoint=settings.s3_endpoint,
            access_key=settings.s3_access_key,
            secret_key=settings.s3_secret_key,
            bucket=settings.s3_bucket,
            region=settings.s3_region,
        )

    def ensure_bucket(self, retries: int = 30) -> None:
        """Create the bucket if missing; retry while MinIO is still booting."""
        from botocore.exceptions import BotoCoreError, ClientError

        for attempt in range(retries):
            try:
                self._client.head_bucket(Bucket=self.bucket)
                return
            except ClientError as exc:
                if exc.response.get("Error", {}).get("Code") not in ("404", "NoSuchBucket"):
                    raise
                try:
                    self._client.create_bucket(Bucket=self.bucket)
                    return
                except ClientError:
                    raise
            except (BotoCoreError, OSError) as exc:
                log.info("s3 not ready (attempt %d/%d): %s", attempt + 1, retries, exc)
            time.sleep(1)
        raise RuntimeError(f"s3 bucket {self.bucket!r} unavailable")

    @staticmethod
    def _missing(exc: Exception) -> bool:
        from botocore.exceptions import ClientError

        return (
            isinstance(exc, ClientError)
            and exc.response.get("Error", {}).get("Code") in ("404", "NoSuchKey", "NoSuchBucket")
        )

    def exists(self, key: str) -> bool:
        return self.size(key) is not None

    def size(self, key: str) -> Optional[int]:
        try:
            return self._client.head_object(Bucket=self.bucket, Key=key)["ContentLength"]
        except Exception as exc:  # noqa: BLE001 - missing key maps to None
            if self._missing(exc):
                return None
            raise

    def put_file(self, key: str, path: Path) -> None:
        self._client.upload_file(str(path), self.bucket, key)

    def put_bytes(self, key: str, data: bytes) -> None:
        self._client.put_object(Bucket=self.bucket, Key=key, Body=data)

    def copy(self, src_key: str, dst_key: str) -> None:
        self._client.copy_object(
            CopySource={"Bucket": self.bucket, "Key": src_key},
            Bucket=self.bucket,
            Key=dst_key,
        )

    def get_bytes(self, key: str) -> Optional[bytes]:
        try:
            return self._client.get_object(Bucket=self.bucket, Key=key)["Body"].read()
        except Exception as exc:  # noqa: BLE001 - missing key maps to None
            if self._missing(exc):
                return None
            raise

    def get_range(self, key: str, start: int, end: int) -> tuple[bytes, int] | None:
        """Return ``(slice, total_size)`` for ``bytes=start-end`` or None if missing."""
        try:
            resp = self._client.get_object(Bucket=self.bucket, Key=key, Range=f"bytes={start}-{end}")
        except Exception as exc:  # noqa: BLE001 - missing key maps to None
            if self._missing(exc):
                return None
            raise
        total = resp.get("ContentLength", 0)
        match = _CONTENT_RANGE_RE.search(resp.get("ContentRange", ""))
        if match:
            total = int(match.group(1))
        return resp["Body"].read(), total

    def stream(
        self, key: str, start: int | None = None, end: int | None = None
    ) -> tuple[Iterator[bytes], int] | None:
        """Yield the object (or a byte range) without loading it fully into RAM.

        Returns ``(chunks, total_size)`` or None when the key does not exist.
        """
        kwargs: dict = {"Bucket": self.bucket, "Key": key}
        if start is not None:
            kwargs["Range"] = f"bytes={start}-{end if end is not None else ''}"
        try:
            resp = self._client.get_object(**kwargs)
        except Exception as exc:  # noqa: BLE001 - missing key maps to None
            if self._missing(exc):
                return None
            raise
        total = resp.get("ContentLength", 0)
        match = _CONTENT_RANGE_RE.search(resp.get("ContentRange", ""))
        if match:
            total = int(match.group(1))
        body = resp["Body"]
        return (body.iter_chunks(chunk_size=64 * 1024), total)

    def delete(self, key: str) -> None:
        try:
            self._client.delete_object(Bucket=self.bucket, Key=key)
        except Exception as exc:  # noqa: BLE001 - best effort
            if not self._missing(exc):
                raise

    def list(self, prefix: str) -> list[S3Object]:
        paginator = self._client.get_paginator("list_objects_v2")
        out: list[S3Object] = []
        for page in paginator.paginate(Bucket=self.bucket, Prefix=prefix):
            for item in page.get("Contents", []):
                out.append(
                    S3Object(
                        key=item["Key"],
                        size=item.get("Size", 0),
                        last_modified=item["LastModified"].timestamp(),
                    )
                )
        return out

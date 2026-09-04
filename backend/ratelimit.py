import time
from collections import defaultdict
from threading import Lock
from typing import Optional

from fastapi import Request
from fastapi.responses import JSONResponse


class SlidingWindowLimiter:
    """In-memory sliding window rate limiter keyed by client IP."""

    def __init__(self, max_requests: int, window_seconds: int):
        self.max_requests = max_requests
        self.window_seconds = window_seconds
        self._hits: dict[str, list[float]] = defaultdict(list)
        self._lock = Lock()

    def _clean(self, key: str, now: float) -> list[float]:
        cutoff = now - self.window_seconds
        times = self._hits[key]
        self._hits[key] = [t for t in times if t > cutoff]
        return self._hits[key]

    def allow(self, key: str) -> bool:
        now = time.time()
        with self._lock:
            times = self._clean(key, now)
            if len(times) >= self.max_requests:
                return False
            times.append(now)
            return True

    def remaining(self, key: str) -> int:
        now = time.time()
        with self._lock:
            times = self._clean(key, now)
            return max(0, self.max_requests - len(times))


def _client_ip(request: Request) -> str:
    forwarded = request.headers.get("x-forwarded-for")
    if forwarded:
        return forwarded.split(",")[0].strip()
    return request.client.host if request.client else "unknown"


def rate_limit(
    max_requests: int,
    window_seconds: int,
    status_code: int = 429,
    message: str = "Too many requests",
):
    """Return a FastAPI dependency that enforces a per-IP rate limit."""
    limiter = SlidingWindowLimiter(max_requests, window_seconds)

    async def _dep(request: Request):
        ip = _client_ip(request)
        if not limiter.allow(ip):
            retry_after = window_seconds
            return JSONResponse(
                status_code=status_code,
                content={"error": message, "retry_after": retry_after},
                headers={"Retry-After": str(retry_after)},
            )
        return None

    return _dep

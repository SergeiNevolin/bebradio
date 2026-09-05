"""HTTP client for the external media service."""

import httpx

from config import MEDIA_SERVICE_URL

class MediaServiceError(RuntimeError):
    pass


async def _request(method: str, path: str, **kwargs) -> httpx.Response:
    try:
        async with httpx.AsyncClient(base_url=MEDIA_SERVICE_URL, timeout=kwargs.pop("timeout", 30.0)) as client:
            response = await client.request(method, path, **kwargs)
    except httpx.HTTPError as exc:
        raise MediaServiceError("Media service is unavailable") from exc
    if response.is_error:
        raise MediaServiceError(response.text[:500])
    return response


async def search_youtube(query: str, limit: int = 5) -> list[dict]:
    response = await _request("POST", "/v1/search", json={"query": query, "limit": limit}, timeout=35.0)
    return response.json()


async def fetch_track(url: str) -> dict | None:
    try:
        response = await _request("POST", "/v1/resolve", json={"url": url}, timeout=75.0)
    except MediaServiceError:
        return None
    data = response.json()
    return data if data.get("media_id") else None


async def download_track(source_url: str, media_id: str) -> dict | None:
    try:
        response = await _request(
            "POST", "/v1/media/download",
            json={"url": source_url, "media_id": media_id}, timeout=150.0,
        )
    except MediaServiceError:
        return None
    return response.json()


async def ensure_media(items: list[dict]) -> set[str]:
    try:
        response = await _request("POST", "/v1/media/ensure", json={"items": items}, timeout=150.0)
    except MediaServiceError:
        return set()
    return set(response.json().get("ready", []))


async def fetch_related(source_url: str, limit: int) -> list[str]:
    try:
        response = await _request(
            "POST", "/v1/radio/related",
            json={"source_url": source_url, "limit": limit}, timeout=55.0,
        )
    except MediaServiceError:
        return []
    return response.json()


async def fetch_subtitles(source_url: str, lang: str = "") -> dict:
    try:
        response = await _request(
            "GET", "/v1/captions", params={"source_url": source_url, "lang": lang}, timeout=35.0,
        )
    except MediaServiceError:
        return {"lang": "", "auto": False, "cues": []}
    return response.json()


async def media_content(media_id: str, range_header: str | None = None) -> httpx.Response:
    headers = {"Range": range_header} if range_header else {}
    return await _request("GET", f"/v1/media/{media_id}", headers=headers, timeout=60.0)


async def update_media_references(media_ids: list[str]) -> None:
    try:
        await _request("POST", "/v1/media/references", json={"media_ids": media_ids}, timeout=10.0)
    except MediaServiceError:
        return

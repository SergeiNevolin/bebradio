from pydantic import BaseModel, Field


class QueryRequest(BaseModel):
    query: str
    limit: int = Field(default=5, ge=1, le=20)


class ResolveRequest(BaseModel):
    url: str


class DownloadRequest(BaseModel):
    url: str
    media_id: str


class EnsureItem(BaseModel):
    source_url: str
    media_id: str


class EnsureRequest(BaseModel):
    items: list[EnsureItem] = Field(default_factory=list, max_length=2)


class RelatedRequest(BaseModel):
    source_url: str
    limit: int = Field(default=12, ge=1, le=40)


class ReferencesRequest(BaseModel):
    media_ids: list[str] = Field(default_factory=list)

from pydantic import BaseModel, field_validator
from typing import Optional


class RegisterRequest(BaseModel):
    email: str
    username: str
    password: str

    @field_validator("email")
    @classmethod
    def validate_email(cls, v: str) -> str:
        if "@" not in v or "." not in v:
            raise ValueError("Invalid email")
        return v.lower().strip()

    @field_validator("username")
    @classmethod
    def validate_username(cls, v: str) -> str:
        v = v.strip()
        if len(v) < 2 or len(v) > 30:
            raise ValueError("Username must be 2-30 characters")
        return v


class LoginRequest(BaseModel):
    email: str
    password: str

    @field_validator("email")
    @classmethod
    def validate_email(cls, v: str) -> str:
        return v.lower().strip()


class TokenResponse(BaseModel):
    token: str
    user: dict


class CreateRoomRequest(BaseModel):
    name: str = "My Room"
    password: Optional[str] = None


class JoinRequest(BaseModel):
    username: str = "Anonymous"
    password: Optional[str] = None


class AddTrackRequest(BaseModel):
    url: str
    added_by: str = "Anonymous"


class SearchRequest(BaseModel):
    query: str
    limit: int = 5


class PlaybackRequest(BaseModel):
    action: Optional[str] = None
    position: Optional[float] = None
    index: Optional[int] = None


class RoomSettingsRequest(BaseModel):
    allow_anonymous_add: Optional[bool] = None
    is_private: Optional[bool] = None
    auto_radio: Optional[bool] = None
    # ``None`` (field unset) means "leave unchanged"; an empty string means
    # "remove the password"; a non-empty string sets a new password.
    password: Optional[str] = None


class UpdateProfileRequest(BaseModel):
    bio: Optional[str] = None
    avatar_url: Optional[str] = None

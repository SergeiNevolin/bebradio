import os

DATABASE_URL = os.getenv(
    "DATABASE_URL",
    "postgresql+asyncpg://postgres:postgres@localhost:5432/bebradio",
)

SECRET_KEY = os.getenv("SECRET_KEY", "bebradio-secret-key-change-in-production")
JWT_ALGORITHM = "HS256"
JWT_EXPIRE_HOURS = 72

MAX_CHAT_MESSAGES = 100

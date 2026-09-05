# bebradio Go Backend

High-performance Go rewrite of the bebradio backend.

## Architecture

Clean Architecture with dependency inversion:

```
cmd/server/          - Entry point, DI, graceful shutdown
internal/
  config/            - Env-based configuration
  domain/
    entity/          - Pure domain models
    repository/      - Port interfaces
  usecase/           - Business logic
  delivery/
    http/            - REST API handlers (chi)
    ws/              - WebSocket handler + ConnectionManager
  infrastructure/
    postgres/        - Repository implementations
    media/           - Media service HTTP client
    auth/            - JWT + bcrypt
    worker/          - Background workers
  pkg/
    id/              - ID generation
    ratelimit/       - Sliding window rate limiter
migrations/          - SQL migrations
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgresql://postgres:postgres@localhost:5432/bebradio` | PostgreSQL connection string |
| `SECRET_KEY` | `bebradio-secret-key-change-in-production` | JWT signing key |
| `MEDIA_SERVICE_URL` | `http://127.0.0.1:8100` | Media service URL |
| `CORS_ORIGINS` | `http://localhost:3000` | Allowed CORS origins (comma-separated) |
| `PORT` | `8000` | Server port |
| `JWT_EXPIRE_HOURS` | `72` | JWT token expiry |
| `MAX_DURATION` | `3600` | Max track duration (seconds) |

## Build & Run

```bash
go build -o server ./cmd/server
./server
```

## Docker

```bash
docker build -t bebradio-backend-go .
docker run -p 8000:8000 bebradio-backend-go
```

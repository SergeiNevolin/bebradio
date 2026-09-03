# syntax=docker/dockerfile:1

# --- Build the server ---------------------------------------------------------
FROM golang:1.27-alpine AS build

WORKDIR /src

# Dependencies are copied and downloaded on their own, so a change to the source
# does not invalidate the module cache layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# A static binary: the runtime image needs no Go toolchain and no libc shim.
# -trimpath keeps build paths out of the binary; -s -w drop the symbol table.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bebradio ./cmd/bebradio


# --- Fetch yt-dlp -------------------------------------------------------------
# yt-dlp is the only component that tracks YouTube's frequent player changes, so
# it is fetched at build time rather than pinned to whatever a distribution
# happens to package. Pass YTDLP_VERSION to pin a specific release.
FROM debian:bookworm-slim AS ytdlp

ARG TARGETARCH
ARG YTDLP_VERSION=latest

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) asset=yt-dlp_linux ;; \
      arm64) asset=yt-dlp_linux_aarch64 ;; \
      *) echo "unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    if [ "${YTDLP_VERSION}" = "latest" ]; then \
      url="https://github.com/yt-dlp/yt-dlp/releases/latest/download/${asset}"; \
    else \
      url="https://github.com/yt-dlp/yt-dlp/releases/download/${YTDLP_VERSION}/${asset}"; \
    fi; \
    curl -fsSL "${url}" -o /usr/local/bin/yt-dlp; \
    chmod +x /usr/local/bin/yt-dlp; \
    /usr/local/bin/yt-dlp --version


# --- Runtime ------------------------------------------------------------------
FROM debian:bookworm-slim

# Node is not optional: YouTube's player requires a JavaScript runtime before
# yt-dlp can derive a stream URL at all.
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates nodejs \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --create-home --uid 10001 bebradio

COPY --from=ytdlp /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp
COPY --from=build /out/bebradio /usr/local/bin/bebradio

USER bebradio
WORKDIR /home/bebradio

EXPOSE 8000

# The binary checks its own health endpoint, so the image needs no HTTP client.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/bebradio", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/bebradio"]

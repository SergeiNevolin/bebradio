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


# --- Fetch Deno ---------------------------------------------------------------
# YouTube's player is JavaScript, and yt-dlp cannot derive a stream URL without a
# runtime to execute it. Deno is the runtime yt-dlp enables by default, and it
# ships as one self-contained binary -- unlike Node, whose Debian package is too
# old for yt-dlp to accept.
FROM debian:bookworm-slim AS deno

ARG TARGETARCH
ARG DENO_VERSION=latest

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl unzip \
 && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) asset=deno-x86_64-unknown-linux-gnu.zip ;; \
      arm64) asset=deno-aarch64-unknown-linux-gnu.zip ;; \
      *) echo "unsupported architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    if [ "${DENO_VERSION}" = "latest" ]; then \
      url="https://github.com/denoland/deno/releases/latest/download/${asset}"; \
    else \
      url="https://github.com/denoland/deno/releases/download/${DENO_VERSION}/${asset}"; \
    fi; \
    curl -fsSL "${url}" -o /tmp/deno.zip; \
    unzip -q /tmp/deno.zip -d /usr/local/bin; \
    chmod +x /usr/local/bin/deno; \
    /usr/local/bin/deno --version


# --- Runtime ------------------------------------------------------------------
FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --create-home --uid 10001 bebradio

COPY --from=deno /usr/local/bin/deno /usr/local/bin/deno
COPY --from=ytdlp /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp
COPY --from=build /out/bebradio /usr/local/bin/bebradio

# yt-dlp only looks for Deno when told to; without a runtime it cannot extract a
# stream URL at all.
ENV YTDLP_JS_RUNTIME=deno

USER bebradio
WORKDIR /home/bebradio

EXPOSE 8000

# The binary checks its own health endpoint, so the image needs no HTTP client.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/bebradio", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/bebradio"]

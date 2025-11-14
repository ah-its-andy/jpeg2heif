# Build stage
FROM golang:1.22-bookworm AS build
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/jpeg2heif ./cmd/worker

# Runtime stage
FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    imagemagick exiftool libheif-examples libheif1 ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# enable HEIC in ImageMagick by ensuring delegates present; bookworm ships HEIF support via libheif

WORKDIR /app
COPY --from=build /out/jpeg2heif /usr/local/bin/jpeg2heif
COPY static ./static
COPY .env.example /app/.env.example

# runtime data dir
VOLUME ["/data"]
ENV WATCH_DIRS=/data/images \
    DB_PATH=/data/tasks.db \
    LOG_LEVEL=INFO \
    POLL_INTERVAL=1 \
    MAX_WORKERS=4 \
    CONVERT_QUALITY=90 \
    HTTP_PORT=8000 \
    PRESERVE_METADATA=true \
    METADATA_STABILITY_DELAY=1 \
    MD5_CHUNK_SIZE=4194304

EXPOSE 8000
ENTRYPOINT ["/usr/local/bin/jpeg2heif"]

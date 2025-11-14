# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

RUN go mod download

# Copy source code
COPY . /build/

# Build with CGO disabled for pure Go binary
RUN mkdir -p /build/bin && go mod download && \
    go mod verify && \
    CGO_ENABLED=0 GOOS=linux go build -o /build/bin/jpeg2heif /build/cmd/jpeg2heif

# Runtime stage
FROM debian:bookworm-slim

# Install external CLI dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libheif-examples \
    libimage-exiftool-perl \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Verify installations
RUN heif-enc --version && exiftool -ver

# Create app user
RUN useradd -m -u 1000 appuser

# Create data directories
RUN mkdir -p /data/watch /data/db && chown -R appuser:appuser /data

# Copy binary from builder
COPY --from=builder /build/bin/jpeg2heif /usr/local/bin/jpeg2heif
RUN chmod +x /usr/local/bin/jpeg2heif

# Copy static files
COPY --chown=appuser:appuser static /app/static

USER appuser
WORKDIR /app

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/api/stats || exit 1

# Run application
CMD ["/usr/local/bin/jpeg2heif"]

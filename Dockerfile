# Use Python 3.11 on Debian base
FROM python:3.11-slim-bookworm

# Install system dependencies for HEIF/HEIC support
# libheif: HEIF/HEIC format support
# libde265: H.265/HEVC decoder for HEIF
# libx265: H.265 encoder
# libexif: EXIF metadata handling
RUN apt-get update && apt-get install -y \
    libheif1 \
    libheif-dev \
    libde265-0 \
    libde265-dev \
    libx265-199 \
    libx265-dev \
    libexif12 \
    libexif-dev \
    libjpeg62-turbo \
    libjpeg62-turbo-dev \
    build-essential \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY app/ ./app/
COPY static/ ./static/

# Create data directory for database and ensure permissions
RUN mkdir -p /data && chmod 777 /data

# Expose HTTP port
EXPOSE 8000

# Set environment variables defaults (can be overridden)
ENV MODE=once
ENV WATCH_DIRS=/data/images
ENV DB_PATH=/data/tasks.db
ENV LOG_LEVEL=INFO
ENV HTTP_PORT=8000
ENV PRESERVE_METADATA=true
ENV CONVERT_QUALITY=90
ENV MAX_WORKERS=4
ENV POLL_INTERVAL=1
ENV METADATA_STABILITY_DELAY=1

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/health')" || exit 1

# Run application
CMD ["python", "-m", "app.main"]

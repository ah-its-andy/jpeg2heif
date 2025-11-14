# jpeg2heif (watcher-only)

A Dockerized Go service that recursively watches one or more directories for new JPEG files and converts them to HEIC, while preserving key metadata (at least DateTimeOriginal). It maintains an index in SQLite to avoid duplicate processing and exposes a minimal web UI for monitoring and controlling a full index rebuild.

## Highlights
- Watcher-only mode: continuously watches directories via fsnotify with recursive registration and provides a manual scan as compensation.
- SQLite index via GORM: files_index and tasks_history track path, MD5, status, errors, and metadata results.
- Duplicate avoidance: checks by file_path and file_md5; skips if already successfully processed.
- HEIC conversion strategy: ImageMagick (magick) for conversion + exiftool to copy metadata into the HEIC file.
- Atomic writes: convert to a temporary file then rename; if the target exists, append a timestamp suffix.
- Web UI + REST API: list files, tasks, stats; trigger full index rebuild and see progress.
- Worker pool: configurable concurrency, streamed MD5 for large files.
- Graceful shutdown: stops new events, lets inflight workers finish (with a short timeout), keeps DB consistent.

## Why ImageMagick + exiftool?
Writing EXIF into HEIC reliably is still best handled by mature system tools. While libvips/bimg can write HEIF, metadata embedding varies by build; using ImageMagick + exiftool is more predictable in containers. This project converts with `magick` and then copies all metadata using `exiftool -TagsFromFile src -all:all dest` to preserve fields. We explicitly verify `DateTimeOriginal` post-write.

## System dependencies (inside container)
- ImageMagick (`magick`) with HEIC support (via `libheif`)
- `exiftool`
- `libheif`

The provided Dockerfile installs these on Debian Bookworm, where ImageMagick has HEIF enabled via `libheif1`.

## Data model
- `files_index`:
  - id (PK)
  - file_path (unique)
  - file_md5 (indexed)
  - status: pending/processing/success/failed
  - last_error (nullable)
  - created_at, updated_at, processed_at
  - metadata_preserved (bool)
  - metadata_summary (text)
- `tasks_history`:
  - id, file_index_id (FK), action (convert), status
  - start_time, end_time, duration_ms, log

Indexes: unique on `file_path`, index on `file_md5`.

## Destination path rule
For a source `/a/b/c/pic.jpg`, the HEIC output goes to `/a/b/heic/pic.heic`. If that path already exists, the service appends a timestamp suffix: `pic_YYYYmmddTHHMMSS.heic`.

## Environment variables
- `WATCH_DIRS` (required): comma-separated absolute directories to watch, e.g. `/data/images,/mnt/shared/photos`.
- `DB_PATH` (default `/data/tasks.db`)
- `LOG_LEVEL` (DEBUG/INFO/WARNING/ERROR; default `INFO`)
- `POLL_INTERVAL` (seconds; default `1`) – reserved, used as a general timing base.
- `MAX_WORKERS` (default `4`)
- `CONVERT_QUALITY` (0–100; default `90`)
- `HTTP_PORT` (default `8000`)
- `PRESERVE_METADATA` (true/false; default `true`)
- `METADATA_STABILITY_DELAY` (seconds; default `1`) – wait between size checks to avoid half-written files.
- `MD5_CHUNK_SIZE` (bytes; default `4194304` = 4 MB)

An example is provided in `.env.example`.

## Build and run (Docker)

### Build image
```
# from repo root
docker build -t jpeg2heif:local .
```

### Run container
```
# Map host /data to container /data to persist DB and access photos
# Example: watch host directory /data/images

docker run --rm -it \
  -e WATCH_DIRS=/data/images \
  -e DB_PATH=/data/tasks.db \
  -e LOG_LEVEL=INFO \
  -e POLL_INTERVAL=1 \
  -e MAX_WORKERS=4 \
  -e CONVERT_QUALITY=90 \
  -e HTTP_PORT=8000 \
  -e PRESERVE_METADATA=true \
  -e METADATA_STABILITY_DELAY=1 \
  -e MD5_CHUNK_SIZE=4194304 \
  -v /data:/data \
  -p 8000:8000 \
  jpeg2heif:local
```

Or use docker-compose:
```
docker compose up --build
```

Then open http://localhost:8000 to view the UI.

## CI/CD: 使用 GitHub Actions 自动构建并推送 Docker 镜像

仓库已包含工作流文件 `.github/workflows/docker-build-push.yml`，在以下事件触发：
- push 到 `main` 分支（自动打 `latest`、分支名、`sha` 标签）
- 推送带有 `v*` 的 tag（自动打版本标签）
- 手动触发（Workflow Dispatch）

推送目标：`docker.io/<你的 DockerHub 用户名>/jpeg2heif`。

在 GitHub 仓库的 Settings → Secrets and variables → Actions 下配置两个 Secrets：
- `DOCKERHUB_USERNAME`：你的 Docker Hub 用户名
- `DOCKERHUB_TOKEN`：Docker Hub 的 Access Token（推荐）或密码

启用后，工作流会使用 Buildx 构建多架构镜像（linux/amd64, linux/arm64），并使用 GHA 缓存加速后续构建。

## REST API summary
- `GET /api/files?status=&limit=&offset=` – list files_index with optional status filter.
- `GET /api/files/{id}` – get file record.
- `GET /api/tasks` – recent tasks history (limit optional, default 100).
- `GET /api/stats` – totals, queue length, metadata preservation rate, watcher state.
- `POST /api/rebuild-index` – rebuild all index, returns `{job_id}`.
- `GET /api/rebuild-status/{job_id}` – rebuild job progress.
- `POST /api/scan-now` – trigger a full scan.

## Rebuild index behavior
- Pauses watcher event enqueueing.
- Wipes `files_index`.
- Recursively scans all `WATCH_DIRS`, computes MD5, reinserts as `pending`, and enqueues for conversion.
- Exposes progress via job id.
- Resumes watcher afterwards.

## Tests
Two tests are provided under `tests/`:
1. Index build + MD5: creates a temp JPEG, triggers indexing, and validates `files_index` MD5.
2. Metadata preservation: creates a temp JPEG, sets `DateTimeOriginal` via exiftool, runs conversion, and verifies the field in the resulting HEIC via exiftool. Skips if required tools are missing.

Run tests (optional, outside container):
```
go test ./...
```
Note: Integration tests require `magick` and `exiftool` available on PATH.

## Known limitations and notes
- HEIC metadata writing depends on ImageMagick + exiftool behavior; specific distro/library versions may change tag writing paths (EXIF vs XMP). We explicitly verify `DateTimeOriginal` where possible.
- Watchers can miss events on some platforms; we include an initial full scan and a `/api/scan-now` endpoint. Consider scheduling periodic scans externally if needed.
- Rebuild is a destructive operation (wipes index) but is idempotent and safe to re-run. Long rebuilds may take time to compute MD5 for large datasets.
- Very large files: MD5 is streamed with configurable chunk size to avoid high memory usage.

## Acceptance demo
1. Start the container with a watched directory mounted.
2. Place a JPEG (`IMG_0001.jpg`) in `/data/images/some/sub/dir` on the host.
3. Observe in the UI that a `success` record appears; the output HEIC is at `/data/heic/IMG_0001.heic` (per rule `/a/b/c -> /a/b/heic`).
4. For a JPEG with `DateTimeOriginal`, confirm the HEIC has the same value (UI shows `metadata_summary` mentioning `DateTimeOriginal preserved`).
5. Click “重建全部索引” to wipe and rebuild; progress will show, and items will be reprocessed.

## License
MIT

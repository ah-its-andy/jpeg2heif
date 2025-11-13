# Project Structure

Complete file tree of the JPEG to HEIC Converter project.

```
jpeg2heic/
├── app/                          # Main application package
│   ├── __init__.py              # Package initialization
│   ├── api.py                   # FastAPI web application, routes, lifespan management
│   ├── config.py                # Environment variable configuration
│   ├── converter.py             # Core image conversion with EXIF metadata handling
│   ├── database.py              # SQLAlchemy models and database operations
│   ├── main.py                  # Application entry point
│   └── watcher.py               # File monitoring (watchdog) and scanning logic
│
├── static/                       # Web frontend
│   └── index.html               # Single-page dashboard with real-time updates
│
├── tests/                        # Test suite
│   ├── __init__.py              # Test package
│   └── test_converter.py        # Unit tests for conversion and metadata
│
├── data/                         # Runtime data (created at runtime, gitignored)
│   ├── images/                  # Input JPEG files
│   ├── heic/                    # Output HEIC files (auto-created)
│   └── db/                      # SQLite database
│
├── .env                         # Local environment variables (gitignored)
├── .env.example                 # Example environment configuration
├── .gitignore                   # Git ignore rules
├── CHANGELOG.md                 # Version history and release notes
├── Dockerfile                   # Docker image definition
├── docker-compose.yml           # Docker Compose configuration
├── LICENSE                      # MIT License
├── QUICKSTART.md                # Quick start guide
├── README.md                    # Comprehensive documentation
├── requirements.txt             # Python dependencies
├── create_samples.py            # Script to generate test JPEG files
├── run_tests.sh                 # Test runner script
├── setup.sh                     # Local development setup script
└── verify_install.py            # Installation verification script
```

## File Descriptions

### Core Application (`app/`)

| File | Lines | Purpose |
|------|-------|---------|
| `api.py` | ~300 | FastAPI app, REST endpoints, lifespan management, request handlers |
| `converter.py` | ~280 | Image conversion logic, EXIF extraction/preservation, path calculation |
| `database.py` | ~180 | SQLAlchemy ORM models, database operations, statistics |
| `watcher.py` | ~180 | watchdog file monitoring, scanning, conversion queue management |
| `config.py` | ~70 | Environment variable parsing and validation |
| `main.py` | ~50 | Entry point, signal handling, uvicorn server startup |

### Frontend (`static/`)

| File | Lines | Purpose |
|------|-------|---------|
| `index.html` | ~450 | Single-page web dashboard with JavaScript for API calls |

### Tests (`tests/`)

| File | Lines | Purpose |
|------|-------|---------|
| `test_converter.py` | ~280 | pytest test cases for conversion, metadata, database |

### Configuration Files

| File | Purpose |
|------|---------|
| `Dockerfile` | Multi-stage Docker build with system dependencies (libheif, etc.) |
| `docker-compose.yml` | Container orchestration with volume mounts and env vars |
| `requirements.txt` | Python package dependencies with pinned versions |
| `.env.example` | Template for environment variables |
| `.gitignore` | Git exclusions (venv, .env, *.db, __pycache__, etc.) |

### Documentation

| File | Lines | Purpose |
|------|-------|---------|
| `README.md` | ~650 | Complete project documentation |
| `QUICKSTART.md` | ~250 | 5-minute quick start guide |
| `CHANGELOG.md` | ~70 | Version history |
| `LICENSE` | ~20 | MIT License |

### Utility Scripts

| File | Purpose |
|------|---------|
| `setup.sh` | Bash script for local development setup |
| `run_tests.sh` | Test runner wrapper |
| `verify_install.py` | Dependency checker and diagnostic tool |
| `create_samples.py` | Generate test JPEG files with EXIF metadata |

## Technology Stack

### Backend
- **Python 3.10+**: Modern Python with type hints
- **FastAPI**: Async web framework for REST API
- **Uvicorn**: ASGI server
- **SQLAlchemy**: ORM for SQLite database
- **watchdog**: Cross-platform file system monitoring

### Image Processing
- **Pillow**: Core image manipulation
- **pillow-heif (0.14.0)**: HEIF/HEIC format support
- **piexif (1.1.3)**: Pure Python EXIF read/write

### System Libraries (Docker)
- **libheif**: HEIF codec implementation
- **libde265**: H.265 decoder
- **libx265**: H.265 encoder
- **libexif**: EXIF metadata support

### Frontend
- **Vanilla JavaScript**: No frameworks, simple fetch API
- **HTML5 + CSS3**: Responsive design

## Data Flow

```
┌─────────────────┐
│  JPEG Files     │
│  in WATCH_DIRS  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  FileScanner    │  (once mode)
│  or             │
│  FileWatcher    │  (watch mode)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  ConversionQueue│
│  (ThreadPool)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  ImageConverter │
│  + Metadata     │
│  Extractor      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  HEIC Files     │
│  in heic/       │
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  Database       │
│  (Task Records) │
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  Web API        │
│  (FastAPI)      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Web Dashboard  │
│  (Browser)      │
└─────────────────┘
```

## Database Schema

```sql
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_type VARCHAR(10) NOT NULL,              -- 'once' or 'watch'
    source_path VARCHAR(512) NOT NULL,
    target_path VARCHAR(512),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending/running/success/failed
    error_message TEXT,
    start_time DATETIME,
    end_time DATETIME,
    duration FLOAT,                               -- seconds
    metadata_preserved BOOLEAN DEFAULT 0,
    metadata_summary TEXT,                        -- JSON or text
    source_datetime VARCHAR(50),                  -- DateTimeOriginal from source
    target_datetime VARCHAR(50),                  -- DateTimeOriginal in target
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/` | Serve web dashboard |
| GET | `/health` | Health check endpoint |
| GET | `/api/tasks` | List tasks (with filters) |
| GET | `/api/tasks/{id}` | Get task details |
| GET | `/api/stats` | Get conversion statistics |
| POST | `/api/scan-now` | Trigger manual scan |

## Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| MODE | `once` | No | `once` or `watch` |
| WATCH_DIRS | - | Yes | Comma-separated paths |
| DB_PATH | `/data/tasks.db` | No | SQLite database path |
| LOG_LEVEL | `INFO` | No | Logging level |
| POLL_INTERVAL | `1` | No | Watch mode interval (s) |
| MAX_WORKERS | `4` | No | Conversion threads |
| CONVERT_QUALITY | `90` | No | HEIC quality (0-100) |
| HTTP_PORT | `8000` | No | Web server port |
| PRESERVE_METADATA | `true` | No | Preserve EXIF metadata |
| METADATA_STABILITY_DELAY | `1` | No | File stability wait (s) |

## Code Metrics

- **Total Python Files**: 10
- **Total Lines of Code**: ~1,500
- **Test Coverage**: Core conversion and database logic
- **Dependencies**: 10 Python packages + 4 system libraries
- **Docker Image Size**: ~400MB (with all dependencies)

## Key Features Implementation

### Metadata Preservation
- `converter.py`: `MetadataExtractor` class extracts EXIF using piexif
- Priority: DateTimeOriginal > DateTime
- GPS, camera info, lens info all extracted
- EXIF bytes injected into HEIC via `img.save(exif=...)`
- Post-conversion verification compares source vs target datetime

### File Watching
- `watcher.py`: watchdog `Observer` with recursive monitoring
- Stability detection: checks file size doesn't change over 3 intervals
- Debouncing: tracks processing files to avoid duplicates
- Callback-based architecture for loose coupling

### Conversion Queue
- `watcher.py`: `ConversionQueue` class wraps `ThreadPoolExecutor`
- Configurable worker count
- Task tracking with active file set
- Graceful shutdown support

### Atomic Writes
- `converter.py`: `tempfile.mkstemp()` creates temp file
- Write to temp, then `shutil.move()` to final path
- Prevents partial file reads

### Path Calculation
- Source: `/a/b/c/pic.jpg`
- Target: `/a/b/heic/pic.heic`
- Auto-create `heic/` directory
- Index appended on conflict: `pic_1.heic`

## Security Considerations

- No authentication (add nginx reverse proxy with auth if needed)
- Database is local SQLite (no network exposure)
- File paths from environment only (no user input for paths)
- All file operations in controlled directories
- Docker container runs as non-root by default

## Performance Characteristics

- **Memory**: ~100-200MB base + ~50MB per concurrent worker
- **CPU**: Highly dependent on quality setting and image size
- **Disk I/O**: Sequential writes, minimal seeks
- **Throughput**: ~10-50 images/minute on modern hardware (depends on size/quality)

## Extensibility Points

1. **Custom path logic**: Modify `get_heic_target_path()` in `converter.py`
2. **Additional metadata**: Add fields in `MetadataExtractor.extract_exif()`
3. **New output formats**: Add format parameter to `ImageConverter`
4. **Webhooks**: Add callback in `process_conversion()` in `api.py`
5. **Custom filters**: Add query parameters in `/api/tasks` endpoint
6. **Additional statistics**: Extend `get_stats()` in `database.py`

## Testing Strategy

- **Unit tests**: converter, metadata extraction, database operations
- **Integration test**: Full conversion with metadata verification (critical)
- **Path tests**: Target path calculation and conflict resolution
- **Mock-free**: Uses real files, real database for accuracy

## Deployment Options

1. **Docker Compose**: Recommended for most users
2. **Docker Run**: For custom orchestration
3. **Kubernetes**: Possible with StatefulSet for DB persistence
4. **Systemd**: For bare-metal Linux deployments
5. **Local**: Development and testing

## Maintenance

### Updating Dependencies

```bash
pip list --outdated
pip install -U package-name
pip freeze > requirements.txt
```

### Logs

- **Docker**: `docker-compose logs -f`
- **Local**: Console output or redirect to file
- **Format**: Timestamp - Logger - Level - Message

### Database Backup

```bash
cp data/tasks.db data/tasks.db.backup
# Or use SQLite backup command
sqlite3 data/tasks.db ".backup data/tasks.db.backup"
```

---

**Total Implementation Time**: ~4-6 hours for experienced developer

**Lines of Code**: ~1,500 (Python) + ~450 (HTML/JS)

**Deliverables**: 25 files (code + docs + configs)

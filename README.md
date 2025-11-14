# JPEG2HEIF - Pure Go Media Converter with Pluggable Architecture

A high-performance, pure Go (CGO_ENABLED=0) media file converter with real-time directory monitoring. Features a pluggable converter architecture that makes it easy to add support for new format conversions.

## Features

- ✅ **Pure Go Implementation**: No CGO dependencies, fully static binary
- 🔌 **Pluggable Converter Architecture**: Easy to extend with new format converters
- 🔍 **Real-time Monitoring**: fsnotify-based recursive directory watching
- 📊 **SQLite Database**: Pure Go SQLite (modernc.org/sqlite) for indexing and history
- 🏷️ **Metadata Preservation**: Maintains EXIF DateTimeOriginal and optional full metadata
- 🔄 **Duplicate Detection**: MD5-based file tracking to avoid redundant conversions
- 🎯 **Smart File Stability**: Waits for file writes to complete before processing
- 🚀 **Concurrent Processing**: Configurable worker pool for parallel conversions
- 🌐 **REST API + Web UI**: Monitor and control conversions via web interface
- 🐳 **Docker Ready**: Multi-stage build with external CLI tools

## Architecture

### Converter Abstraction Layer

The project uses a pluggable converter architecture:

```go
type Converter interface {
    Name() string
    CanConvert(srcPath string, srcMime string) bool
    TargetFormat() string
    Convert(ctx context.Context, srcPath string, dstPath string, opts ConvertOptions) (MetaResult, error)
}
```

**Built-in Converters:**
- `jpeg2heic`: JPEG → HEIC/HEIF conversion with EXIF preservation

**Adding New Converters:**

1. Create a new file in `internal/converter/` (e.g., `png2heic.go`)
2. Implement the `Converter` interface
3. Register it in `init()`:
   ```go
   func init() {
       Register(&PNG2HEICConverter{})
   }
   ```
4. Implement the conversion logic using external CLIs or pure Go libraries

See `internal/converter/jpeg2heic.go` for a complete example.

## Directory Structure

```
/Users/andi/source/github/jpeg2heif/
├── cmd/
│   └── jpeg2heif/
│       └── main.go              # Application entry point
├── internal/
│   ├── converter/               # Converter abstraction layer
│   │   ├── interface.go         # Converter interface definition
│   │   ├── registry.go          # Converter registration
│   │   └── jpeg2heic.go         # JPEG→HEIC implementation
│   ├── db/                      # Database layer
│   │   ├── db.go                # SQLite operations
│   │   └── models.go            # Data models
│   ├── watcher/                 # File system monitoring
│   │   └── watcher.go           # fsnotify-based watcher
│   ├── worker/                  # Task processing
│   │   └── worker.go            # Worker pool
│   ├── api/                     # REST API
│   │   └── api.go               # HTTP handlers
│   └── util/                    # Utilities
│       ├── md5.go               # MD5 calculation
│       └── config.go            # Configuration
├── static/                      # Web UI
│   └── index.html              # Dashboard
├── tests/                       # Test suite
│   ├── converter_test.go        # Converter tests
│   └── db_test.go              # Database tests
├── Dockerfile                   # Multi-stage Docker build
├── docker-compose.yml           # Docker Compose configuration
├── .env.example                # Environment variables template
├── go.mod                      # Go module definition
└── README.md                   # This file
```

## Environment Variables

Create a `.env` file based on `.env.example`:

| Variable | Default | Description |
|----------|---------|-------------|
| `WATCH_DIRS` | `/data/watch` | Comma-separated directories to monitor |
| `DB_PATH` | `/data/jpeg2heif.db` | SQLite database path |
| `HTTP_PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `POLL_INTERVAL` | `300s` | Periodic scan interval |
| `METADATA_STABILITY_DELAY` | `5s` | Wait time for file stability |
| `MAX_WORKERS` | `4` | Concurrent conversion workers |
| `CONVERT_QUALITY` | `85` | Conversion quality (1-100) |
| `PRESERVE_METADATA` | `true` | Preserve full EXIF/XMP metadata |
| `MD5_CHUNK_SIZE` | `8192` | MD5 calculation chunk size |

## External Dependencies

The application requires these external CLI tools in the runtime environment:

### Required Tools

1. **libheif-examples** (heif-enc)
   - Version: 1.16.0+
   - Install: `apt-get install libheif-examples`
   - Purpose: HEIC/HEIF encoding

2. **exiftool**
   - Version: 12.40+
   - Install: `apt-get install libimage-exiftool-perl`
   - Purpose: EXIF metadata reading and writing

### Verification

The application checks for these tools on startup:
```bash
heif-enc --version
exiftool -ver
```

If tools are missing, the application will log warnings but continue (converters requiring them will fail).

## Building

### Local Build (Pure Go)

```bash
# Build static binary
CGO_ENABLED=0 go build -o jpeg2heif ./cmd/jpeg2heif

# Run
./jpeg2heif
```

### Docker Build

```bash
# Build image
docker build -t jpeg2heif:latest .

# Or use docker-compose
docker-compose build
```

The Dockerfile uses a multi-stage build:
1. **Build stage**: golang:1.21-alpine with `CGO_ENABLED=0`
2. **Runtime stage**: debian:bookworm-slim with external CLI tools

## Running

### Docker Compose (Recommended)

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Docker Run

```bash
docker run -d \
  -p 8080:8080 \
  -v /path/to/watch:/data/watch \
  -v /path/to/db:/data/db \
  -e WATCH_DIRS=/data/watch \
  -e DB_PATH=/data/db/jpeg2heif.db \
  --name jpeg2heif \
  jpeg2heif:latest
```

### Local Run

```bash
export WATCH_DIRS=/path/to/watch
export DB_PATH=./jpeg2heif.db
./jpeg2heif
```

## How It Works

### Conversion Flow

1. **File Detection**: fsnotify detects new/modified files
2. **Stability Check**: Waits `METADATA_STABILITY_DELAY` and verifies file size is stable
3. **MD5 Calculation**: Computes file hash for duplicate detection
4. **Database Check**: Queries `files_index` for existing successful conversion
5. **Converter Selection**: Registry selects appropriate converter based on file type
6. **Conversion**: 
   - Creates temporary output file
   - Calls external CLI (e.g., `heif-enc`)
   - Extracts source metadata with `exiftool`
   - Injects metadata into target file
   - Verifies DateTimeOriginal preservation
   - Atomic rename to final location
7. **Database Update**: Records success/failure in `files_index` and `tasks_history`

### Output Path Strategy

Source: `/a/b/c/photo.jpg`
→ Output: `/a/b/heic/photo.heic`

The output is placed in a `heic` subdirectory at the parent's parent level.

### Metadata Preservation

**Always Preserved:**
- EXIF DateTimeOriginal (verified after conversion)

**When PRESERVE_METADATA=true:**
- All EXIF tags
- XMP metadata
- GPS coordinates
- Orientation
- Camera model/settings

**Verification:**
```bash
# Check DateTimeOriginal in converted file
exiftool -DateTimeOriginal /path/to/output.heic

# Compare source and output metadata
exiftool -s -G1 source.jpg > source_meta.txt
exiftool -s -G1 output.heic > output_meta.txt
diff source_meta.txt output_meta.txt
```

## API Endpoints

### Files

- `GET /api/files?status=success&limit=50&offset=0` - List files
- `GET /api/files/{id}` - Get file details

### Tasks

- `GET /api/tasks?limit=100&offset=0` - List task history

### Statistics

- `GET /api/stats` - Get conversion statistics

### Operations

- `POST /api/rebuild-index` - Rebuild file index
  - Body: `{"converter": "jpeg2heic"}` (optional)
  - Returns: `{"job_id": "uuid"}`
- `GET /api/rebuild-status/{job_id}` - Check rebuild status
- `POST /api/scan-now` - Trigger immediate scan

### Converters

- `GET /api/converters` - List registered converters
  - Returns converter capabilities and enabled status

## Web UI

Access the web dashboard at `http://localhost:8080/`

Features:
- File index table with status filtering
- Task history log
- Real-time statistics
- Manual scan trigger
- Index rebuild
- Converter management

## Database Schema

### files_index

```sql
CREATE TABLE files_index (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT NOT NULL UNIQUE,
    file_md5 TEXT NOT NULL,
    status TEXT NOT NULL,
    converter_name TEXT,
    metadata_preserved BOOLEAN DEFAULT 0,
    metadata_summary TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_file_path ON files_index(file_path);
CREATE INDEX idx_file_md5 ON files_index(file_md5);
```

### tasks_history

```sql
CREATE TABLE tasks_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT NOT NULL,
    converter_name TEXT,
    status TEXT NOT NULL,
    error_message TEXT,
    duration_ms INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Testing

### Run Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/converter/
```

### Test Files

- `tests/converter_test.go`: Converter abstraction and jpeg2heic integration
- `tests/db_test.go`: Database operations and indexing

### Integration Testing

The converter tests require external CLIs. Set up a test environment:

```bash
# Install dependencies (Debian/Ubuntu)
apt-get update
apt-get install -y libheif-examples libimage-exiftool-perl

# Run integration tests
go test ./tests/ -v
```

## Known Limitations

1. **External CLI Dependency**: Requires heif-enc and exiftool in PATH
2. **HEIC-Only Output**: Currently only supports HEIC output (extensible via converters)
3. **Single Target per Source**: One source file produces one output file
4. **No Batch Retry**: Failed conversions require manual retry or index rebuild
5. **Limited Format Detection**: Uses file extension primarily (could use libmagic)

## Future Improvements

1. **Additional Converters**:
   - PNG → HEIC
   - MPEG-4 → HEVC
   - RAW → HEIC
   
2. **Pure Go Encoding**: Replace external CLIs with pure Go libraries when available

3. **Advanced Features**:
   - Configurable output directory patterns
   - Multiple output formats per source
   - Automatic retry with backoff
   - Webhooks for conversion events
   - S3/cloud storage support

4. **Performance**:
   - Distributed worker pool
   - GPU acceleration support
   - Chunked file processing for large files

5. **UI Enhancements**:
   - Real-time WebSocket updates
   - Detailed progress bars
   - Thumbnail previews
   - Metadata diff viewer

## Contributing

To add a new converter:

1. Implement the `Converter` interface
2. Add registration in `init()`
3. Add tests in `tests/converter_test.go`
4. Update README with converter details
5. Submit pull request

Example converter template:

```go
package converter

type MyConverter struct{}

func (c *MyConverter) Name() string {
    return "my_converter"
}

func (c *MyConverter) CanConvert(srcPath string, srcMime string) bool {
    // Check if this converter can handle the file
    return strings.HasSuffix(strings.ToLower(srcPath), ".ext")
}

func (c *MyConverter) TargetFormat() string {
    return "output_ext"
}

func (c *MyConverter) Convert(ctx context.Context, srcPath string, dstPath string, opts ConvertOptions) (MetaResult, error) {
    // Implement conversion logic
    return MetaResult{}, nil
}

func init() {
    Register(&MyConverter{})
}
```

## License

MIT License

## Support

For issues and questions, please open a GitHub issue.

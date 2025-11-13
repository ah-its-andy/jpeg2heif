# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-11-13

### Added
- Initial release
- JPEG to HEIC conversion with metadata preservation
- Two operating modes: once (batch) and watch (real-time)
- EXIF metadata preservation including DateTimeOriginal
- GPS coordinates preservation
- Camera make/model/lens information preservation
- FastAPI web interface with real-time task monitoring
- SQLite database for task history and statistics
- RESTful API for task management
- Docker support with pre-configured Dockerfile
- Docker Compose setup for easy deployment
- Multi-threaded conversion with configurable worker pool
- Atomic file writing with temp files
- File stability detection for watch mode
- Automatic filename conflict resolution
- Comprehensive test suite with metadata verification
- Web dashboard with statistics and task filtering
- Manual scan trigger via API
- Health check endpoint
- Graceful shutdown handling
- Extensive documentation and README

### Features
- Configure multiple watch directories via environment variables
- Adjustable HEIC encoding quality (0-100)
- Configurable metadata stability delay
- Metadata preservation verification and reporting
- Task status tracking (pending/running/success/failed)
- Conversion duration tracking
- Error logging with detailed messages
- Real-time statistics (success rate, metadata preservation rate)
- Responsive web interface
- Cross-platform support (Linux, macOS via Docker)

### Known Limitations
- XMP metadata not supported (pillow-heif limitation)
- IPTC metadata not supported
- Timezone information in datetime fields not preserved (EXIF specification)
- ICC color profiles handled automatically but may be converted

## [Unreleased]

### Planned
- XMP metadata support via exiftool integration
- IPTC metadata preservation
- Additional output formats (WebP, AVIF)
- Batch EXIF editing
- Duplicate detection
- Resume interrupted conversions
- S3/cloud storage support
- Email/webhook notifications
- Progress bars for batch operations

# Quick Start Guide

Get up and running with JPEG to HEIC Converter in under 5 minutes!

## 🚀 Fastest Way (Docker Compose)

### 1. Prerequisites
- Docker and Docker Compose installed
- JPEG images to convert

### 2. Setup

```bash
# Clone the repository
git clone <your-repo-url>
cd jpeg2heic

# Create directories
mkdir -p data/images data/db

# Copy your JPEG files to data/images/
cp /path/to/your/photos/*.jpg data/images/

# Optional: Edit configuration
cp .env.example .env
# Edit .env if you want to change defaults
```

### 3. Run

```bash
# Build and start (one-time scan mode)
docker-compose up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### 4. Access

Open http://localhost:8000 in your browser to view the dashboard!

Your converted HEIC files will be in `data/heic/`

---

## 🔄 Watch Mode (Continuous Monitoring)

Edit `docker-compose.yml` and change:

```yaml
environment:
  - MODE=watch  # Change from 'once' to 'watch'
```

Then restart:

```bash
docker-compose restart
```

Now any new JPEG files added to `data/images/` will be automatically converted!

---

## 💻 Local Development (Without Docker)

### 1. Install System Dependencies

**macOS:**
```bash
brew install libheif libde265 x265 libexif
```

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install libheif1 libheif-dev libde265-dev libx265-dev libexif-dev
```

### 2. Setup Python Environment

```bash
# Run setup script
chmod +x setup.sh
./setup.sh

# Or manually:
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

### 3. Configure

```bash
cp .env.example .env

# Edit .env
export MODE=once
export WATCH_DIRS=/path/to/your/images
export DB_PATH=./data/tasks.db
```

### 4. Run

```bash
python -m app.main
```

Visit http://localhost:8000

---

## 📝 Common Configurations

### Multiple Directories

Edit `.env` or `docker-compose.yml`:

```bash
WATCH_DIRS=/data/images,/data/photos,/mnt/camera
```

### High Quality Conversion

```bash
CONVERT_QUALITY=95  # Default is 90
```

### Faster Conversion

```bash
MAX_WORKERS=8  # Default is 4, increase for more CPU cores
CONVERT_QUALITY=80  # Lower quality = faster
```

### Debug Mode

```bash
LOG_LEVEL=DEBUG  # See detailed logs
```

---

## 🧪 Test the Installation

### 1. Generate Sample Images

```bash
python create_samples.py
```

This creates test JPEG files with EXIF metadata in `data/images/test_samples/`

### 2. Verify Installation

```bash
python verify_install.py
```

### 3. Run Tests

```bash
chmod +x run_tests.sh
./run_tests.sh

# Or directly:
pytest tests/ -v
```

---

## 📊 Using the Web Interface

1. **Dashboard**: View statistics (total, success, failed, queue size)
2. **Tasks Table**: See all conversion tasks with status
3. **Filters**: Click tabs to filter by status (All, Running, Success, Failed)
4. **Metadata Info**: Expand task rows to see metadata details
5. **Manual Scan**: Click "Trigger Manual Scan" button to scan on-demand
6. **Auto-Refresh**: Page updates every 3 seconds automatically

---

## 🔍 Check Your Converted Files

### Verify Metadata Preservation

```bash
# Install exiftool (optional)
brew install exiftool  # macOS
apt-get install libimage-exiftool-perl  # Ubuntu

# Check source JPEG
exiftool data/images/photo.jpg | grep DateTimeOriginal

# Check converted HEIC
exiftool data/heic/photo.heic | grep DateTimeOriginal

# Should match!
```

### View in File Explorer

- **macOS**: HEIC files open natively in Preview
- **Windows 10+**: Install HEIF extension from Microsoft Store
- **Linux**: Use tools like `heif-convert` or viewers with libheif support

---

## 🛠 Troubleshooting

### "libheif not found"

**Solution**: Use Docker! The Dockerfile includes all system dependencies.

```bash
docker-compose up -d
```

### "No tasks appearing"

**Check**:
1. Are JPEG files in the correct directory?
2. Is `WATCH_DIRS` pointing to the correct path (container path if using Docker)?
3. Check logs: `docker-compose logs -f`

### "Metadata not preserved"

**Check**:
1. Source JPEG has EXIF: `exiftool photo.jpg`
2. `PRESERVE_METADATA=true` in environment
3. View task details in web interface for metadata_summary

### "Conversion fails"

**Common causes**:
- Corrupted JPEG file
- Insufficient disk space
- Insufficient memory (try reducing MAX_WORKERS)
- Check error message in task details

---

## 📚 Next Steps

- Read the full [README.md](README.md) for detailed documentation
- Check [CHANGELOG.md](CHANGELOG.md) for version history
- Review the [API documentation](#api-endpoints) in README
- Customize path logic in `app/converter.py`
- Add webhooks or notifications in `app/api.py`

---

## 💡 Pro Tips

1. **Batch Processing**: Use `once` mode for existing files, then switch to `watch` mode
2. **Resource Limits**: Set Docker memory/CPU limits in docker-compose.yml
3. **Backup**: Keep original JPEGs! The converter doesn't delete sources
4. **Performance**: SSD storage significantly improves conversion speed
5. **Quality**: Quality 90-95 gives best balance of size and quality
6. **Monitoring**: Use the API for external monitoring/alerting

---

## 🆘 Getting Help

1. Check the [README.md](README.md) troubleshooting section
2. Review logs: `docker-compose logs` or check console output
3. Run verification: `python verify_install.py`
4. Open an issue on GitHub with:
   - Your environment (Docker/local, OS)
   - Error messages from logs
   - Sample JPEG (if possible)

---

## ✅ Acceptance Checklist

Verify your setup works:

- [ ] Docker container starts successfully
- [ ] Web interface accessible at http://localhost:8000
- [ ] Dashboard shows statistics
- [ ] JPEG files in watch directory are detected
- [ ] Conversion creates HEIC files in heic/ subdirectory
- [ ] Tasks appear in web interface with status
- [ ] Metadata (DateTimeOriginal) preserved (check in task details)
- [ ] Database persists across restarts
- [ ] Manual scan button works

If all checkboxes pass, you're ready to go! 🎉

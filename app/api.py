"""
FastAPI web application for JPEG to HEIC converter
Provides REST API and web interface
"""
import os
import logging
import asyncio
from typing import Optional
from datetime import datetime
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, Query
from fastapi.staticfiles import StaticFiles
from fastapi.responses import HTMLResponse, FileResponse
from pydantic import BaseModel

from app.database import Database, Task
from app.converter import ImageConverter, get_heic_target_path
from app.watcher import FileWatcher, FileScanner, ConversionQueue
from app.config import config

logger = logging.getLogger(__name__)

# Global state
db: Database = None
converter: ImageConverter = None
watcher: FileWatcher = None
scanner: FileScanner = None
queue: ConversionQueue = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan management"""
    global db, converter, watcher, scanner, queue
    
    logger.info("Starting application")
    
    # Initialize database
    db = Database(config.DB_PATH)
    
    # Initialize converter
    converter = ImageConverter(
        quality=config.CONVERT_QUALITY,
        preserve_metadata=config.PRESERVE_METADATA
    )
    
    # Initialize conversion queue
    queue = ConversionQueue(
        max_workers=config.MAX_WORKERS,
        converter_func=process_conversion
    )
    
    # Initialize scanner
    scanner = FileScanner(config.WATCH_DIRS)
    
    # Start based on mode
    if config.MODE == 'watch':
        # Initialize and start watcher
        watcher = FileWatcher(
            directories=config.WATCH_DIRS,
            callback=lambda path: handle_new_file(path),
            stability_delay=config.METADATA_STABILITY_DELAY,
            poll_interval=config.POLL_INTERVAL
        )
        watcher.start()
        logger.info("Started in WATCH mode")
    else:
        # Run one-time scan
        logger.info("Starting one-time scan (ONCE mode)")
        perform_scan()
    
    yield
    
    # Cleanup
    logger.info("Shutting down application")
    if watcher:
        watcher.stop()
    if queue:
        queue.shutdown(wait=True)


app = FastAPI(title="JPEG to HEIC Converter", lifespan=lifespan)


# API Models
class TaskResponse(BaseModel):
    id: int
    task_type: str
    source_path: str
    target_path: Optional[str]
    status: str
    error_message: Optional[str]
    start_time: Optional[str]
    end_time: Optional[str]
    duration: Optional[float]
    metadata_preserved: bool
    metadata_summary: Optional[str]
    source_datetime: Optional[str]
    target_datetime: Optional[str]
    datetime_consistent: Optional[bool]
    created_at: Optional[str]
    updated_at: Optional[str]


class StatsResponse(BaseModel):
    total: int
    success: int
    failed: int
    running: int
    pending: int
    metadata_preserved: int
    metadata_preservation_rate: float
    queue_size: int


class ScanRequest(BaseModel):
    pass


# Helper functions
def handle_new_file(file_path: str):
    """Handle new JPEG file detected by watcher"""
    logger.info(f"New file detected: {file_path}")
    
    # Create task in database
    task = db.create_task(task_type='watch', source_path=file_path)
    
    # Submit to queue
    queue.submit(file_path, task.id)


def process_conversion(source_path: str, task_id: int):
    """Process a single conversion task"""
    logger.info(f"Processing conversion: {source_path} (task {task_id})")
    
    start_time = datetime.utcnow()
    
    # Update task to running
    db.update_task(task_id, status='running', start_time=start_time)
    
    try:
        # Calculate target path
        target_path = get_heic_target_path(source_path)
        
        # Perform conversion
        result = converter.convert(source_path, target_path)
        
        end_time = datetime.utcnow()
        duration = (end_time - start_time).total_seconds()
        
        # Update task based on result
        if result['success']:
            db.update_task(
                task_id,
                status='success',
                target_path=target_path,
                end_time=end_time,
                duration=duration,
                metadata_preserved=result.get('metadata_preserved', False),
                metadata_summary=result.get('metadata_summary', ''),
                source_datetime=result.get('source_datetime'),
                target_datetime=result.get('target_datetime')
            )
            logger.info(f"Conversion succeeded: {source_path} -> {target_path}")
        else:
            db.update_task(
                task_id,
                status='failed',
                error_message=result.get('error', 'Unknown error'),
                end_time=end_time,
                duration=duration
            )
            logger.error(f"Conversion failed: {source_path} - {result.get('error')}")
    
    except Exception as e:
        end_time = datetime.utcnow()
        duration = (end_time - start_time).total_seconds()
        
        db.update_task(
            task_id,
            status='failed',
            error_message=str(e),
            end_time=end_time,
            duration=duration
        )
        logger.error(f"Conversion error: {source_path} - {e}")


def perform_scan():
    """Perform one-time scan of directories"""
    logger.info("Performing directory scan")
    
    jpeg_files = scanner.scan()
    
    logger.info(f"Submitting {len(jpeg_files)} files for conversion")
    
    for file_path in jpeg_files:
        # Create task
        task = db.create_task(task_type='once', source_path=file_path)
        
        # Submit to queue
        queue.submit(file_path, task.id)
    
    logger.info(f"Scan complete, {len(jpeg_files)} tasks queued")


# API Endpoints
@app.get("/api/tasks", response_model=list[TaskResponse])
async def get_tasks(
    task_type: Optional[str] = Query(None, description="Filter by task type (once/watch)"),
    status: Optional[str] = Query(None, description="Filter by status"),
    limit: int = Query(100, ge=1, le=1000),
    offset: int = Query(0, ge=0)
):
    """Get list of tasks"""
    tasks = db.get_tasks(task_type=task_type, status=status, limit=limit, offset=offset)
    return [task.to_dict() for task in tasks]


@app.get("/api/tasks/{task_id}", response_model=TaskResponse)
async def get_task(task_id: int):
    """Get single task by ID"""
    task = db.get_task(task_id)
    if not task:
        raise HTTPException(status_code=404, detail="Task not found")
    return task.to_dict()


@app.get("/api/stats", response_model=StatsResponse)
async def get_stats():
    """Get conversion statistics"""
    stats = db.get_stats()
    stats['queue_size'] = queue.get_queue_size() if queue else 0
    return stats


@app.post("/api/scan-now")
async def scan_now():
    """Trigger manual scan"""
    logger.info("Manual scan triggered via API")
    perform_scan()
    return {"status": "scan started", "message": "Scan has been triggered"}


@app.get("/", response_class=HTMLResponse)
async def root():
    """Serve main HTML page"""
    static_dir = os.path.join(os.path.dirname(__file__), '..', 'static')
    index_path = os.path.join(static_dir, 'index.html')
    
    if os.path.exists(index_path):
        with open(index_path, 'r', encoding='utf-8') as f:
            return HTMLResponse(content=f.read())
    else:
        return HTMLResponse(content="<h1>JPEG to HEIC Converter</h1><p>Frontend not found</p>")


@app.get("/health")
async def health():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "mode": config.MODE,
        "watch_dirs": config.WATCH_DIRS,
        "watcher_running": watcher.is_running() if watcher else False
    }


# Mount static files
static_dir = os.path.join(os.path.dirname(__file__), '..', 'static')
if os.path.exists(static_dir):
    app.mount("/static", StaticFiles(directory=static_dir), name="static")

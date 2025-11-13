"""
File watcher and scanner for JPEG files
Supports both one-time scanning and continuous monitoring
"""
import os
import time
import logging
import asyncio
from typing import List, Set, Callable
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler, FileCreatedEvent

logger = logging.getLogger(__name__)


class JPEGFileHandler(FileSystemEventHandler):
    """Handle file system events for JPEG files"""
    
    def __init__(self, callback: Callable, stability_delay: float = 1.0):
        self.callback = callback
        self.stability_delay = stability_delay
        self.processing_files: Set[str] = set()
        super().__init__()
    
    def on_created(self, event):
        """Handle file creation events"""
        if event.is_directory:
            return
        
        file_path = event.src_path
        
        # Check if it's a JPEG file
        if not self._is_jpeg(file_path):
            return
        
        # Avoid duplicate processing
        if file_path in self.processing_files:
            return
        
        self.processing_files.add(file_path)
        
        # Wait for file to stabilize
        if self.stability_delay > 0:
            time.sleep(self.stability_delay)
            if not self._is_file_stable(file_path):
                logger.warning(f"File {file_path} is not stable, skipping")
                self.processing_files.discard(file_path)
                return
        
        # Trigger callback
        try:
            self.callback(file_path)
        except Exception as e:
            logger.error(f"Error processing {file_path}: {e}")
        finally:
            self.processing_files.discard(file_path)
    
    def _is_jpeg(self, file_path: str) -> bool:
        """Check if file is a JPEG"""
        ext = os.path.splitext(file_path)[1].lower()
        return ext in ['.jpg', '.jpeg']
    
    def _is_file_stable(self, file_path: str, checks: int = 3, interval: float = 0.3) -> bool:
        """Check if file size is stable (not being written)"""
        try:
            if not os.path.exists(file_path):
                return False
            
            prev_size = os.path.getsize(file_path)
            
            for _ in range(checks):
                time.sleep(interval)
                if not os.path.exists(file_path):
                    return False
                current_size = os.path.getsize(file_path)
                if current_size != prev_size:
                    return False
                prev_size = current_size
            
            return True
        except Exception as e:
            logger.warning(f"Error checking file stability for {file_path}: {e}")
            return False


class FileWatcher:
    """Watch directories for new JPEG files"""
    
    def __init__(self, directories: List[str], callback: Callable, 
                 stability_delay: float = 1.0, poll_interval: float = 1.0):
        self.directories = directories
        self.callback = callback
        self.stability_delay = stability_delay
        self.poll_interval = poll_interval
        self.observer = Observer()
        self.running = False
    
    def start(self):
        """Start watching directories"""
        logger.info(f"Starting file watcher for directories: {self.directories}")
        
        handler = JPEGFileHandler(self.callback, self.stability_delay)
        
        for directory in self.directories:
            if not os.path.exists(directory):
                logger.warning(f"Watch directory does not exist: {directory}")
                continue
            
            self.observer.schedule(handler, directory, recursive=True)
            logger.info(f"Watching directory: {directory}")
        
        self.observer.start()
        self.running = True
    
    def stop(self):
        """Stop watching"""
        if self.running:
            logger.info("Stopping file watcher")
            self.observer.stop()
            self.observer.join()
            self.running = False
    
    def is_running(self) -> bool:
        """Check if watcher is running"""
        return self.running


class FileScanner:
    """Scan directories for existing JPEG files"""
    
    def __init__(self, directories: List[str]):
        self.directories = directories
    
    def scan(self) -> List[str]:
        """
        Scan all directories recursively for JPEG files
        Returns list of file paths
        """
        jpeg_files = []
        
        logger.info(f"Scanning directories: {self.directories}")
        
        for directory in self.directories:
            if not os.path.exists(directory):
                logger.warning(f"Scan directory does not exist: {directory}")
                continue
            
            logger.info(f"Scanning directory: {directory}")
            count = 0
            
            for root, _, files in os.walk(directory):
                for filename in files:
                    if self._is_jpeg(filename):
                        file_path = os.path.join(root, filename)
                        jpeg_files.append(file_path)
                        count += 1
            
            logger.info(f"Found {count} JPEG files in {directory}")
        
        logger.info(f"Total JPEG files found: {len(jpeg_files)}")
        return jpeg_files
    
    def _is_jpeg(self, filename: str) -> bool:
        """Check if filename is a JPEG"""
        ext = os.path.splitext(filename)[1].lower()
        return ext in ['.jpg', '.jpeg']


class ConversionQueue:
    """Manage conversion task queue with thread pool"""
    
    def __init__(self, max_workers: int, converter_func: Callable):
        self.max_workers = max_workers
        self.converter_func = converter_func
        self.executor = ThreadPoolExecutor(max_workers=max_workers)
        self.active_tasks: Set[str] = set()
        self.pending_count = 0
    
    def submit(self, source_path: str, task_id: int):
        """Submit a conversion task"""
        if source_path in self.active_tasks:
            logger.warning(f"Task already active for {source_path}")
            return
        
        self.active_tasks.add(source_path)
        self.pending_count += 1
        
        future = self.executor.submit(self._run_conversion, source_path, task_id)
        future.add_done_callback(lambda f: self._task_done(source_path))
    
    def _run_conversion(self, source_path: str, task_id: int):
        """Run the conversion (executed in thread pool)"""
        try:
            self.converter_func(source_path, task_id)
        except Exception as e:
            logger.error(f"Conversion failed for {source_path}: {e}")
    
    def _task_done(self, source_path: str):
        """Cleanup after task completion"""
        self.active_tasks.discard(source_path)
        self.pending_count = max(0, self.pending_count - 1)
    
    def get_queue_size(self) -> int:
        """Get number of active tasks"""
        return len(self.active_tasks)
    
    def shutdown(self, wait: bool = True):
        """Shutdown the executor"""
        logger.info("Shutting down conversion queue")
        self.executor.shutdown(wait=wait)

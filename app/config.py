"""
Configuration management
Load settings from environment variables
"""
import os
import logging
from typing import List


class Config:
    """Application configuration"""
    
    def __init__(self):
        # Operating mode
        self.MODE = os.getenv('MODE', 'once').lower()
        if self.MODE not in ['once', 'watch']:
            raise ValueError(f"Invalid MODE: {self.MODE}. Must be 'once' or 'watch'")
        
        # Watch directories
        watch_dirs_str = os.getenv('WATCH_DIRS', '')
        if not watch_dirs_str:
            raise ValueError("WATCH_DIRS environment variable is required")
        self.WATCH_DIRS = [d.strip() for d in watch_dirs_str.split(',') if d.strip()]
        
        # Database path
        self.DB_PATH = os.getenv('DB_PATH', '/data/tasks.db')
        
        # Logging level
        log_level_str = os.getenv('LOG_LEVEL', 'INFO').upper()
        self.LOG_LEVEL = getattr(logging, log_level_str, logging.INFO)
        
        # Poll interval for watch mode
        self.POLL_INTERVAL = float(os.getenv('POLL_INTERVAL', '1'))
        
        # Max workers for conversion
        self.MAX_WORKERS = int(os.getenv('MAX_WORKERS', '4'))
        
        # HEIC quality
        self.CONVERT_QUALITY = int(os.getenv('CONVERT_QUALITY', '90'))
        if not 0 <= self.CONVERT_QUALITY <= 100:
            raise ValueError(f"CONVERT_QUALITY must be between 0 and 100")
        
        # HTTP port
        self.HTTP_PORT = int(os.getenv('HTTP_PORT', '8000'))
        
        # Metadata preservation
        preserve_str = os.getenv('PRESERVE_METADATA', 'true').lower()
        self.PRESERVE_METADATA = preserve_str in ['true', '1', 'yes', 'on']
        
        # Metadata stability delay
        self.METADATA_STABILITY_DELAY = float(os.getenv('METADATA_STABILITY_DELAY', '1'))
    
    def __repr__(self):
        return (
            f"Config(MODE={self.MODE}, WATCH_DIRS={self.WATCH_DIRS}, "
            f"DB_PATH={self.DB_PATH}, MAX_WORKERS={self.MAX_WORKERS}, "
            f"QUALITY={self.CONVERT_QUALITY})"
        )


# Global config instance
config = Config()


def setup_logging():
    """Setup logging configuration"""
    logging.basicConfig(
        level=config.LOG_LEVEL,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        datefmt='%Y-%m-%d %H:%M:%S'
    )

"""
Main entry point for JPEG to HEIC converter
"""
import os
import sys
import signal
import logging
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

from app.config import config, setup_logging
from app.api import app

# Setup logging
setup_logging()
logger = logging.getLogger(__name__)


def signal_handler(signum, frame):
    """Handle shutdown signals"""
    logger.info(f"Received signal {signum}, shutting down gracefully")
    sys.exit(0)


def main():
    """Main application entry point"""
    # Register signal handlers
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    logger.info("="*60)
    logger.info("JPEG to HEIC Converter")
    logger.info("="*60)
    logger.info(f"Configuration: {config}")
    logger.info("="*60)
    
    # Run FastAPI with Uvicorn
    import uvicorn
    
    uvicorn.run(
        app,
        host="0.0.0.0",
        port=config.HTTP_PORT,
        log_level=logging.getLevelName(config.LOG_LEVEL).lower()
    )


if __name__ == "__main__":
    main()

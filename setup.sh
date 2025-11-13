#!/bin/bash
# Quick start script for local development

set -e

echo "==================================="
echo "JPEG to HEIC Converter - Setup"
echo "==================================="

# Check Python version
echo "Checking Python version..."
python3 --version

# Create virtual environment
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
echo "Activating virtual environment..."
source venv/bin/activate

# Install dependencies
echo "Installing dependencies..."
pip install --upgrade pip
pip install -r requirements.txt

# Create data directories
echo "Creating data directories..."
mkdir -p data/images data/output data/db

# Copy example env if .env doesn't exist
if [ ! -f ".env" ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
    echo "⚠️  Please edit .env with your settings!"
fi

echo ""
echo "==================================="
echo "Setup complete!"
echo "==================================="
echo ""
echo "Next steps:"
echo "1. Edit .env with your configuration"
echo "2. Place JPEG files in data/images/"
echo "3. Run: python -m app.main"
echo "4. Open: http://localhost:8000"
echo ""
echo "For Docker:"
echo "  docker-compose up -d"
echo ""

#!/bin/bash
set -e

echo "🚀 JPEG2HEIF Quick Start"
echo "========================"
echo ""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

echo "✅ Docker is installed"
echo ""

# Create necessary directories
echo "📁 Creating directories..."
mkdir -p watch db

echo "✅ Directories created"
echo ""

# Copy .env.example to .env if not exists
if [ ! -f .env ]; then
    echo "📝 Creating .env file from .env.example..."
    cp .env.example .env
    echo "✅ .env file created"
else
    echo "ℹ️  .env file already exists"
fi
echo ""

# Build and start
echo "🔨 Building Docker image..."
docker-compose build

echo ""
echo "🚀 Starting services..."
docker-compose up -d

echo ""
echo "✅ JPEG2HEIF is now running!"
echo ""
echo "📊 Web UI: http://localhost:8080"
echo "📂 Watch directory: ./watch"
echo "💾 Database: ./db/jpeg2heif.db"
echo ""
echo "📝 View logs: docker-compose logs -f"
echo "🛑 Stop: docker-compose down"
echo ""
echo "💡 Add JPEG files to the ./watch directory to start conversion"

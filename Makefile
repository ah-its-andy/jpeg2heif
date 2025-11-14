.PHONY: build test clean run docker-build docker-up docker-down

# Build the binary
build:
	CGO_ENABLED=0 go build -o jpeg2heif ./cmd/jpeg2heif

# Run tests
test:
	go test ./... -v

# Run tests in short mode (skip integration tests)
test-short:
	go test ./... -v -short

# Run tests with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f jpeg2heif coverage.out coverage.html
	rm -rf dist/

# Run the application locally
run: build
	./jpeg2heif

# Build Docker image
docker-build:
	docker build -t jpeg2heif:latest .

# Start with docker-compose
docker-up:
	docker-compose up -d

# Stop docker-compose
docker-down:
	docker-compose down

# View docker logs
docker-logs:
	docker-compose logs -f

# Tidy up dependencies
tidy:
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Install dependencies
deps:
	go mod download

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  test          - Run all tests"
	@echo "  test-short    - Run tests without integration tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  run           - Build and run locally"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-up     - Start with docker-compose"
	@echo "  docker-down   - Stop docker-compose"
	@echo "  docker-logs   - View docker logs"
	@echo "  tidy          - Tidy up dependencies"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code"
	@echo "  deps          - Download dependencies"
	@echo "  help          - Show this help message"

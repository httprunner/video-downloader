.PHONY: build build-cli build-server clean test test-coverage deps run run-server run-cli install install-cli install-server docker-build docker-run

# Build targets
build: build-cli build-server

build-cli:
	go build -o bin/video-downloader ./cmd/cli

build-server:
	go build -o bin/video-downloader-server ./cmd/server

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Run tests
test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Dependency management
deps:
	go mod download
	go mod tidy

# Run applications
run: run-cli

run-cli:
	go run ./cmd/cli

run-server:
	go run ./cmd/server

# Install applications
install: install-cli install-server

install-cli:
	go install ./cmd/cli

install-server:
	go install ./cmd/server

# Docker targets
docker-build:
	docker build -t video-downloader .

docker-run:
	docker run -p 8080:8080 -v $(PWD)/downloads:/app/downloads video-downloader

# Development helpers
dev-setup:
	mkdir -p config data downloads logs temp
	go mod download
	go generate ./...

# Build for multiple platforms
build-all: build-cli-linux build-cli-windows build-cli-darwin build-server-linux build-server-windows build-server-darwin

build-cli-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/video-downloader-linux-amd64 ./cmd/cli

build-cli-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/video-downloader-windows-amd64.exe ./cmd/cli

build-cli-darwin:
	GOOS=darwin GOARCH=amd64 go build -o bin/video-downloader-darwin-amd64 ./cmd/cli

build-server-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/video-downloader-server-linux-amd64 ./cmd/server

build-server-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/video-downloader-server-windows-amd64.exe ./cmd/server

build-server-darwin:
	GOOS=darwin GOARCH=amd64 go build -o bin/video-downloader-server-darwin-amd64 ./cmd/server

# Linting and formatting
lint:
	golangci-lint run

fmt:
	go fmt ./...

# Generate documentation
docs:
	godoc -http=:6060

# Example usage
example-cli:
	./bin/video-downloader download "https://www.tiktok.com/@tiktok/video/7000000000000000000"

example-server:
	./bin/video-downloader-server &

example-api:
	curl -X POST http://localhost:8080/api/v1/videos/info \
		-H "Content-Type: application/json" \
		-d '{"url": "https://www.tiktok.com/@tiktok/video/7000000000000000000"}'

# Database operations
db-backup:
	cp data/video-downloader.db data/video-downloader.db.backup

db-restore:
	cp data/video-downloader.db.backup data/video-downloader.db

db-clean:
	rm -f data/video-downloader.db
	@echo "Database removed. A new one will be created on next run."

# Help
help:
	@echo "Available targets:"
	@echo "  build              - Build CLI and server"
	@echo "  build-cli          - Build CLI only"
	@echo "  build-server       - Build server only"
	@echo "  clean              - Clean build artifacts"
	@echo "  test               - Run tests"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  deps               - Download dependencies"
	@echo "  run-cli            - Run CLI application"
	@echo "  run-server         - Run server application"
	@echo "  install-cli        - Install CLI application"
	@echo "  install-server     - Install server application"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run Docker container"
	@echo "  dev-setup          - Setup development environment"
	@echo "  build-all          - Build for all platforms"
	@echo "  lint               - Run linter"
	@echo "  fmt                - Format code"
	@echo "  docs               - Generate documentation"
	@echo "  example-cli        - Example CLI usage"
	@echo "  example-server     - Example server usage"
	@echo "  example-api        - Example API usage"
	@echo "  db-backup          - Backup database"
	@echo "  db-restore         - Restore database from backup"
	@echo "  db-clean           - Clean database"
	@echo "  help               - Show this help message"
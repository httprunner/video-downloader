# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

### Development
```bash
# Setup development environment
make dev-setup

# Build applications
make build              # Build both CLI and server
make build-cli          # Build CLI only
make build-server       # Build server only

# Run applications
make run-cli            # Run CLI application
make run-server         # Run server application

# Testing
make test               # Run all tests
make test-coverage      # Run tests with coverage report

# Code quality
make lint               # Run linter (golangci-lint)
make fmt                # Format code

# Dependency management
make deps               # Download and tidy dependencies

# Clean build artifacts
make clean
```

### Docker
```bash
# Build and run with Docker
make docker-build
make docker-run

# Or use docker-compose directly
docker-compose up -d
docker-compose -f docker-compose.yml --profile proxy up -d  # With nginx
```

### Cross-compilation
```bash
make build-all          # Build for all platforms
```

## Architecture Overview

This is a Go-based video downloader application that supports multiple platforms (TikTok, XHS, Kuaishou) with both CLI and REST API interfaces.

### Core Components

1. **Platform Abstraction** (`internal/platform/`)
   - Each platform (TikTok, XHS, Kuaishou) implements the `PlatformExtractor` interface
   - Platform-specific logic is isolated in separate packages
   - New platforms can be added by implementing the interface

2. **Download Manager** (`internal/downloader/manager.go`)
   - Manages download queue with configurable workers
   - Handles progress tracking and retry logic
   - Supports batch downloads and resume functionality

3. **Storage Layer** (`internal/storage/`)
   - SQLite-based storage for video metadata and download tracking
   - Implements the `Storage` interface for database operations
   - Tracks video info, download tasks, and user sessions

4. **Server** (`internal/server/`)
   - REST API built with Gin framework
   - JWT-based authentication
   - Rate limiting and request monitoring
   - Prometheus metrics integration

5. **Configuration** (`internal/config/`)
   - YAML-based configuration with environment variable overrides
   - Platform-specific settings for cookies and user agents
   - Proxy configuration support

### Key Interfaces

- `PlatformExtractor`: Defines methods for extracting video/author info from platforms
- `Downloader`: Handles file downloads with progress tracking
- `Storage`: Provides database operations for videos, users, and sessions

### Project Structure
```
cmd/                    # Application entry points (CLI, server, TUI)
internal/
  auth/                 # JWT authentication middleware
  config/               # Configuration management
  downloader/           # Download queue and worker management
  extractor/            # Generic extraction logic
  monitor/              # Metrics and monitoring
  platform/             # Platform-specific extractors
  ratelimit/            # Rate limiting implementation
  registry/             # Service registry
  server/               # HTTP server implementation
  storage/              # Database operations
  utils/                # Utility functions (HTTP client, downloader)
pkg/
  api/                  # API models and handlers
  models/               # Data models and interfaces
```

### Database Schema
- `videos`: Stores video metadata and download status
- `authors`: Stores author information
- `download_tasks`: Tracks active downloads
- `users`: User accounts for web interface
- `sessions`: JWT session management

### Configuration Notes
- Default config location: `./config/config.yaml`
- Environment variables override with `VD_` prefix (e.g., `VD_SERVER_PORT`)
- Platform cookies and user agents are configurable per platform
- File naming uses templates with placeholders: `{platform}`, `{author}`, `{title}`, `{id}`, `{date}`

### Development Tips
- Use `make dev-setup` to create required directories and download dependencies
- The application creates necessary directories (config, data, downloads, logs, temp) on first run
- Test coverage reports are generated as HTML with `make test-coverage`
- Cross-platform builds are available via Makefile targets
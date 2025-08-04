# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

### Development
```bash
# Setup development environment
make dev-setup

# Build applications
make build              # Build CLI, server, and TUI
make build-cli          # Build CLI only
make build-server       # Build server only
make build-tui          # Build TUI only

# Run applications
make run-cli            # Run CLI application
make run-server         # Run server application
make run-tui            # Run TUI application

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

This is a Go-based video downloader application that supports multiple platforms (TikTok, XHS, Kuaishou) with CLI, TUI, and REST API interfaces.

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

6. **TUI Interface** (`internal/tui/`)
   - Terminal User Interface built with Bubble Tea framework
   - Interactive menus for download management
   - Real-time progress tracking
   - Multi-screen navigation

7. **Batch Download** (`internal/batch/`)
   - Concurrent batch download management
   - User profile and playlist processing
   - Progress tracking and error handling
   - Configurable retry and skip logic

8. **Cookie Management** (`internal/cookie/`)
   - Browser cookie extraction (Chrome, Firefox, Safari, Edge, etc.)
   - Cross-platform cookie reading
   - Cookie validation and persistence
   - Platform-specific cookie handling

9. **Resume Download** (`internal/resume/`)
   - HTTP Range request support for resumable downloads
   - Metadata persistence for interrupted downloads
   - File integrity verification with checksums
   - Progress state management

10. **Data Export** (`internal/export/`)
    - Multi-format export (CSV, XLSX, JSON, TXT)
    - Configurable column selection
    - Template generation for bulk imports
    - Data validation and formatting

11. **Comment Extraction** (`internal/comment/`)
    - Platform-specific comment extraction
    - Reply thread processing
    - Mention detection and parsing
    - Multi-format comment export

### Key Interfaces

- `PlatformExtractor`: Defines methods for extracting video/author info from platforms
- `Downloader`: Handles file downloads with progress tracking
- `Storage`: Provides database operations for videos, users, and sessions

### Project Structure
```
cmd/                    # Application entry points (CLI, server, TUI)
internal/
  auth/                 # JWT authentication middleware
  batch/                # Batch download management
  comment/              # Comment extraction system
  config/               # Configuration management
  cookie/               # Browser cookie management
  downloader/           # Download queue and worker management
  export/               # Multi-format data export
  extractor/            # Generic extraction logic
  monitor/              # Metrics and monitoring
  platform/             # Platform-specific extractors
  ratelimit/            # Rate limiting implementation
  registry/             # Service registry
  resume/               # Resumable download management
  server/               # HTTP server implementation
  storage/              # Database operations
  tui/                  # Terminal User Interface
  utils/                # Utility functions (HTTP client, downloader)
pkg/
  api/                  # API models and handlers
  models/               # Data models and interfaces
```

### New Features Implemented

#### TUI (Terminal User Interface)
- Interactive terminal interface with menu navigation
- Real-time download progress display
- Multi-screen application (Main Menu, Downloads, Settings, Help)
- Built with Charm's Bubble Tea framework

#### Advanced Download Features
- **Batch Downloads**: Process multiple URLs simultaneously
  - User profile scraping
  - Playlist/collection processing
  - Configurable concurrency limits

- **Resumable Downloads**: HTTP Range request support
  - Metadata persistence for interrupted downloads
  - File integrity verification
  - Progress state management

- **Cookie Management**: Browser cookie extraction
  - Support for Chrome, Firefox, Safari, Edge, Opera, Brave
  - Cross-platform compatibility (Windows, macOS, Linux)
  - Cookie validation and updating

#### Data Management
- **Multi-format Export**: Export video metadata to various formats
  - CSV with configurable delimiters
  - Excel (XLSX) with formatting and filters
  - JSON with complete metadata
  - Plain text reports

- **Comment Extraction**: Extract and analyze comments
  - Platform-specific comment APIs
  - Reply thread processing
  - Mention detection (@username)
  - Export to multiple formats

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
- TUI can be tested with `make run-tui` for interactive development

### Feature Comparison with Original Python Implementations

The Go implementation now includes most features from the original Python repositories:

**From KS-Downloader:**
✅ Multi-format downloads (video/image)
✅ Batch processing
✅ SQLite database storage
✅ Cookie management
✅ File naming templates
✅ Progress tracking
✅ Error handling and retries
✅ GraphQL API integration for real video URLs

**From TikTokDownloader:**
✅ Multiple interface support (CLI/TUI/API)
✅ Comment extraction
✅ Multi-platform support
✅ Data export (CSV/XLSX/JSON)
✅ Resume capability
✅ Authentication system
✅ Rate limiting

**From XHS-Downloader:**
✅ Multi-language support architecture
✅ Browser cookie extraction
✅ Image format conversion support
✅ Metadata persistence
✅ Progress monitoring
✅ File integrity verification

### Key Implementation Details

#### Kuaishou GraphQL API Integration
The Go implementation uses Kuaishou's internal GraphQL API to extract real video URLs, but faces modern anti-scraping challenges:

- **Primary Method**: GraphQL query to `https://www.kuaishou.com/graphql` using the `visionVideoDetail` query
- **Anti-Scraping Protection**: Kuaishou implements risk control that blocks unauthenticated requests
- **Authentication Required**: Requires valid browser cookies from an authenticated session
- **Fallback Method**: HTML parsing for cases where the API is unavailable
- **Video URL Extraction**: Successfully extracts `mainMvUrls` when properly authenticated
- **Multi-quality Support**: Handles different quality types returned by the API
- **Image Support**: Falls back to `mainImageUrls` for image content

**Current Status**: ✅ API framework implemented, ⚠️ requires browser cookies for success

**Setup Required**: Users must configure browser cookies in `config/config.yaml` - see `docs/KUAISHOU_SETUP.md` for detailed instructions.

#### Platform-Specific Extractors
- **TikTok**: Uses web scraping with fallback to mobile API endpoints
- **XHS**: Leverages XHS internal APIs with proper headers and authentication  
- **Kuaishou**: GraphQL API first, HTML parsing second, with proper cookie management

#### Enhanced Error Handling
- **Graceful Degradation**: If GraphQL API fails, falls back to HTML parsing
- **Detailed Logging**: Comprehensive logging at debug and info levels for troubleshooting
- **User-Friendly Messages**: Clear error messages explaining extraction failures
- **Retry Logic**: Configurable retry attempts with exponential backoff

### Missing Features (Future Implementation)
- Live streaming download (FFmpeg integration)
- Web interface for management
- Clipboard monitoring
- Advanced analytics and visualization
- Plugin/extension system
- Image format conversion
- Video quality selection algorithms
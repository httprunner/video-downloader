# Video Downloader

A unified video downloader for TikTok, Xiaohongshu (XHS), and Kuaishou platforms built with Go.

## Features

- **Multi-platform Support**: Download videos from TikTok, XHS, and Kuaishou
- **High Quality Downloads**: Download videos in original quality
- **Batch Processing**: Download multiple videos simultaneously
- **Progress Tracking**: Real-time download progress with speed and ETA
- **Resume Support**: Resume interrupted downloads
- **Proxy Support**: HTTP/HTTPS and SOCKS5 proxy support
- **Customizable Output**: Flexible file naming and organization
- **REST API**: HTTP API for integration with other applications
- **Command Line Interface**: Easy-to-use CLI for manual downloads
- **Database Storage**: SQLite database for tracking downloads

## Installation

### Prerequisites

- Go 1.21 or higher
- Git

### Build from Source

```bash
git clone https://github.com/httprunner/video-downloader.git
cd video-downloader
go mod download
go build -o video-downloader ./cmd/cli
```

### Install via Go

```bash
go install github.com/httprunner/video-downloader@latest
```

## Configuration

The application uses a YAML configuration file. Run the following command to create a default configuration:

```bash
video-downloader config init
```

This will create a `config.yaml` file in the `./config` directory with default settings.

### Configuration Options

```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30
  write_timeout: 30

download:
  max_workers: 5
  chunk_size: 1048576  # 1MB
  timeout: 300
  retry_count: 3
  save_path: ./downloads
  create_folder: true
  file_naming: "{platform}_{author}_{title}_{id}"

database:
  type: sqlite
  path: ./data/video-downloader.db
  max_conns: 10

log:
  level: info
  format: text
  output: stdout

proxy:
  enabled: false
  type: http
  host: ""
  port: 0
  username: ""
  password: ""

platforms:
  tiktok:
    enabled: true
    api_key: ""
    cookie: ""
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

  xhs:
    enabled: true
    cookie: ""
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

  kuaishou:
    enabled: true
    cookie: ""
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
```

## Usage

### Command Line Interface

#### Download a Single Video

```bash
video-downloader download "https://www.tiktok.com/@username/video/1234567890"
```

#### Download with Custom Options

```bash
video-downloader download \
  --output ./my-videos \
  --format mp4 \
  --quality hd \
  "https://www.tiktok.com/@username/video/1234567890"
```

#### Batch Download

Create a file `urls.txt` with one URL per line:

```
https://www.tiktok.com/@username/video/1234567890
https://www.xiaohongshu.com/explore/abcdef
https://www.kuaishou.com/short-video/ghijkl
```

Then run:

```bash
video-downloader batch urls.txt
```

#### Get Video Information

```bash
video-downloader info "https://www.tiktok.com/@username/video/1234567890"
```

#### List Downloaded Videos

```bash
video-downloader list
```

#### Start API Server

```bash
video-downloader server
```

### API Usage

Start the server:

```bash
video-downloader server
```

The server will be available at `http://localhost:8080`.

#### Endpoints

##### Health Check
```http
GET /health
```

##### Download Video
```http
POST /api/v1/videos/download
Content-Type: application/json

{
  "url": "https://www.tiktok.com/@username/video/1234567890",
  "output_path": "./downloads",
  "format": "mp4",
  "quality": "hd",
  "download": true
}
```

##### Batch Download
```http
POST /api/v1/videos/batch
Content-Type: application/json

{
  "urls": [
    "https://www.tiktok.com/@username/video/1234567890",
    "https://www.xiaohongshu.com/explore/abcdef"
  ],
  "output_path": "./downloads",
  "format": "mp4"
}
```

##### Get Video Information
```http
POST /api/v1/videos/info
Content-Type: application/json

{
  "url": "https://www.tiktok.com/@username/video/1234567890"
}
```

##### List Videos
```http
GET /api/v1/videos?platform=tiktok&limit=10&offset=0
```

##### Get Download Status
```http
GET /api/v1/downloads
```

##### Cancel Download
```http
DELETE /api/v1/downloads/{download_id}
```

##### Get Statistics
```http
GET /api/v1/stats
```

## Supported Platforms

### TikTok
- Regular video URLs: `https://www.tiktok.com/@username/video/1234567890`
- Short URLs: `https://vm.tiktok.com/XYZ123`
- User profile URLs: `https://www.tiktok.com/@username`

### Xiaohongshu (XHS)
- Explore URLs: `https://www.xiaohongshu.com/explore/abcdef`
- Discovery URLs: `https://www.xiaohongshu.com/discovery/item/abcdef`
- User profile URLs: `https://www.xiaohongshu.com/user/profile/abcdef`
- Short URLs: `https://xhslink.com/abcdef`

### Kuaishou
- Short video URLs: `https://www.kuaishou.com/short-video/abcdef`
- Profile URLs: `https://www.kuaishou.com/profile/abcdef`
- Short URLs: `https://v.kuaishou.com/abcdef`

## File Naming

The application supports flexible file naming using placeholders:

- `{platform}`: Platform name (tiktok, xhs, kuaishou)
- `{author}`: Author username
- `{title}`: Video title
- `{id}`: Video ID
- `{date}`: Publication date (YYYY-MM-DD)

Example: `{platform}_{author}_{title}_{id}` → `tiktok_johndoe_My_Video_1234567890.mp4`

## Proxy Configuration

To use a proxy, update the configuration:

```yaml
proxy:
  enabled: true
  type: http  # http, https, socks5
  host: proxy.example.com
  port: 8080
  username: your_username  # Optional
  password: your_password  # Optional
```

## Development

### Project Structure

```
video-downloader/
├── cmd/
│   ├── cli/          # CLI entry point
│   ├── server/       # API server entry point
│   └── tui/          # TUI interface (future)
├── internal/
│   ├── config/       # Configuration management
│   ├── downloader/   # Download management
│   ├── extractor/    # Data extraction
│   ├── platform/     # Platform adapters
│   │   ├── tiktok/
│   │   ├── xhs/
│   │   └── kuaishou/
│   ├── storage/      # Data storage
│   └── utils/        # Utility functions
├── pkg/
│   ├── models/       # Data models
│   └── api/          # API definitions
├── go.mod
├── go.sum
└── README.md
```

### Running Tests

```bash
go test ./...
```

### Building

```bash
# Build CLI
go build -o video-downloader ./cmd/cli

# Build server
go build -o video-downloader-server ./cmd/server

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o video-downloader-linux ./cmd/cli
GOOS=windows GOARCH=amd64 go build -o video-downloader.exe ./cmd/cli
GOOS=darwin GOARCH=amd64 go build -o video-downloader-macos ./cmd/cli
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/new-feature`
3. Commit your changes: `git commit -am 'Add new feature'`
4. Push to the branch: `git push origin feature/new-feature`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Disclaimer

This tool is for educational and personal use only. Please respect the terms of service of the respective platforms and ensure you have the right to download and use the content. The developers are not responsible for any misuse of this tool.

## Support

If you encounter any issues or have questions, please open an issue on GitHub.
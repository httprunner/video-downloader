#!/bin/sh
set -e

# Initialize directories and config if needed
if [ ! -f "/app/config/config.yaml" ]; then
    echo "Initializing default configuration..."
    mkdir -p /app/config
    cat > /app/config/config.yaml << EOF
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30
  write_timeout: 30

download:
  max_workers: 5
  chunk_size: 1048576
  timeout: 300
  retry_count: 3
  save_path: /app/downloads
  create_folder: true
  file_naming: "{platform}_{author}_{title}_{id}"

database:
  type: sqlite
  path: /app/data/video-downloader.db
  max_conns: 10

log:
  level: info
  format: text
  output: /app/logs/app.log

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
EOF
fi

# Run the command
case "$1" in
    "server")
        echo "Starting video-downloader server..."
        exec ./video-downloader-server --config /app/config/config.yaml
        ;;
    "cli")
        shift
        echo "Running video-downloader CLI..."
        exec ./video-downloader --config /app/config/config.yaml "$@"
        ;;
    *)
        exec "$@"
        ;;
esac
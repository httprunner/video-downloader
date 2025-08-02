# Multi-stage build for video-downloader
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build CLI and server
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o video-downloader ./cmd/cli
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o video-downloader-server ./cmd/server

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata && \
    rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 -S appuser && \
    adduser -u 1000 -S appuser -G appuser

# Create directories
RUN mkdir -p /app/config /app/data /app/downloads /app/logs /app/temp && \
    chown -R appuser:appuser /app

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/video-downloader .
COPY --from=builder /app/video-downloader-server .

# Copy default config
COPY --chown=appuser:appuser config/config.yaml ./config/

# Copy entrypoint script
COPY --chown=appuser:appuser docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Switch to non-root user
USER appuser

# Expose port for server
EXPOSE 8080

# Set environment variables
ENV VD_DOWNLOAD_SAVE_PATH=/app/downloads
ENV VD_DATABASE_PATH=/app/data/video-downloader.db
ENV VD_LOG_OUTPUT=/app/logs/app.log

# Volume mounts
VOLUME ["/app/downloads", "/app/data", "/app/logs"]

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Entrypoint
ENTRYPOINT ["docker-entrypoint.sh"]

# Default command
CMD ["server"]
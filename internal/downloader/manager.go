package downloader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"video-downloader/internal/platform"
	"video-downloader/internal/utils"
	"video-downloader/pkg/models"
)

// Manager manages the download process
type Manager struct {
	config     *models.Config
	logger     zerolog.Logger
	storage    models.Storage
	downloader *utils.DownloadManager
	extractors map[models.Platform]models.PlatformExtractor
	queue      chan *DownloadRequest
	workers    int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// DownloadRequest represents a download request
type DownloadRequest struct {
	URL      string
	Platform models.Platform
	Options  *DownloadOptions
}

// DownloadOptions represents download options
type DownloadOptions struct {
	OutputPath    string
	Format        string
	Quality       string
	DownloadAudio bool
	Metadata      bool
	Progress      bool
}

// DownloadResult represents download result
type DownloadResult struct {
	Success bool
	Message string
	Video   *models.VideoInfo
	Error   error
}

// NewManager creates a new download manager
func NewManager(cfg *models.Config, storage models.Storage) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// Create download manager
	dm := utils.NewDownloadManager(utils.DownloadConfig{
		MaxWorkers: cfg.Download.MaxWorkers,
		ChunkSize:  int64(cfg.Download.ChunkSize),
		RetryCount: cfg.Download.RetryCount,
		TempDir:    "./temp",
		Timeout:    time.Duration(cfg.Download.Timeout) * time.Second,
	})

	// Create extractors
	extractors := make(map[models.Platform]models.PlatformExtractor)

	if cfg.Platforms.TikTok.Enabled {
		extractors[models.PlatformTikTok] = platform.NewTikTokExtractor(&models.ExtractorConfig{
			Timeout:    time.Duration(cfg.Download.Timeout) * time.Second,
			Proxy:      getProxyURL(cfg),
			UserAgent:  cfg.Platforms.TikTok.UserAgent,
			Cookie:     cfg.Platforms.TikTok.Cookie,
			MaxRetries: cfg.Download.RetryCount,
		})
	}

	if cfg.Platforms.XHS.Enabled {
		extractors[models.PlatformXHS] = platform.NewXHSExtractor(&models.ExtractorConfig{
			Timeout:    time.Duration(cfg.Download.Timeout) * time.Second,
			Proxy:      getProxyURL(cfg),
			UserAgent:  cfg.Platforms.XHS.UserAgent,
			Cookie:     cfg.Platforms.XHS.Cookie,
			MaxRetries: cfg.Download.RetryCount,
		})
	}

	if cfg.Platforms.Kuaishou.Enabled {
		extractors[models.PlatformKuaishou] = platform.NewKuaishouExtractor(&models.ExtractorConfig{
			Timeout:    time.Duration(cfg.Download.Timeout) * time.Second,
			Proxy:      getProxyURL(cfg),
			UserAgent:  cfg.Platforms.Kuaishou.UserAgent,
			Cookie:     cfg.Platforms.Kuaishou.Cookie,
			MaxRetries: cfg.Download.RetryCount,
		})
	}

	return &Manager{
		config:     cfg,
		logger:     zerolog.New(os.Stdout).With().Timestamp().Logger(),
		storage:    storage,
		downloader: dm,
		extractors: extractors,
		queue:      make(chan *DownloadRequest, 100),
		workers:    cfg.Download.MaxWorkers,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the download manager
func (m *Manager) Start() error {
	// Start worker goroutines
	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}

	m.logger.Info().Msg("Download manager started")
	return nil
}

// Stop stops the download manager
func (m *Manager) Stop() error {
	m.cancel()
	m.wg.Wait()
	m.logger.Info().Msg("Download manager stopped")
	return nil
}

// Download downloads a video from URL
func (m *Manager) Download(url string, options *DownloadOptions) (*DownloadResult, error) {
	// Determine platform
	platform := m.detectPlatform(url)
	if platform == "" {
		return nil, fmt.Errorf("unsupported platform")
	}

	// Create download request
	req := &DownloadRequest{
		URL:      url,
		Platform: platform,
		Options:  options,
	}

	// Add to queue
	m.queue <- req

	// Wait for result (simplified - in production, use channels)
	result := <-m.processDownload(req)

	return result, nil
}

// DownloadBatch downloads multiple videos
func (m *Manager) DownloadBatch(urls []string, options *DownloadOptions) ([]*DownloadResult, error) {
	results := make([]*DownloadResult, len(urls))
	errors := make([]error, len(urls))

	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			result, err := m.Download(u, options)
			if err != nil {
				errors[idx] = err
				return
			}
			results[idx] = result
		}(i, url)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return results, fmt.Errorf("some downloads failed")
		}
	}

	return results, nil
}

// GetStatus returns the status of downloads
func (m *Manager) GetStatus() map[string]interface{} {
	jobs := m.downloader.GetActiveJobs()

	status := map[string]interface{}{
		"active_downloads": len(jobs),
		"max_workers":      m.workers,
		"queue_size":       len(m.queue),
		"jobs":             jobs,
	}

	return status
}

// worker processes download requests
func (m *Manager) worker(id int) {
	defer m.wg.Done()

	m.logger.Info().Str("worker_id", fmt.Sprintf("%d", id)).Msg("Download worker started")

	for {
		select {
		case <-m.ctx.Done():
			return
		case req := <-m.queue:
			m.processDownload(req)
		}
	}
}

// processDownload processes a download request
func (m *Manager) processDownload(req *DownloadRequest) chan *DownloadResult {
	resultChan := make(chan *DownloadResult, 1)

	go func() {
		defer close(resultChan)

		result := &DownloadResult{
			Success: false,
		}

		// Get extractor
		extractor, ok := m.extractors[req.Platform]
		if !ok {
			result.Error = fmt.Errorf("extractor not available for platform: %s", req.Platform)
			resultChan <- result
			return
		}

		// Extract video info
		videoInfo, err := extractor.ExtractVideoInfo(req.URL)
		if err != nil {
			result.Error = fmt.Errorf("error extracting video info: %w", err)
			resultChan <- result
			return
		}

		// Check if already downloaded
		existing, err := m.storage.GetVideoInfo(videoInfo.ID)
		if err == nil && existing != nil && existing.Status == "completed" {
			result.Success = true
			result.Message = "Already downloaded"
			result.Video = existing
			resultChan <- result
			return
		}

		// Save video info
		if err := m.storage.SaveVideoInfo(videoInfo); err != nil {
			m.logger.Error().Err(err).Msg("Error saving video info")
		}

		// Generate output path
		outputPath := m.generateOutputPath(videoInfo, req.Options)

		// Download file
		progressChan := make(chan float64)
		go func() {
			for progress := range progressChan {
				// Update progress in storage
				if err := m.storage.UpdateDownloadProgress(videoInfo.ID, progress); err != nil {
					m.logger.Error().Err(err).Msg("Error updating download progress")
				}
			}
		}()

		err = m.downloader.Download(videoInfo.DownloadURL, outputPath, progressChan)
		close(progressChan)

		if err != nil {
			// Update status to failed
			videoInfo.Status = "failed"
			videoInfo.ErrorMessage = err.Error()
			videoInfo.RetryCount++

			if err := m.storage.SaveVideoInfo(videoInfo); err != nil {
				m.logger.Error().Err(err).Msg("Error updating video status")
			}

			result.Error = fmt.Errorf("error downloading video: %w", err)
			resultChan <- result
			return
		}

		// Update video info with file details
		if stat, err := os.Stat(outputPath); err == nil {
			videoInfo.FileSize = stat.Size()
			videoInfo.FilePath = outputPath
			videoInfo.Status = "completed"
			now := time.Now()
			videoInfo.DownloadedAt = &now
		}

		// Save updated video info
		if err := m.storage.SaveVideoInfo(videoInfo); err != nil {
			m.logger.Error().Err(err).Msg("Error saving updated video info")
		}

		result.Success = true
		result.Message = "Download completed"
		result.Video = videoInfo
		resultChan <- result
	}()

	return resultChan
}

// detectPlatform detects the platform from URL
func (m *Manager) detectPlatform(url string) models.Platform {
	for platform, extractor := range m.extractors {
		if extractor.ValidateURL(url) {
			return platform
		}
	}
	return ""
}

// generateOutputPath generates the output file path
func (m *Manager) generateOutputPath(videoInfo *models.VideoInfo, options *DownloadOptions) string {
	// Use provided output path or generate one
	outputPath := options.OutputPath
	if outputPath == "" {
		outputPath = m.config.Download.SavePath
	}

	// Create folder if needed
	if m.config.Download.CreateFolder {
		authorFolder := fmt.Sprintf("%s_%s", videoInfo.AuthorID, videoInfo.AuthorName)
		authorFolder = utils.SanitizeFilename(authorFolder)
		outputPath = filepath.Join(outputPath, authorFolder)
	}

	// Generate filename
	filename := m.generateFilename(videoInfo, options)

	return filepath.Join(outputPath, filename)
}

// generateFilename generates filename based on configuration
func (m *Manager) generateFilename(videoInfo *models.VideoInfo, options *DownloadOptions) string {
	template := m.config.Download.FileNaming
	if template == "" {
		template = "{platform}_{author}_{title}_{id}"
	}

	// Replace placeholders
	filename := template
	filename = strings.ReplaceAll(filename, "{platform}", string(videoInfo.Platform))
	filename = strings.ReplaceAll(filename, "{author}", videoInfo.AuthorName)
	filename = strings.ReplaceAll(filename, "{title}", videoInfo.Title)
	filename = strings.ReplaceAll(filename, "{id}", videoInfo.ID)
	filename = strings.ReplaceAll(filename, "{date}", videoInfo.PublishedAt.Format("2006-01-02"))

	// Sanitize filename
	filename = utils.SanitizeFilename(filename)

	// Add extension
	ext := options.Format
	if ext == "" {
		switch videoInfo.MediaType {
		case models.MediaTypeVideo:
			ext = "mp4"
		case models.MediaTypeImage:
			ext = "jpg"
		case models.MediaTypeAudio:
			ext = "mp3"
		default:
			ext = "mp4"
		}
	}

	filename += "." + ext

	return filename
}

// getProxyURL returns proxy URL if enabled
func getProxyURL(cfg *models.Config) string {
	if !cfg.Proxy.Enabled {
		return ""
	}

	if cfg.Proxy.Username != "" && cfg.Proxy.Password != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%d",
			cfg.Proxy.Type,
			cfg.Proxy.Username,
			cfg.Proxy.Password,
			cfg.Proxy.Host,
			cfg.Proxy.Port,
		)
	}

	return fmt.Sprintf("%s://%s:%d",
		cfg.Proxy.Type,
		cfg.Proxy.Host,
		cfg.Proxy.Port,
	)
}

// GetVideoInfo retrieves video information without downloading
func (m *Manager) GetVideoInfo(url string) (*models.VideoInfo, error) {
	platform := m.detectPlatform(url)
	if platform == "" {
		return nil, fmt.Errorf("unsupported platform")
	}

	extractor, ok := m.extractors[platform]
	if !ok {
		return nil, fmt.Errorf("extractor not available for platform: %s", platform)
	}

	return extractor.ExtractVideoInfo(url)
}

// GetAuthorInfo retrieves author information
func (m *Manager) GetAuthorInfo(platform models.Platform, authorID string) (*models.AuthorInfo, error) {
	extractor, ok := m.extractors[platform]
	if !ok {
		return nil, fmt.Errorf("extractor not available for platform: %s", platform)
	}

	return extractor.ExtractAuthorInfo(authorID)
}

// ListVideos lists videos with filters
func (m *Manager) ListVideos(filter models.VideoFilter) ([]*models.VideoInfo, error) {
	return m.storage.ListVideos(filter)
}

// CancelDownload cancels a download
func (m *Manager) CancelDownload(videoID string) error {
	// This would need to track active downloads
	// For now, just update status
	video, err := m.storage.GetVideoInfo(videoID)
	if err != nil {
		return fmt.Errorf("video not found: %w", err)
	}

	video.Status = "cancelled"
	return m.storage.SaveVideoInfo(video)
}

// RetryDownload retries a failed download
func (m *Manager) RetryDownload(videoID string) error {
	video, err := m.storage.GetVideoInfo(videoID)
	if err != nil {
		return fmt.Errorf("video not found: %w", err)
	}

	if video.Status != "failed" {
		return fmt.Errorf("video is not in failed state")
	}

	// Reset status
	video.Status = "pending"
	video.RetryCount = 0
	video.ErrorMessage = ""

	if err := m.storage.SaveVideoInfo(video); err != nil {
		return fmt.Errorf("error updating video: %w", err)
	}

	// Create download request
	req := &DownloadRequest{
		URL:      video.URL,
		Platform: video.Platform,
		Options: &DownloadOptions{
			OutputPath: filepath.Dir(video.FilePath),
			Format:     video.Format,
		},
	}

	// Add to queue
	m.queue <- req

	return nil
}

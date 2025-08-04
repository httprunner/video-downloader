package batch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"video-downloader/internal/downloader"
	"video-downloader/internal/platform"
	"video-downloader/internal/registry"
	"video-downloader/pkg/models"
)

// BatchManager manages batch download operations
type BatchManager struct {
	registry      *registry.Registry
	downloader    *downloader.Manager
	logger        zerolog.Logger
	maxConcurrent int
	semaphore     chan struct{}
	workers       sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

// BatchDownloadConfig holds configuration for batch downloads
type BatchDownloadConfig struct {
	MaxConcurrent int
	OutputPath    string
	Format        string
	Quality       string
	SkipExisting  bool
	RetryFailed   bool
}

// BatchJob represents a batch download job
type BatchJob struct {
	ID          string
	Type        BatchJobType
	URLs        []string
	Config      BatchDownloadConfig
	Status      JobStatus
	Progress    BatchProgress
	Results     []BatchResult
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       error
}

// BatchJobType represents the type of batch job
type BatchJobType string

const (
	BatchJobTypeUserProfile BatchJobType = "user_profile"
	BatchJobTypePlaylist    BatchJobType = "playlist"
	BatchJobTypeURLList     BatchJobType = "url_list"
)

// JobStatus represents the status of a batch job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusPartial   JobStatus = "partial"
)

// BatchProgress tracks progress of a batch job
type BatchProgress struct {
	Total      int
	Completed  int
	Failed     int
	Skipped    int
	InProgress int
	Percentage float64
}

// BatchResult represents the result of a single item in batch
type BatchResult struct {
	URL       string
	VideoInfo *models.VideoInfo
	Status    string
	Error     error
	FilePath  string
	Size      int64
	Duration  time.Duration
}

// NewBatchManager creates a new batch manager
func NewBatchManager(reg *registry.Registry, dm *downloader.Manager, maxConcurrent int) *BatchManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &BatchManager{
		registry:      reg,
		downloader:    dm,
		logger:        zerolog.New(nil).With().Str("component", "batch_manager").Logger(),
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// StartBatchDownload starts a batch download job
func (bm *BatchManager) StartBatchDownload(jobType BatchJobType, urls []string, config BatchDownloadConfig) (*BatchJob, error) {
	job := &BatchJob{
		ID:        fmt.Sprintf("batch_%d", time.Now().Unix()),
		Type:      jobType,
		URLs:      urls,
		Config:    config,
		Status:    JobStatusPending,
		Progress:  BatchProgress{Total: len(urls)},
		Results:   make([]BatchResult, 0),
		StartedAt: time.Now(),
	}

	// Start the job asynchronously
	go bm.processBatchJob(job)

	return job, nil
}

// processBatchJob processes a batch download job
func (bm *BatchManager) processBatchJob(job *BatchJob) {
	bm.logger.Info().Str("job_id", job.ID).Msg("Starting batch job")

	job.Status = JobStatusRunning
	defer func() {
		now := time.Now()
		job.CompletedAt = &now
		bm.updateJobStatus(job)
		bm.logger.Info().Str("job_id", job.ID).Str("status", string(job.Status)).Msg("Batch job completed")
	}()

	switch job.Type {
	case BatchJobTypeUserProfile:
		bm.processUserProfile(job)
	case BatchJobTypePlaylist:
		bm.processPlaylist(job)
	case BatchJobTypeURLList:
		bm.processURLList(job)
	default:
		job.Status = JobStatusFailed
		job.Error = fmt.Errorf("unsupported job type: %s", job.Type)
		return
	}

	bm.updateJobStatus(job)
}

// processUserProfile processes user profile batch download
func (bm *BatchManager) processUserProfile(job *BatchJob) {
	var allVideos []*models.VideoInfo
	var errors []error

	// Extract videos from each profile URL
	for _, url := range job.URLs {
		extractor, err := bm.getExtractorForURL(url)
		if err != nil {
			errors = append(errors, fmt.Errorf("no extractor for URL %s: %w", url, err))
			continue
		}

		videos, err := extractor.ExtractBatch(url, 100) // Default limit of 100
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to extract batch from %s: %w", url, err))
			continue
		}

		allVideos = append(allVideos, videos...)
	}

	if len(allVideos) == 0 {
		job.Status = JobStatusFailed
		job.Error = fmt.Errorf("no videos found in user profiles")
		return
	}

	// Update job with extracted videos
	job.Progress.Total = len(allVideos)

	// Download videos
	bm.downloadVideos(job, allVideos)
}

// processPlaylist processes playlist batch download
func (bm *BatchManager) processPlaylist(job *BatchJob) {
	// Similar to user profile but for playlists
	// This would need platform-specific playlist extraction
	job.Status = JobStatusFailed
	job.Error = fmt.Errorf("playlist batch download not implemented")
}

// processURLList processes URL list batch download
func (bm *BatchManager) processURLList(job *BatchJob) {
	var allVideos []*models.VideoInfo

	// Extract video info from each URL
	for _, url := range job.URLs {
		extractor, err := bm.getExtractorForURL(url)
		if err != nil {
			job.Results = append(job.Results, BatchResult{
				URL:    url,
				Status: "failed",
				Error:  fmt.Errorf("no extractor for URL: %w", err),
			})
			job.Progress.Failed++
			continue
		}

		videoInfo, err := extractor.ExtractVideoInfo(url)
		if err != nil {
			job.Results = append(job.Results, BatchResult{
				URL:    url,
				Status: "failed",
				Error:  fmt.Errorf("failed to extract video info: %w", err),
			})
			job.Progress.Failed++
			continue
		}

		allVideos = append(allVideos, videoInfo)
	}

	if len(allVideos) == 0 {
		job.Status = JobStatusFailed
		job.Error = fmt.Errorf("no valid videos found in URL list")
		return
	}

	// Download videos
	bm.downloadVideos(job, allVideos)
}

// downloadVideos downloads a list of videos
func (bm *BatchManager) downloadVideos(job *BatchJob, videos []*models.VideoInfo) {
	// Create download tasks
	for _, video := range videos {
		bm.workers.Add(1)
		go func(v *models.VideoInfo) {
			defer bm.workers.Done()

			// Acquire semaphore
			bm.semaphore <- struct{}{}
			defer func() { <-bm.semaphore }()

			// Check if context is cancelled
			select {
			case <-bm.ctx.Done():
				job.Progress.Skipped++
				return
			default:
			}

			result := BatchResult{
				URL:       v.URL,
				VideoInfo: v,
				Status:    "downloading",
			}

			start := time.Now()

			// Create download task
			task := &models.DownloadTask{
				ID:        fmt.Sprintf("dl_%s_%d", job.ID, time.Now().UnixNano()),
				VideoID:   v.ID,
				URL:       v.DownloadURL,
				Platform:  v.Platform,
				Status:    "pending",
				Progress:  0,
				FilePath:  bm.generateFilePath(v, job.Config),
				CreatedAt: time.Now(),
			}

			// Check if file already exists and skip if configured
			if job.Config.SkipExisting && bm.fileExists(task.FilePath) {
				result.Status = "skipped"
				result.FilePath = task.FilePath
				job.Progress.Skipped++
				job.Results = append(job.Results, result)
				return
			}

			// Start download
			progressChan := make(chan float64, 1)
			go bm.monitorProgress(progressChan, task)

			_, err := bm.downloader.Download(task.URL, &downloader.DownloadOptions{
				OutputPath: task.FilePath,
			})
			result.Duration = time.Since(start)

			if err != nil {
				result.Status = "failed"
				result.Error = err
				job.Progress.Failed++
				bm.logger.Error().Err(err).Str("url", v.URL).Msg("Download failed")
			} else {
				result.Status = "completed"
				result.FilePath = task.FilePath
				result.Size = bm.getFileSize(task.FilePath)
				job.Progress.Completed++
				bm.logger.Info().Str("url", v.URL).Str("file", task.FilePath).Msg("Download completed")
			}

			job.Results = append(job.Results, result)
			job.Progress.InProgress--
			bm.updateProgress(job)
		}(video)

		job.Progress.InProgress++
	}

	// Wait for all downloads to complete
	bm.workers.Wait()
}

// getExtractorForURL returns the appropriate extractor for a URL
func (bm *BatchManager) getExtractorForURL(url string) (models.PlatformExtractor, error) {
	// Check TikTok
	tiktokExtractor := platform.NewTikTokExtractor(&models.ExtractorConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	})
	if tiktokExtractor.ValidateURL(url) {
		return tiktokExtractor, nil
	}

	// Check XHS
	xhsExtractor := platform.NewXHSExtractor(&models.ExtractorConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	})
	if xhsExtractor.ValidateURL(url) {
		return xhsExtractor, nil
	}

	// Check Kuaishou
	kuaishouExtractor := platform.NewKuaishouExtractor(&models.ExtractorConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	})
	if kuaishouExtractor.ValidateURL(url) {
		return kuaishouExtractor, nil
	}

	return nil, fmt.Errorf("no supported extractor found for URL: %s", url)
}

// generateFilePath generates a file path for a video
func (bm *BatchManager) generateFilePath(video *models.VideoInfo, config BatchDownloadConfig) string {
	// Generate filename based on template
	filename := fmt.Sprintf("%s_%s_%s_%s.%s",
		video.Platform,
		video.AuthorName,
		video.Title,
		video.ID,
		video.Format,
	)

	// Clean filename
	filename = bm.cleanFilename(filename)

	return fmt.Sprintf("%s/%s", config.OutputPath, filename)
}

// cleanFilename removes invalid characters from filename
func (bm *BatchManager) cleanFilename(filename string) string {
	// Remove invalid characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	cleanName := filename
	for _, invalidChar := range invalidChars {
		cleanName = strings.ReplaceAll(cleanName, invalidChar, "_")
	}
	return cleanName
}

// fileExists checks if a file exists
func (bm *BatchManager) fileExists(filepath string) bool {
	// Implementation would check file existence
	return false
}

// getFileSize returns the size of a file
func (bm *BatchManager) getFileSize(filepath string) int64 {
	// Implementation would return actual file size
	return 0
}

// monitorProgress monitors download progress
func (bm *BatchManager) monitorProgress(progressChan <-chan float64, task *models.DownloadTask) {
	for progress := range progressChan {
		task.Progress = progress
		// Update task in database if needed
	}
}

// updateProgress updates job progress
func (bm *BatchManager) updateProgress(job *BatchJob) {
	total := float64(job.Progress.Total)
	if total > 0 {
		job.Progress.Percentage = float64(job.Progress.Completed+job.Progress.Failed+job.Progress.Skipped) / total * 100
	}
}

// updateJobStatus updates the final job status
func (bm *BatchManager) updateJobStatus(job *BatchJob) {
	if job.Progress.Failed > 0 && job.Progress.Completed > 0 {
		job.Status = JobStatusPartial
	} else if job.Progress.Failed == job.Progress.Total {
		job.Status = JobStatusFailed
	} else if job.Progress.Completed+job.Progress.Skipped == job.Progress.Total {
		job.Status = JobStatusCompleted
	}
}

// CancelJob cancels a running batch job
func (bm *BatchManager) CancelJob(jobID string) error {
	// In a real implementation, you would track jobs and cancel them
	bm.cancel()
	return nil
}

// GetJobStatus returns the status of a batch job
func (bm *BatchManager) GetJobStatus(jobID string) (*BatchJob, error) {
	// In a real implementation, you would store and retrieve job status
	return nil, fmt.Errorf("job not found: %s", jobID)
}

// Close shuts down the batch manager
func (bm *BatchManager) Close() error {
	bm.cancel()
	bm.workers.Wait()
	return nil
}

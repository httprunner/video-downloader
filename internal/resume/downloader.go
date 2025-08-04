package resume

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ResumableDownloader handles resumable downloads with metadata tracking
type ResumableDownloader struct {
	client     *http.Client
	logger     zerolog.Logger
	metaDir    string
	tempDir    string
	chunkSize  int64
	maxRetries int
	timeout    time.Duration
	activeJobs map[string]*ResumableJob
	jobsMutex  sync.RWMutex
}

// ResumableJob represents a resumable download job
type ResumableJob struct {
	ID           string                `json:"id"`
	URL          string                `json:"url"`
	FilePath     string                `json:"file_path"`
	TempPath     string                `json:"temp_path"`
	MetaPath     string                `json:"meta_path"`
	FileSize     int64                 `json:"file_size"`
	Downloaded   int64                 `json:"downloaded"`
	Status       string                `json:"status"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	CompletedAt  *time.Time            `json:"completed_at"`
	Headers      map[string]string     `json:"headers"`
	Checksum     string                `json:"checksum"`
	RetryCount   int                   `json:"retry_count"`
	LastError    string                `json:"last_error"`
	Progress     float64               `json:"progress"`
	Speed        float64               `json:"speed"`
	ETA          time.Duration         `json:"eta"`
	ctx          context.Context       `json:"-"`
	cancel       context.CancelFunc    `json:"-"`
	progressChan chan<- ProgressUpdate `json:"-"`
	mutex        sync.RWMutex          `json:"-"`
}

// ProgressUpdate represents a progress update
type ProgressUpdate struct {
	JobID      string
	Progress   float64
	Downloaded int64
	FileSize   int64
	Speed      float64
	ETA        time.Duration
	Status     string
}

// ResumableConfig holds configuration for resumable downloader
type ResumableConfig struct {
	MetaDir    string
	TempDir    string
	ChunkSize  int64
	MaxRetries int
	Timeout    time.Duration
	UserAgent  string
}

// NewResumableDownloader creates a new resumable downloader
func NewResumableDownloader(config ResumableConfig) *ResumableDownloader {
	if config.MetaDir == "" {
		config.MetaDir = "./temp/metadata"
	}
	if config.TempDir == "" {
		config.TempDir = "./temp/downloads"
	}
	if config.ChunkSize == 0 {
		config.ChunkSize = 1024 * 1024 // 1MB
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	rd := &ResumableDownloader{
		client:     client,
		logger:     zerolog.New(nil).With().Str("component", "resumable_downloader").Logger(),
		metaDir:    config.MetaDir,
		tempDir:    config.TempDir,
		chunkSize:  config.ChunkSize,
		maxRetries: config.MaxRetries,
		timeout:    config.Timeout,
		activeJobs: make(map[string]*ResumableJob),
	}

	// Create directories
	os.MkdirAll(config.MetaDir, 0755)
	os.MkdirAll(config.TempDir, 0755)

	// Load existing jobs
	rd.loadExistingJobs()

	return rd
}

// StartDownload starts a new resumable download
func (rd *ResumableDownloader) StartDownload(url, filePath string, headers map[string]string, progressChan chan<- ProgressUpdate) (*ResumableJob, error) {
	// Generate job ID
	jobID := rd.generateJobID(url, filePath)

	// Check if job already exists
	rd.jobsMutex.Lock()
	if existingJob, exists := rd.activeJobs[jobID]; exists {
		rd.jobsMutex.Unlock()
		if existingJob.Status == "completed" {
			return existingJob, nil
		}
		// Resume existing job
		return rd.resumeDownload(existingJob, progressChan)
	}
	rd.jobsMutex.Unlock()

	// Create new job
	job := &ResumableJob{
		ID:           jobID,
		URL:          url,
		FilePath:     filePath,
		TempPath:     filepath.Join(rd.tempDir, jobID+".tmp"),
		MetaPath:     filepath.Join(rd.metaDir, jobID+".json"),
		Headers:      headers,
		Status:       "pending",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		progressChan: progressChan,
	}

	// Create context
	job.ctx, job.cancel = context.WithCancel(context.Background())

	// Add to active jobs
	rd.jobsMutex.Lock()
	rd.activeJobs[jobID] = job
	rd.jobsMutex.Unlock()

	// Start download
	go rd.downloadJob(job)

	return job, nil
}

// ResumeDownload resumes a paused download
func (rd *ResumableDownloader) ResumeDownload(jobID string, progressChan chan<- ProgressUpdate) (*ResumableJob, error) {
	rd.jobsMutex.RLock()
	job, exists := rd.activeJobs[jobID]
	rd.jobsMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return rd.resumeDownload(job, progressChan)
}

// resumeDownload resumes an existing job
func (rd *ResumableDownloader) resumeDownload(job *ResumableJob, progressChan chan<- ProgressUpdate) (*ResumableJob, error) {
	job.mutex.Lock()
	defer job.mutex.Unlock()

	if job.Status == "downloading" {
		return job, nil // Already downloading
	}

	if job.Status == "completed" {
		return job, nil // Already completed
	}

	// Update progress channel
	job.progressChan = progressChan

	// Create new context
	job.ctx, job.cancel = context.WithCancel(context.Background())

	// Resume download
	go rd.downloadJob(job)

	return job, nil
}

// downloadJob performs the actual download
func (rd *ResumableDownloader) downloadJob(job *ResumableJob) {
	rd.logger.Info().Str("job_id", job.ID).Str("url", job.URL).Msg("Starting download")

	job.mutex.Lock()
	job.Status = "initializing"
	job.UpdatedAt = time.Now()
	job.mutex.Unlock()

	// Save initial metadata
	rd.saveJobMetadata(job)

	// Get file information
	if err := rd.getFileInfo(job); err != nil {
		rd.handleJobError(job, fmt.Errorf("failed to get file info: %w", err))
		return
	}

	// Check if file already exists
	if rd.fileExistsAndValid(job) {
		rd.completeJob(job)
		return
	}

	// Start downloading
	job.mutex.Lock()
	job.Status = "downloading"
	job.UpdatedAt = time.Now()
	job.mutex.Unlock()

	rd.saveJobMetadata(job)

	// Perform download with retries
	for attempt := 0; attempt <= rd.maxRetries; attempt++ {
		if attempt > 0 {
			rd.logger.Warn().
				Str("job_id", job.ID).
				Int("attempt", attempt).
				Msg("Retrying download")

			job.mutex.Lock()
			job.RetryCount = attempt
			job.mutex.Unlock()
		}

		err := rd.performDownload(job)
		if err == nil {
			rd.completeJob(job)
			return
		}

		if job.ctx.Err() != nil {
			// Context cancelled
			job.mutex.Lock()
			job.Status = "paused"
			job.LastError = "download cancelled"
			job.UpdatedAt = time.Now()
			job.mutex.Unlock()
			rd.saveJobMetadata(job)
			return
		}

		rd.logger.Error().
			Err(err).
			Str("job_id", job.ID).
			Int("attempt", attempt).
			Msg("Download attempt failed")

		job.mutex.Lock()
		job.LastError = err.Error()
		job.UpdatedAt = time.Now()
		job.mutex.Unlock()

		if attempt < rd.maxRetries {
			// Wait before retry
			select {
			case <-time.After(time.Duration(attempt+1) * time.Second):
			case <-job.ctx.Done():
				return
			}
		}
	}

	// All retries failed
	rd.handleJobError(job, fmt.Errorf("download failed after %d attempts", rd.maxRetries))
}

// getFileInfo retrieves file information from the server
func (rd *ResumableDownloader) getFileInfo(job *ResumableJob) error {
	req, err := http.NewRequestWithContext(job.ctx, "HEAD", job.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HEAD request: %w", err)
	}

	// Set headers
	for key, value := range job.Headers {
		req.Header.Set(key, value)
	}

	resp, err := rd.client.Do(req)
	if err != nil {
		return fmt.Errorf("HEAD request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HEAD request returned status %d", resp.StatusCode)
	}

	// Get file size
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		if size, err := parseFileSize(contentLength); err == nil {
			job.mutex.Lock()
			job.FileSize = size
			job.mutex.Unlock()
		}
	}

	// Check if server supports range requests
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		rd.logger.Warn().Str("job_id", job.ID).Msg("Server does not support range requests")
	}

	return nil
}

// performDownload performs the actual file download
func (rd *ResumableDownloader) performDownload(job *ResumableJob) error {
	// Check current downloaded size
	var startByte int64
	if stat, err := os.Stat(job.TempPath); err == nil {
		startByte = stat.Size()
		job.mutex.Lock()
		job.Downloaded = startByte
		job.mutex.Unlock()
	}

	// Create request
	req, err := http.NewRequestWithContext(job.ctx, "GET", job.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}

	// Set headers
	for key, value := range job.Headers {
		req.Header.Set(key, value)
	}

	// Set range header for resume
	if startByte > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))
	}

	// Make request
	resp, err := rd.client.Do(req)
	if err != nil {
		return fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	expectedStatus := http.StatusOK
	if startByte > 0 {
		expectedStatus = http.StatusPartialContent
	}

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Open temp file for writing
	file, err := os.OpenFile(job.TempPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	// Create progress writer
	pw := &progressWriter{
		writer:   file,
		job:      job,
		rd:       rd,
		lastTime: time.Now(),
	}

	// Copy data
	_, err = io.Copy(pw, resp.Body)
	if err != nil && job.ctx.Err() == nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}

// completeJob completes a download job
func (rd *ResumableDownloader) completeJob(job *ResumableJob) {
	rd.logger.Info().Str("job_id", job.ID).Msg("Download completed")

	// Verify file integrity
	if err := rd.verifyFile(job); err != nil {
		rd.handleJobError(job, fmt.Errorf("file verification failed: %w", err))
		return
	}

	// Move file to final location
	if err := os.MkdirAll(filepath.Dir(job.FilePath), 0755); err != nil {
		rd.handleJobError(job, fmt.Errorf("failed to create directory: %w", err))
		return
	}

	if err := os.Rename(job.TempPath, job.FilePath); err != nil {
		rd.handleJobError(job, fmt.Errorf("failed to move file: %w", err))
		return
	}

	// Update job status
	job.mutex.Lock()
	job.Status = "completed"
	job.Progress = 100.0
	now := time.Now()
	job.CompletedAt = &now
	job.UpdatedAt = now
	job.mutex.Unlock()

	// Save metadata
	rd.saveJobMetadata(job)

	// Send final progress update
	if job.progressChan != nil {
		job.progressChan <- ProgressUpdate{
			JobID:    job.ID,
			Progress: 100.0,
			Status:   "completed",
		}
	}

	// Clean up metadata file
	os.Remove(job.MetaPath)
}

// handleJobError handles job errors
func (rd *ResumableDownloader) handleJobError(job *ResumableJob, err error) {
	rd.logger.Error().Err(err).Str("job_id", job.ID).Msg("Job failed")

	job.mutex.Lock()
	job.Status = "failed"
	job.LastError = err.Error()
	job.UpdatedAt = time.Now()
	job.mutex.Unlock()

	rd.saveJobMetadata(job)

	if job.progressChan != nil {
		job.progressChan <- ProgressUpdate{
			JobID:  job.ID,
			Status: "failed",
		}
	}
}

// verifyFile verifies the downloaded file
func (rd *ResumableDownloader) verifyFile(job *ResumableJob) error {
	// Check file size
	stat, err := os.Stat(job.TempPath)
	if err != nil {
		return fmt.Errorf("failed to stat temp file: %w", err)
	}

	if job.FileSize > 0 && stat.Size() != job.FileSize {
		return fmt.Errorf("file size mismatch: expected %d, got %d", job.FileSize, stat.Size())
	}

	// Verify checksum if available
	if job.Checksum != "" {
		actualChecksum, err := rd.calculateChecksum(job.TempPath)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum: %w", err)
		}

		if actualChecksum != job.Checksum {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", job.Checksum, actualChecksum)
		}
	}

	return nil
}

// calculateChecksum calculates MD5 checksum of a file
func (rd *ResumableDownloader) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// fileExistsAndValid checks if the file already exists and is valid
func (rd *ResumableDownloader) fileExistsAndValid(job *ResumableJob) bool {
	stat, err := os.Stat(job.FilePath)
	if err != nil {
		return false
	}

	// Check file size
	if job.FileSize > 0 && stat.Size() != job.FileSize {
		return false
	}

	return true
}

// saveJobMetadata saves job metadata to disk
func (rd *ResumableDownloader) saveJobMetadata(job *ResumableJob) {
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		rd.logger.Error().Err(err).Str("job_id", job.ID).Msg("Failed to marshal job metadata")
		return
	}

	if err := os.WriteFile(job.MetaPath, data, 0644); err != nil {
		rd.logger.Error().Err(err).Str("job_id", job.ID).Msg("Failed to save job metadata")
	}
}

// loadExistingJobs loads existing jobs from metadata
func (rd *ResumableDownloader) loadExistingJobs() {
	files, err := filepath.Glob(filepath.Join(rd.metaDir, "*.json"))
	if err != nil {
		rd.logger.Error().Err(err).Msg("Failed to glob metadata files")
		return
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			rd.logger.Error().Err(err).Str("file", file).Msg("Failed to read metadata file")
			continue
		}

		var job ResumableJob
		if err := json.Unmarshal(data, &job); err != nil {
			rd.logger.Error().Err(err).Str("file", file).Msg("Failed to unmarshal metadata")
			continue
		}

		// Set metadata path
		job.MetaPath = file

		rd.activeJobs[job.ID] = &job
	}

	rd.logger.Info().Int("count", len(rd.activeJobs)).Msg("Loaded existing jobs")
}

// generateJobID generates a unique job ID
func (rd *ResumableDownloader) generateJobID(url, filePath string) string {
	data := fmt.Sprintf("%s:%s", url, filePath)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GetJob returns a job by ID
func (rd *ResumableDownloader) GetJob(jobID string) (*ResumableJob, bool) {
	rd.jobsMutex.RLock()
	defer rd.jobsMutex.RUnlock()

	job, exists := rd.activeJobs[jobID]
	return job, exists
}

// GetAllJobs returns all jobs
func (rd *ResumableDownloader) GetAllJobs() []*ResumableJob {
	rd.jobsMutex.RLock()
	defer rd.jobsMutex.RUnlock()

	jobs := make([]*ResumableJob, 0, len(rd.activeJobs))
	for _, job := range rd.activeJobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// PauseJob pauses a download job
func (rd *ResumableDownloader) PauseJob(jobID string) error {
	rd.jobsMutex.RLock()
	job, exists := rd.activeJobs[jobID]
	rd.jobsMutex.RUnlock()

	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.mutex.Lock()
	defer job.mutex.Unlock()

	if job.cancel != nil {
		job.cancel()
	}

	job.Status = "paused"
	job.UpdatedAt = time.Now()

	rd.saveJobMetadata(job)
	return nil
}

// DeleteJob deletes a job
func (rd *ResumableDownloader) DeleteJob(jobID string) error {
	rd.jobsMutex.Lock()
	defer rd.jobsMutex.Unlock()

	job, exists := rd.activeJobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Cancel if running
	if job.cancel != nil {
		job.cancel()
	}

	// Remove files
	os.Remove(job.TempPath)
	os.Remove(job.MetaPath)

	// Remove from active jobs
	delete(rd.activeJobs, jobID)

	return nil
}

// progressWriter wraps a writer to track progress
type progressWriter struct {
	writer      io.Writer
	job         *ResumableJob
	rd          *ResumableDownloader
	written     int64
	lastTime    time.Time
	lastWritten int64
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	if n > 0 {
		pw.written += int64(n)

		pw.job.mutex.Lock()
		pw.job.Downloaded += int64(n)

		// Calculate progress
		if pw.job.FileSize > 0 {
			pw.job.Progress = float64(pw.job.Downloaded) / float64(pw.job.FileSize) * 100
		}

		// Calculate speed and ETA
		now := time.Now()
		if now.Sub(pw.lastTime) >= time.Second {
			elapsed := now.Sub(pw.lastTime)
			bytes := pw.written - pw.lastWritten
			pw.job.Speed = float64(bytes) / elapsed.Seconds()

			if pw.job.Speed > 0 && pw.job.FileSize > 0 {
				remaining := pw.job.FileSize - pw.job.Downloaded
				pw.job.ETA = time.Duration(float64(remaining)/pw.job.Speed) * time.Second
			}

			pw.lastTime = now
			pw.lastWritten = pw.written
		}

		pw.job.UpdatedAt = now
		pw.job.mutex.Unlock()

		// Send progress update
		if pw.job.progressChan != nil {
			select {
			case pw.job.progressChan <- ProgressUpdate{
				JobID:      pw.job.ID,
				Progress:   pw.job.Progress,
				Downloaded: pw.job.Downloaded,
				FileSize:   pw.job.FileSize,
				Speed:      pw.job.Speed,
				ETA:        pw.job.ETA,
				Status:     pw.job.Status,
			}:
			default:
				// Don't block if channel is full
			}
		}

		// Save metadata periodically
		if pw.written%pw.rd.chunkSize == 0 {
			pw.rd.saveJobMetadata(pw.job)
		}
	}

	return n, err
}

// parseFileSize parses file size from string
func parseFileSize(s string) (int64, error) {
	var size int64
	_, err := fmt.Sscanf(s, "%d", &size)
	return size, err
}

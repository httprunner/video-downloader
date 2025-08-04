package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// DownloadManager manages concurrent downloads
type DownloadManager struct {
	client     *HTTPClient
	logger     zerolog.Logger
	maxWorkers int
	chunkSize  int64
	retryCount int
	tempDir    string
	activeJobs map[string]*DownloadJob
	jobsMutex  sync.Mutex
	progress   map[string]float64
	progressMu sync.Mutex
}

// DownloadJob represents a single download job
type DownloadJob struct {
	ID         string
	URL        string
	FilePath   string
	FileSize   int64
	Downloaded int64
	Speed      float64
	ETA        time.Duration
	Status     string
	Error      error
	StartTime  time.Time
	EndTime    *time.Time
	CancelFunc context.CancelFunc
	Progress   chan float64
}

// DownloadConfig represents download configuration
type DownloadConfig struct {
	MaxWorkers int
	ChunkSize  int64
	RetryCount int
	TempDir    string
	Timeout    time.Duration
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(config DownloadConfig) *DownloadManager {
	if config.TempDir == "" {
		config.TempDir = "./temp"
	}

	return &DownloadManager{
		client:     NewHTTPClient(ClientConfig{Timeout: config.Timeout}),
		logger:     zerolog.New(os.Stdout).With().Timestamp().Logger(),
		maxWorkers: config.MaxWorkers,
		chunkSize:  config.ChunkSize,
		retryCount: config.RetryCount,
		tempDir:    config.TempDir,
		activeJobs: make(map[string]*DownloadJob),
		progress:   make(map[string]float64),
	}
}

// Download downloads a file to the specified path
func (dm *DownloadManager) Download(url, filePath string, progressChan chan<- float64) error {
	// Create job
	job := &DownloadJob{
		ID:        generateJobID(),
		URL:       url,
		FilePath:  filePath,
		Status:    "pending",
		StartTime: time.Now(),
		Progress:  make(chan float64),
	}

	// Add to active jobs
	dm.jobsMutex.Lock()
	dm.activeJobs[job.ID] = job
	dm.jobsMutex.Unlock()

	// Start download in goroutine
	go dm.downloadFile(job, progressChan)

	return nil
}

// downloadFile performs the actual file download
func (dm *DownloadManager) downloadFile(job *DownloadJob, progressChan chan<- float64) {
	defer close(job.Progress)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	job.CancelFunc = cancel
	defer cancel()

	// Get file size
	size, err := dm.client.GetFileSize(job.URL)
	if err != nil {
		job.Error = fmt.Errorf("error getting file size: %w", err)
		job.Status = "failed"
		return
	}

	job.FileSize = size
	job.Status = "downloading"

	// Create temp file
	tempDir := dm.tempDir
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		job.Error = fmt.Errorf("error creating temp directory: %w", err)
		job.Status = "failed"
		return
	}

	tempFile := filepath.Join(tempDir, job.ID+".tmp")
	file, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		job.Error = fmt.Errorf("error creating temp file: %w", err)
		job.Status = "failed"
		return
	}
	defer file.Close()

	// Get current file size (for resume)
	stat, err := file.Stat()
	if err != nil {
		job.Error = fmt.Errorf("error getting file stat: %w", err)
		job.Status = "failed"
		return
	}

	job.Downloaded = stat.Size()

	// Create request with range header
	req, err := http.NewRequestWithContext(ctx, "GET", job.URL, nil)
	if err != nil {
		job.Error = fmt.Errorf("error creating request: %w", err)
		job.Status = "failed"
		return
	}

	if job.Downloaded > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", job.Downloaded))
	}

	// Set common headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	// Make request
	resp, err := dm.client.client.Do(req)
	if err != nil {
		job.Error = fmt.Errorf("error making request: %w", err)
		job.Status = "failed"
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		job.Error = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		job.Status = "failed"
		return
	}

	// Create progress reader
	reader := &ProgressReader{
		Reader:    resp.Body,
		Total:     job.FileSize,
		Completed: job.Downloaded,
		OnProgress: func(completed int64) {
			job.Downloaded = completed
			progress := float64(completed) / float64(job.FileSize) * 100
			dm.progressMu.Lock()
			dm.progress[job.ID] = progress
			dm.progressMu.Unlock()

			if progressChan != nil {
				progressChan <- progress
			}

			// Calculate speed and ETA
			elapsed := time.Since(job.StartTime)
			if elapsed > 0 {
				job.Speed = float64(completed-job.Downloaded) / elapsed.Seconds()
				if job.Speed > 0 {
					remaining := float64(job.FileSize-completed) / job.Speed
					job.ETA = time.Duration(remaining) * time.Second
				}
			}
		},
	}

	// Copy file
	startTime := time.Now()
	if _, err := io.Copy(file, reader); err != nil {
		job.Error = fmt.Errorf("error downloading file: %w", err)
		job.Status = "failed"
		return
	}

	// Move file to final location
	if err := os.MkdirAll(filepath.Dir(job.FilePath), 0755); err != nil {
		job.Error = fmt.Errorf("error creating directory: %w", err)
		job.Status = "failed"
		return
	}

	if err := os.Rename(tempFile, job.FilePath); err != nil {
		job.Error = fmt.Errorf("error moving file: %w", err)
		job.Status = "failed"
		return
	}

	// Update job status
	job.Status = "completed"
	now := time.Now()
	job.EndTime = &now

	dm.logger.Info().
		Str("job_id", job.ID).
		Str("file_size", FormatBytes(job.FileSize)).
		Str("duration", FormatDuration(now.Sub(startTime))).
		Str("speed", FormatBytes(int64(job.Speed))+"/s").
		Msg("Download completed")
}

// GetJobStatus returns the status of a download job
func (dm *DownloadManager) GetJobStatus(jobID string) (*DownloadJob, bool) {
	dm.jobsMutex.Lock()
	defer dm.jobsMutex.Unlock()

	job, exists := dm.activeJobs[jobID]
	if !exists {
		return nil, false
	}

	return job, true
}

// CancelJob cancels a download job
func (dm *DownloadManager) CancelJob(jobID string) error {
	dm.jobsMutex.Lock()
	defer dm.jobsMutex.Unlock()

	job, exists := dm.activeJobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.CancelFunc != nil {
		job.CancelFunc()
	}

	job.Status = "cancelled"
	return nil
}

// GetActiveJobs returns all active download jobs
func (dm *DownloadManager) GetActiveJobs() []*DownloadJob {
	dm.jobsMutex.Lock()
	defer dm.jobsMutex.Unlock()

	jobs := make([]*DownloadJob, 0, len(dm.activeJobs))
	for _, job := range dm.activeJobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// ProgressReader is a reader that reports progress
type ProgressReader struct {
	Reader      io.Reader
	Total       int64
	Completed   int64
	OnProgress  func(int64)
	lastReport  int64
	reportEvery int64
}

// Read implements the io.Reader interface
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	if n > 0 {
		pr.Completed += int64(n)

		// Report progress
		if pr.OnProgress != nil && (pr.Completed-pr.lastReport) >= pr.reportEvery {
			pr.OnProgress(pr.Completed)
			pr.lastReport = pr.Completed
		}
	}

	return n, err
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// M3U8Downloader downloads HLS streams
type M3U8Downloader struct {
	client     *HTTPClient
	logger     zerolog.Logger
	tempDir    string
	maxWorkers int
}

// NewM3U8Downloader creates a new M3U8 downloader
func NewM3U8Downloader(config DownloadConfig) *M3U8Downloader {
	return &M3U8Downloader{
		client:     NewHTTPClient(ClientConfig{Timeout: config.Timeout}),
		logger:     zerolog.New(os.Stdout).With().Timestamp().Logger(),
		tempDir:    config.TempDir,
		maxWorkers: config.MaxWorkers,
	}
}

// DownloadM3U8 downloads an M3U8 playlist and merges the segments
func (md *M3U8Downloader) DownloadM3U8(m3u8URL, outputPath string, progressChan chan<- float64) error {
	// Download M3U8 playlist
	resp, err := md.client.Get(m3u8URL, nil)
	if err != nil {
		return fmt.Errorf("error downloading M3U8 playlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse M3U8 playlist
	scanner := bufio.NewScanner(resp.Body)
	var segments []string
	var baseURI = m3u8URL[:strings.LastIndex(m3u8URL, "/")+1]

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "http") {
				segments = append(segments, line)
			} else {
				segments = append(segments, baseURI+line)
			}
		}
	}

	if len(segments) == 0 {
		return fmt.Errorf("no segments found in M3U8 playlist")
	}

	// Create temp directory
	tempDir := filepath.Join(md.tempDir, "m3u8_"+strconv.FormatInt(time.Now().Unix(), 10))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download segments
	segmentFiles := make([]string, len(segments))
	sem := make(chan struct{}, md.maxWorkers)
	var wg sync.WaitGroup
	var downloadErrors []error
	var errorsMu sync.Mutex

	for i, segmentURL := range segments {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			segmentFile := filepath.Join(tempDir, fmt.Sprintf("segment_%04d.ts", index))
			if err := md.downloadSegment(url, segmentFile); err != nil {
				errorsMu.Lock()
				downloadErrors = append(downloadErrors, err)
				errorsMu.Unlock()
				md.logger.Error().Err(err).Msg("Error downloading segment")
				return
			}
			segmentFiles[index] = segmentFile

			// Report progress
			if progressChan != nil {
				progress := float64(index+1) / float64(len(segments)) * 100
				progressChan <- progress
			}
		}(i, segmentURL)
	}

	wg.Wait()

	if len(downloadErrors) > 0 {
		return fmt.Errorf("errors occurred while downloading segments: %v", downloadErrors)
	}

	// Merge segments
	return md.mergeSegments(segmentFiles, outputPath)
}

// downloadSegment downloads a single segment
func (md *M3U8Downloader) downloadSegment(url, filePath string) error {
	resp, err := md.client.Get(url, nil)
	if err != nil {
		return fmt.Errorf("error downloading segment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating segment file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// mergeSegments merges TS segments into a single file
func (md *M3U8Downloader) mergeSegments(segmentFiles []string, outputPath string) error {
	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer output.Close()

	for _, segmentFile := range segmentFiles {
		segment, err := os.Open(segmentFile)
		if err != nil {
			return fmt.Errorf("error opening segment file: %w", err)
		}

		if _, err := io.Copy(output, segment); err != nil {
			segment.Close()
			return fmt.Errorf("error copying segment: %w", err)
		}
		segment.Close()
	}

	return nil
}

package models

import "time"

// PlatformExtractor defines the interface for platform-specific extractors
type PlatformExtractor interface {
	// ExtractVideoInfo extracts video information from a URL
	ExtractVideoInfo(url string) (*VideoInfo, error)
	
	// ExtractAuthorInfo extracts author information
	ExtractAuthorInfo(authorID string) (*AuthorInfo, error)
	
	// ExtractBatch extracts multiple videos from a page
	ExtractBatch(url string, limit int) ([]*VideoInfo, error)
	
	// ValidateURL validates if the URL belongs to this platform
	ValidateURL(url string) bool
	
	// GetName returns the platform name
	GetName() Platform
	
	// GetSupportedURLPatterns returns supported URL patterns
	GetSupportedURLPatterns() []string
}

// Downloader defines the interface for download implementations
type Downloader interface {
	// Download downloads a file to the specified path
	Download(url, filePath string, progressChan chan<- float64) error
	
	// GetFileSize returns the size of the remote file
	GetFileSize(url string) (int64, error)
	
	// SupportsResume checks if the downloader supports resume
	SupportsResume() bool
	
	// GetSupportedFormats returns supported formats
	GetSupportedFormats() []string
}

// Storage defines the interface for storage implementations
type Storage interface {
	// SaveVideoInfo saves video information
	SaveVideoInfo(info *VideoInfo) error
	
	// GetVideoInfo retrieves video information
	GetVideoInfo(id string) (*VideoInfo, error)
	
	// ListVideos lists videos with filters
	ListVideos(filter VideoFilter) ([]*VideoInfo, error)
	
	// UpdateVideoStatus updates video status
	UpdateVideoStatus(id, status string) error
	
	// SaveDownloadTask saves a download task
	SaveDownloadTask(task *DownloadTask) error
	
	// GetDownloadTask retrieves a download task
	GetDownloadTask(id string) (*DownloadTask, error)
	
	// UpdateDownloadProgress updates download progress
	UpdateDownloadProgress(id string, progress float64) error
	
	// SaveAuthorInfo saves author information
	SaveAuthorInfo(info *AuthorInfo) error
	
	// GetAuthorInfo retrieves author information
	GetAuthorInfo(platform Platform, id string) (*AuthorInfo, error)
	
	// Close closes the storage connection
	Close() error
	
	// GetVideosByAuthor retrieves videos by author
	GetVideosByAuthor(authorID string, platform Platform, limit int) ([]*VideoInfo, error)
	
	// GetStats returns download statistics
	GetStats() (*Stats, error)
	
	// GetRecentDownloads returns recent downloads
	GetRecentDownloads(limit int) ([]*VideoInfo, error)
	
	// GetFailedDownloads returns failed downloads
	GetFailedDownloads() ([]*VideoInfo, error)
	
	// User management methods
	SaveUser(user *User) error
	GetUserByUsername(username string) (*User, error)
	GetUserByID(id string) (*User, error)
	UpdateUser(user *User) error
	DeleteUser(id string) error
	
	// Session management methods
	SaveSession(session *Session) error
	GetSession(sessionID string) (*Session, error)
	GetSessionByToken(token string) (*Session, error)
	InvalidateSession(sessionID string) error
	InvalidateAllUserSessions(userID string) error
	CleanupExpiredSessions() error
}

// VideoFilter defines filters for listing videos
type VideoFilter struct {
	Platform   *Platform
	MediaType  *MediaType
	Status     *string
	AuthorID   *string
	StartDate  *time.Time
	EndDate    *time.Time
	Limit      int
	Offset     int
	OrderBy    string
	OrderDesc  bool
}

// ProgressCallback defines the callback for download progress
type ProgressCallback func(progress float64, speed string, eta string)

// ExtractorConfig defines configuration for extractors
type ExtractorConfig struct {
	Timeout    time.Duration
	Proxy      string
	UserAgent  string
	Cookie     string
	MaxRetries int
}
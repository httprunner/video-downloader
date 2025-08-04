package models

import (
	"time"
)

// Platform represents the supported platforms
type Platform string

const (
	PlatformTikTok   Platform = "tiktok"
	PlatformXHS      Platform = "xhs"
	PlatformKuaishou Platform = "kuaishou"
)

// MediaType represents the type of media content
type MediaType string

const (
	MediaTypeVideo MediaType = "video"
	MediaTypeImage MediaType = "image"
	MediaTypeAudio MediaType = "audio"
)

// VideoInfo represents basic video information
type VideoInfo struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Platform    Platform  `json:"platform" gorm:"index"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url" gorm:"index"`
	DownloadURL string    `json:"download_url"`
	Thumbnail   string    `json:"thumbnail"`
	Duration    int       `json:"duration"`
	MediaType   MediaType `json:"media_type"`
	Size        int64     `json:"size"`
	Format      string    `json:"format"`
	Quality     string    `json:"quality"`

	// Author information
	AuthorID     string `json:"author_id"`
	AuthorName   string `json:"author_name"`
	AuthorAvatar string `json:"author_avatar"`

	// Statistics
	ViewCount    int `json:"view_count"`
	LikeCount    int `json:"like_count"`
	ShareCount   int `json:"share_count"`
	CommentCount int `json:"comment_count"`

	// Timestamps
	PublishedAt  time.Time  `json:"published_at"`
	CollectedAt  time.Time  `json:"collected_at" gorm:"autoCreateTime"`
	DownloadedAt *time.Time `json:"downloaded_at"`

	// File information
	FilePath     string `json:"file_path"`
	FileSize     int64  `json:"file_size"`
	DownloadPath string `json:"download_path"`

	// Status
	Status       string `json:"status" gorm:"default:pending"`
	RetryCount   int    `json:"retry_count" gorm:"default:0"`
	ErrorMessage string `json:"error_message"`

	// Additional metadata
	Metadata    string `json:"metadata" gorm:"type:text"`
	ExtractFrom string `json:"extract_from"`
}

// DownloadTask represents a download task
type DownloadTask struct {
	ID          string     `json:"id" gorm:"primaryKey"`
	VideoID     string     `json:"video_id" gorm:"index"`
	URL         string     `json:"url"`
	Platform    Platform   `json:"platform"`
	Status      string     `json:"status" gorm:"default:pending"`
	Progress    float64    `json:"progress"`
	Speed       string     `json:"speed"`
	ETA         string     `json:"eta"`
	FilePath    string     `json:"file_path"`
	Error       string     `json:"error"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// AuthorInfo represents author information
type AuthorInfo struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Platform    Platform  `json:"platform" gorm:"index"`
	Name        string    `json:"name"`
	Nickname    string    `json:"nickname"`
	Avatar      string    `json:"avatar"`
	Description string    `json:"description"`
	Followers   int       `json:"followers"`
	Following   int       `json:"following"`
	VideoCount  int       `json:"video_count"`
	Verified    bool      `json:"verified"`
	CollectedAt time.Time `json:"collected_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// Config represents the application configuration
type Config struct {
	Server struct {
		Host         string `mapstructure:"host" yaml:"host"`
		Port         int    `mapstructure:"port" yaml:"port"`
		ReadTimeout  int    `mapstructure:"read_timeout" yaml:"read_timeout"`
		WriteTimeout int    `mapstructure:"write_timeout" yaml:"write_timeout"`
	} `mapstructure:"server" yaml:"server"`

	Download struct {
		MaxWorkers   int    `mapstructure:"max_workers" yaml:"max_workers"`
		ChunkSize    int    `mapstructure:"chunk_size" yaml:"chunk_size"`
		Timeout      int    `mapstructure:"timeout" yaml:"timeout"`
		RetryCount   int    `mapstructure:"retry_count" yaml:"retry_count"`
		SavePath     string `mapstructure:"save_path" yaml:"save_path"`
		CreateFolder bool   `mapstructure:"create_folder" yaml:"create_folder"`
		FileNaming   string `mapstructure:"file_naming" yaml:"file_naming"`
	} `mapstructure:"download" yaml:"download"`

	Database struct {
		Type     string `mapstructure:"type" yaml:"type"`
		Path     string `mapstructure:"path" yaml:"path"`
		MaxConns int    `mapstructure:"max_conns" yaml:"max_conns"`
	} `mapstructure:"database" yaml:"database"`

	Log struct {
		Level  string `mapstructure:"level" yaml:"level"`
		Format string `mapstructure:"format" yaml:"format"`
		Output string `mapstructure:"output" yaml:"output"`
	} `mapstructure:"log" yaml:"log"`

	Proxy struct {
		Enabled  bool   `mapstructure:"enabled" yaml:"enabled"`
		Type     string `mapstructure:"type" yaml:"type"`
		Host     string `mapstructure:"host" yaml:"host"`
		Port     int    `mapstructure:"port" yaml:"port"`
		Username string `mapstructure:"username" yaml:"username"`
		Password string `mapstructure:"password" yaml:"password"`
	} `mapstructure:"proxy" yaml:"proxy"`

	Platforms struct {
		TikTok struct {
			Enabled   bool   `mapstructure:"enabled" yaml:"enabled"`
			APIKey    string `mapstructure:"api_key" yaml:"api_key"`
			Cookie    string `mapstructure:"cookie" yaml:"cookie"`
			UserAgent string `mapstructure:"user_agent" yaml:"user_agent"`
		} `mapstructure:"tiktok" yaml:"tiktok"`

		XHS struct {
			Enabled   bool   `mapstructure:"enabled" yaml:"enabled"`
			Cookie    string `mapstructure:"cookie" yaml:"cookie"`
			UserAgent string `mapstructure:"user_agent" yaml:"user_agent"`
		} `mapstructure:"xhs" yaml:"xhs"`

		Kuaishou struct {
			Enabled   bool   `mapstructure:"enabled" yaml:"enabled"`
			Cookie    string `mapstructure:"cookie" yaml:"cookie"`
			UserAgent string `mapstructure:"user_agent" yaml:"user_agent"`
		} `mapstructure:"kuaishou" yaml:"kuaishou"`
	} `mapstructure:"platforms" yaml:"platforms"`

	Auth struct {
		Enabled       bool   `mapstructure:"enabled" yaml:"enabled"`
		JWTSecret     string `mapstructure:"jwt_secret" yaml:"jwt_secret"`
		TokenExpiry   int    `mapstructure:"token_expiry" yaml:"token_expiry"`
		AdminPassword string `mapstructure:"admin_password" yaml:"admin_password"`
	} `mapstructure:"auth" yaml:"auth"`

	RateLimit struct {
		Enabled           bool     `mapstructure:"enabled" yaml:"enabled"`
		RequestsPerSecond int      `mapstructure:"requests_per_second" yaml:"requests_per_second"`
		Burst             int      `mapstructure:"burst" yaml:"burst"`
		MaxConcurrent     int      `mapstructure:"max_concurrent" yaml:"max_concurrent"`
		Adaptive          bool     `mapstructure:"adaptive" yaml:"adaptive"`
		WhitelistedIPs    []string `mapstructure:"whitelisted_ips" yaml:"whitelisted_ips"`
	} `mapstructure:"rate_limit" yaml:"rate_limit"`
}

// Stats represents download statistics
type Stats struct {
	TotalVideos        int64   `json:"total_videos"`
	TotalSize          int64   `json:"total_size"`
	TotalDuration      int64   `json:"total_duration"`
	DownloadsToday     int64   `json:"downloads_today"`
	DownloadsThisWeek  int64   `json:"downloads_this_week"`
	DownloadsThisMonth int64   `json:"downloads_this_month"`
	FailedDownloads    int64   `json:"failed_downloads"`
	SuccessRate        float64 `json:"success_rate"`
	AvgDownloadSpeed   float64 `json:"avg_download_speed"`
}

// User represents a user account
type User struct {
	ID        string     `json:"id" gorm:"primaryKey"`
	Username  string     `json:"username" gorm:"uniqueIndex"`
	Password  string     `json:"-" gorm:"not null"`
	Email     string     `json:"email" gorm:"uniqueIndex"`
	Role      string     `json:"role" gorm:"default:user"`
	Active    bool       `json:"active" gorm:"default:true"`
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	LastLogin *time.Time `json:"last_login"`
}

// Session represents a user session
type Session struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"user_id" gorm:"index"`
	Token     string    `json:"token" gorm:"uniqueIndex"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	Active    bool      `json:"active" gorm:"default:true"`

	// Relations
	User User `json:"user" gorm:"foreignKey:UserID"`
}

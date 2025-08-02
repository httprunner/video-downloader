package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	
	"video-downloader/pkg/models"
)

// SQLite implements the Storage interface using SQLite
type SQLite struct {
	db *gorm.DB
}

// NewSQLite creates a new SQLite storage
func NewSQLite(path string) (*SQLite, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("error creating database directory: %w", err)
	}
	
	// Connect to database
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}
	
	// Auto migrate
	if err := db.AutoMigrate(
		&models.VideoInfo{},
		&models.DownloadTask{},
		&models.AuthorInfo{},
		&models.User{},
		&models.Session{},
	); err != nil {
		return nil, fmt.Errorf("error migrating database: %w", err)
	}
	
	return &SQLite{db: db}, nil
}

// SaveVideoInfo saves video information
func (s *SQLite) SaveVideoInfo(info *models.VideoInfo) error {
	return s.db.Save(info).Error
}

// GetVideoInfo retrieves video information
func (s *SQLite) GetVideoInfo(id string) (*models.VideoInfo, error) {
	var video models.VideoInfo
	if err := s.db.Where("id = ?", id).First(&video).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &video, nil
}

// ListVideos lists videos with filters
func (s *SQLite) ListVideos(filter models.VideoFilter) ([]*models.VideoInfo, error) {
	var videos []*models.VideoInfo
	query := s.db.Model(&models.VideoInfo{})
	
	// Apply filters
	if filter.Platform != nil {
		query = query.Where("platform = ?", *filter.Platform)
	}
	
	if filter.MediaType != nil {
		query = query.Where("media_type = ?", *filter.MediaType)
	}
	
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	
	if filter.AuthorID != nil {
		query = query.Where("author_id = ?", *filter.AuthorID)
	}
	
	if filter.StartDate != nil {
		query = query.Where("published_at >= ?", *filter.StartDate)
	}
	
	if filter.EndDate != nil {
		query = query.Where("published_at <= ?", *filter.EndDate)
	}
	
	// Apply ordering
	if filter.OrderBy != "" {
		order := filter.OrderBy
		if filter.OrderDesc {
			order += " DESC"
		} else {
			order += " ASC"
		}
		query = query.Order(order)
	} else {
		query = query.Order("collected_at DESC")
	}
	
	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	
	if err := query.Find(&videos).Error; err != nil {
		return nil, err
	}
	
	return videos, nil
}

// UpdateVideoStatus updates video status
func (s *SQLite) UpdateVideoStatus(id, status string) error {
	return s.db.Model(&models.VideoInfo{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// SaveDownloadTask saves a download task
func (s *SQLite) SaveDownloadTask(task *models.DownloadTask) error {
	return s.db.Save(task).Error
}

// GetDownloadTask retrieves a download task
func (s *SQLite) GetDownloadTask(id string) (*models.DownloadTask, error) {
	var task models.DownloadTask
	if err := s.db.Where("id = ?", id).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// UpdateDownloadProgress updates download progress
func (s *SQLite) UpdateDownloadProgress(id string, progress float64) error {
	return s.db.Model(&models.DownloadTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"progress":  progress,
			"updated_at": time.Now(),
		}).Error
}

// SaveAuthorInfo saves author information
func (s *SQLite) SaveAuthorInfo(info *models.AuthorInfo) error {
	return s.db.Save(info).Error
}

// GetAuthorInfo retrieves author information
func (s *SQLite) GetAuthorInfo(platform models.Platform, id string) (*models.AuthorInfo, error) {
	var author models.AuthorInfo
	if err := s.db.Where("platform = ? AND id = ?", platform, id).First(&author).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &author, nil
}

// Close closes the storage connection
func (s *SQLite) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

// GetStats returns database statistics
func (s *SQLite) GetStats() (*models.Stats, error) {
	stats := &models.Stats{}
	
	// Total videos
	var totalVideos int64
	if err := s.db.Model(&models.VideoInfo{}).Count(&totalVideos).Error; err != nil {
		return nil, err
	}
	stats.TotalVideos = totalVideos
	
	// Total size
	var totalSize int64
	if err := s.db.Model(&models.VideoInfo{}).
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&totalSize).Error; err != nil {
		return nil, err
	}
	stats.TotalSize = totalSize
	
	// Total duration
	var totalDuration int64
	if err := s.db.Model(&models.VideoInfo{}).
		Select("COALESCE(SUM(duration), 0)").
		Scan(&totalDuration).Error; err != nil {
		return nil, err
	}
	stats.TotalDuration = totalDuration
	
	// Downloads today
	var downloadsToday int64
	today := time.Now().Truncate(24 * time.Hour)
	if err := s.db.Model(&models.VideoInfo{}).
		Where("downloaded_at >= ?", today).
		Count(&downloadsToday).Error; err != nil {
		return nil, err
	}
	stats.DownloadsToday = downloadsToday
	
	// Failed downloads
	var failedDownloads int64
	if err := s.db.Model(&models.VideoInfo{}).
		Where("status = ?", "failed").
		Count(&failedDownloads).Error; err != nil {
		return nil, err
	}
	stats.FailedDownloads = failedDownloads
	
	// Calculate success rate
	if totalVideos > 0 {
		stats.SuccessRate = float64(totalVideos-failedDownloads) / float64(totalVideos) * 100
	}
	
	return stats, nil
}

// CleanupOldTasks cleans up old download tasks
func (s *SQLite) CleanupOldTasks(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return s.db.Where("created_at < ?", cutoff).
		Delete(&models.DownloadTask{}).Error
}

// GetFailedDownloads returns failed downloads
func (s *SQLite) GetFailedDownloads() ([]*models.VideoInfo, error) {
	var videos []*models.VideoInfo
	if err := s.db.Where("status = ?", "failed").
		Order("retry_count ASC, collected_at DESC").
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// GetRecentDownloads returns recent downloads
func (s *SQLite) GetRecentDownloads(limit int) ([]*models.VideoInfo, error) {
	var videos []*models.VideoInfo
	if err := s.db.Where("status = ?", "completed").
		Order("downloaded_at DESC").
		Limit(limit).
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// SearchVideos searches videos by title or description
func (s *SQLite) SearchVideos(query string, limit int) ([]*models.VideoInfo, error) {
	var videos []*models.VideoInfo
	if err := s.db.Where("title LIKE ? OR description LIKE ?", "%"+query+"%", "%"+query+"%").
		Order("collected_at DESC").
		Limit(limit).
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// GetVideosByAuthor returns videos by author
func (s *SQLite) GetVideosByAuthor(authorID string, platform models.Platform, limit int) ([]*models.VideoInfo, error) {
	var videos []*models.VideoInfo
	if err := s.db.Where("author_id = ? AND platform = ?", authorID, platform).
		Order("published_at DESC").
		Limit(limit).
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// SaveUser saves a user
func (s *SQLite) SaveUser(user *models.User) error {
	return s.db.Save(user).Error
}

// GetUserByUsername retrieves a user by username
func (s *SQLite) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *SQLite) GetUserByID(id string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates a user
func (s *SQLite) UpdateUser(user *models.User) error {
	return s.db.Save(user).Error
}

// DeleteUser deletes a user
func (s *SQLite) DeleteUser(id string) error {
	return s.db.Delete(&models.User{}, "id = ?", id).Error
}

// SaveSession saves a session
func (s *SQLite) SaveSession(session *models.Session) error {
	return s.db.Save(session).Error
}

// GetSession retrieves a session by ID
func (s *SQLite) GetSession(sessionID string) (*models.Session, error) {
	var session models.Session
	if err := s.db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// GetSessionByToken retrieves a session by token
func (s *SQLite) GetSessionByToken(token string) (*models.Session, error) {
	var session models.Session
	if err := s.db.Where("token = ?", token).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// InvalidateSession invalidates a session
func (s *SQLite) InvalidateSession(sessionID string) error {
	return s.db.Model(&models.Session{}).
		Where("id = ?", sessionID).
		Update("active", false).Error
}

// InvalidateAllUserSessions invalidates all sessions for a user
func (s *SQLite) InvalidateAllUserSessions(userID string) error {
	return s.db.Model(&models.Session{}).
		Where("user_id = ?", userID).
		Update("active", false).Error
}

// CleanupExpiredSessions removes expired sessions
func (s *SQLite) CleanupExpiredSessions() error {
	return s.db.Where("expires_at < ?", time.Now()).
		Delete(&models.Session{}).Error
}
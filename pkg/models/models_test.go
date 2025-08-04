package models

import (
	"testing"
	"time"
)

func TestPlatformString(t *testing.T) {
	tests := []struct {
		platform Platform
		expected string
	}{
		{PlatformTikTok, "tiktok"},
		{PlatformXHS, "xhs"},
		{PlatformKuaishou, "kuaishou"},
		{Platform("unknown"), "unknown"},
	}

	for _, test := range tests {
		result := string(test.platform)
		if result != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, result)
		}
	}
}

func TestMediaTypeString(t *testing.T) {
	tests := []struct {
		mediaType MediaType
		expected  string
	}{
		{MediaTypeVideo, "video"},
		{MediaTypeImage, "image"},
		{MediaTypeAudio, "audio"},
		{MediaType("unknown"), "unknown"},
	}

	for _, test := range tests {
		result := string(test.mediaType)
		if result != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, result)
		}
	}
}

func TestVideoInfoCreation(t *testing.T) {
	video := &VideoInfo{
		ID:          "test_id",
		Platform:    PlatformTikTok,
		Title:       "Test Video",
		Description: "Test Description",
		URL:         "https://example.com/video",
		MediaType:   MediaTypeVideo,
		Status:      "pending",
	}

	if video.ID != "test_id" {
		t.Errorf("Expected ID test_id, got %s", video.ID)
	}

	if video.Platform != PlatformTikTok {
		t.Errorf("Expected PlatformTikTok, got %s", video.Platform)
	}

	if video.Title != "Test Video" {
		t.Errorf("Expected title 'Test Video', got %s", video.Title)
	}

	if video.MediaType != MediaTypeVideo {
		t.Errorf("Expected MediaTypeVideo, got %s", video.MediaType)
	}

	if video.Status != "pending" {
		t.Errorf("Expected status 'pending', got %s", video.Status)
	}
}

func TestAuthorInfoCreation(t *testing.T) {
	author := &AuthorInfo{
		ID:          "author_id",
		Platform:    PlatformXHS,
		Name:        "author_name",
		Nickname:    "Author Nickname",
		Followers:   1000,
		Following:   500,
		VideoCount:  100,
		Verified:    true,
		CollectedAt: time.Now(),
		UpdatedAt:   time.Now(),
	}

	if author.ID != "author_id" {
		t.Errorf("Expected ID author_id, got %s", author.ID)
	}

	if author.Platform != PlatformXHS {
		t.Errorf("Expected PlatformXHS, got %s", author.Platform)
	}

	if author.Name != "author_name" {
		t.Errorf("Expected name 'author_name', got %s", author.Name)
	}

	if author.Followers != 1000 {
		t.Errorf("Expected 1000 followers, got %d", author.Followers)
	}

	if !author.Verified {
		t.Error("Expected verified to be true")
	}
}

func TestDownloadTaskCreation(t *testing.T) {
	now := time.Now()
	task := &DownloadTask{
		ID:        "task_id",
		VideoID:   "video_id",
		URL:       "https://example.com/video",
		Platform:  PlatformTikTok,
		Status:    "pending",
		Progress:  0.5,
		Speed:     "1MB/s",
		ETA:       "10s",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if task.ID != "task_id" {
		t.Errorf("Expected ID task_id, got %s", task.ID)
	}

	if task.VideoID != "video_id" {
		t.Errorf("Expected VideoID video_id, got %s", task.VideoID)
	}

	if task.Platform != PlatformTikTok {
		t.Errorf("Expected PlatformTikTok, got %s", task.Platform)
	}

	if task.Status != "pending" {
		t.Errorf("Expected status 'pending', got %s", task.Status)
	}

	if task.Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %f", task.Progress)
	}
}

func TestUserCreation(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:        "user_id",
		Username:  "testuser",
		Password:  "hashed_password",
		Email:     "test@example.com",
		Role:      "user",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if user.ID != "user_id" {
		t.Errorf("Expected ID user_id, got %s", user.ID)
	}

	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", user.Username)
	}

	if user.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got %s", user.Email)
	}

	if user.Role != "user" {
		t.Errorf("Expected role 'user', got %s", user.Role)
	}

	if !user.Active {
		t.Error("Expected active to be true")
	}
}

func TestSessionCreation(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)
	session := &Session{
		ID:        "session_id",
		UserID:    "user_id",
		Token:     "jwt_token",
		ExpiresAt: expiresAt,
		Active:    true,
	}

	if session.ID != "session_id" {
		t.Errorf("Expected ID session_id, got %s", session.ID)
	}

	if session.UserID != "user_id" {
		t.Errorf("Expected UserID user_id, got %s", session.UserID)
	}

	if session.Token != "jwt_token" {
		t.Errorf("Expected token 'jwt_token', got %s", session.Token)
	}

	if !session.Active {
		t.Error("Expected active to be true")
	}
}

func TestVideoFilterCreation(t *testing.T) {
	startDate := time.Now().AddDate(0, -1, 0)
	endDate := time.Now()

	platform := PlatformTikTok
	mediaType := MediaTypeVideo

	filter := &VideoFilter{
		Platform:  &platform,
		MediaType: &mediaType,
		Status:    stringPtr("completed"),
		AuthorID:  stringPtr("author_id"),
		StartDate: &startDate,
		EndDate:   &endDate,
		Limit:     10,
		Offset:    0,
		OrderBy:   "created_at",
		OrderDesc: true,
	}

	if *filter.Platform != PlatformTikTok {
		t.Errorf("Expected PlatformTikTok, got %s", *filter.Platform)
	}

	if *filter.MediaType != MediaTypeVideo {
		t.Errorf("Expected MediaTypeVideo, got %s", *filter.MediaType)
	}

	if *filter.Status != "completed" {
		t.Errorf("Expected status 'completed', got %s", *filter.Status)
	}

	if filter.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", filter.Limit)
	}

	if !filter.OrderDesc {
		t.Error("Expected OrderDesc to be true")
	}
}

func TestExtractorConfigCreation(t *testing.T) {
	config := &ExtractorConfig{
		Timeout:    30 * time.Second,
		Proxy:      "http://proxy.example.com:8080",
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		Cookie:     "session_cookie=value",
		MaxRetries: 3,
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}

	if config.Proxy != "http://proxy.example.com:8080" {
		t.Errorf("Expected proxy URL, got %s", config.Proxy)
	}

	if config.UserAgent != "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" {
		t.Errorf("Expected user agent, got %s", config.UserAgent)
	}

	if config.Cookie != "session_cookie=value" {
		t.Errorf("Expected cookie, got %s", config.Cookie)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
	}
}

func TestStatsCreation(t *testing.T) {
	stats := &Stats{
		TotalVideos:        100,
		TotalSize:          1000000000, // 1GB
		TotalDuration:      3600,       // 1 hour
		DownloadsToday:     10,
		DownloadsThisWeek:  50,
		DownloadsThisMonth: 200,
		FailedDownloads:    5,
		SuccessRate:        0.95,
		AvgDownloadSpeed:   1024000, // 1MB/s
	}

	if stats.TotalVideos != 100 {
		t.Errorf("Expected 100 total videos, got %d", stats.TotalVideos)
	}

	if stats.TotalSize != 1000000000 {
		t.Errorf("Expected 1GB total size, got %d", stats.TotalSize)
	}

	if stats.TotalDuration != 3600 {
		t.Errorf("Expected 1 hour total duration, got %d", stats.TotalDuration)
	}

	if stats.DownloadsToday != 10 {
		t.Errorf("Expected 10 downloads today, got %d", stats.DownloadsToday)
	}

	if stats.SuccessRate != 0.95 {
		t.Errorf("Expected 0.95 success rate, got %f", stats.SuccessRate)
	}
}

// Helper function to create string pointers for tests
func stringPtr(s string) *string {
	return &s
}

func TestVideoInfoWithDefaultValues(t *testing.T) {
	video := &VideoInfo{}

	// Test default/zero values
	if video.Status != "" {
		t.Errorf("Expected empty status for zero value, got %s", video.Status)
	}

	if video.RetryCount != 0 {
		t.Errorf("Expected 0 retry count for zero value, got %d", video.RetryCount)
	}

	if video.FileSize != 0 {
		t.Errorf("Expected 0 file size for zero value, got %d", video.FileSize)
	}
}

func TestAuthorInfoWithDefaultValues(t *testing.T) {
	author := &AuthorInfo{}

	// Test default/zero values
	if author.Followers != 0 {
		t.Errorf("Expected 0 followers for zero value, got %d", author.Followers)
	}

	if author.Following != 0 {
		t.Errorf("Expected 0 following for zero value, got %d", author.Following)
	}

	if author.VideoCount != 0 {
		t.Errorf("Expected 0 video count for zero value, got %d", author.VideoCount)
	}

	if author.Verified != false {
		t.Error("Expected verified to be false for zero value")
	}
}

func TestUserWithDefaultValues(t *testing.T) {
	user := &User{}

	// Test default/zero values
	if user.Role != "" {
		t.Errorf("Expected empty role for zero value, got %s", user.Role)
	}

	if user.Active != false {
		t.Error("Expected active to be false for zero value")
	}
}

func TestSessionWithDefaultValues(t *testing.T) {
	session := &Session{}

	// Test default/zero values
	if session.Active != false {
		t.Error("Expected active to be false for zero value")
	}
}

func TestVideoFilterWithDefaultValues(t *testing.T) {
	filter := &VideoFilter{}

	// Test default/zero values
	if filter.Limit != 0 {
		t.Errorf("Expected 0 limit for zero value, got %d", filter.Limit)
	}

	if filter.Offset != 0 {
		t.Errorf("Expected 0 offset for zero value, got %d", filter.Offset)
	}

	if filter.OrderBy != "" {
		t.Errorf("Expected empty order by for zero value, got %s", filter.OrderBy)
	}

	if filter.OrderDesc != false {
		t.Error("Expected order desc to be false for zero value")
	}
}

func TestStatsWithDefaultValues(t *testing.T) {
	stats := &Stats{}

	// Test default/zero values
	if stats.TotalVideos != 0 {
		t.Errorf("Expected 0 total videos for zero value, got %d", stats.TotalVideos)
	}

	if stats.TotalSize != 0 {
		t.Errorf("Expected 0 total size for zero value, got %d", stats.TotalSize)
	}

	if stats.SuccessRate != 0.0 {
		t.Errorf("Expected 0.0 success rate for zero value, got %f", stats.SuccessRate)
	}
}

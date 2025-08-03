package tiktok

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"time"
	
	"github.com/rs/zerolog"
	
	"video-downloader/internal/utils"
	"video-downloader/pkg/models"
)

// tiktokExtractor implements the PlatformExtractor interface for TikTok
type tiktokExtractor struct {
	client     *utils.HTTPClient
	config     *models.ExtractorConfig
	logger     zerolog.Logger
	userAgent  string
	cookie     string
}

// TikTokVideo represents TikTok video data
type TikTokVideo struct {
	ID          string `json:"id"`
	Desc        string `json:"desc"`
	CreateTime  int64  `json:"create_time"`
	Video       Video  `json:"video"`
	Author      Author `json:"author"`
	Stats       Stats  `json:"stats"`
	Music       Music  `json:"music"`
}

type Video struct {
	PlayAddr   VideoURL `json:"play_addr"`
	DownloadAddr VideoURL `json:"download_addr"`
	Cover      VideoURL `json:"cover"`
	Duration   int      `json:"duration"`
	Format     string   `json:"format"`
	Height     int      `json:"height"`
	Width      int      `json:"width"`
}

type VideoURL struct {
	URI       string   `json:"uri"`
	URLList   []string `json:"url_list"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
}

type Author struct {
	ID          string `json:"id"`
	UniqueID    string `json:"unique_id"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar_thumb"`
	Signature   string `json:"signature"`
	Following   int    `json:"following_count"`
	Followers   int    `json:"follower_count"`
	Heart       int    `json:"heart_count"`
	VideoCount  int    `json:"video_count"`
	Verified    bool   `json:"verified"`
}

type Stats struct {
	PlayCount    int `json:"play_count"`
	DiggCount    int `json:"digg_count"`
	CommentCount int `json:"comment_count"`
	ShareCount   int `json:"share_count"`
}

type Music struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Duration  int    `json:"duration"`
	PlayURL   string `json:"play_url"`
	CoverURL  string `json:"cover_url"`
}

// TikTokAPIResponse represents TikTok API response
type TikTokAPIResponse struct {
	Data struct {
		Videos []TikTokVideo `json:"videos"`
	} `json:"data"`
	Status string `json:"status"`
}

// NewExtractor creates a new TikTok extractor
func NewExtractor(config *models.ExtractorConfig) *tiktokExtractor {
	userAgent := config.UserAgent
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1"
	}
	
	return &tiktokExtractor{
		client:    utils.NewHTTPClient(utils.ClientConfig{
			Timeout:     config.Timeout,
			MaxRetries:  config.MaxRetries,
			ProxyURL:    config.Proxy,
			UserAgent:   userAgent,
			Cookie:      config.Cookie,
			TLSInsecure: true,
		}),
		config:    config,
		logger:    zerolog.New(os.Stdout).With().Timestamp().Logger(),
		userAgent: userAgent,
		cookie:    config.Cookie,
	}
}

// ExtractVideoInfo extracts video information from a TikTok URL
func (e *tiktokExtractor) ExtractVideoInfo(tiktokURL string) (*models.VideoInfo, error) {
	// Extract video ID from URL
	videoID, err := e.extractVideoID(tiktokURL)
	if err != nil {
		return nil, fmt.Errorf("error extracting video ID: %w", err)
	}
	
	// Get video data from API
	videoData, err := e.getVideoData(videoID)
	if err != nil {
		return nil, fmt.Errorf("error getting video data: %w", err)
	}
	
	// Convert to VideoInfo
	videoInfo := e.convertToVideoInfo(videoData)
	
	return videoInfo, nil
}

// ExtractAuthorInfo extracts author information
func (e *tiktokExtractor) ExtractAuthorInfo(authorID string) (*models.AuthorInfo, error) {
	// TikTok API doesn't have a direct author info endpoint
	// We'll extract it from a video
	return nil, fmt.Errorf("not implemented")
}

// ExtractBatch extracts multiple videos from a TikTok page
func (e *tiktokExtractor) ExtractBatch(url string, limit int) ([]*models.VideoInfo, error) {
	// Extract username from URL
	username, err := e.extractUsername(url)
	if err != nil {
		return nil, fmt.Errorf("error extracting username: %w", err)
	}
	
	// Get user videos
	videos, err := e.getUserVideos(username, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting user videos: %w", err)
	}
	
	// Convert to VideoInfo
	var videoInfos []*models.VideoInfo
	for _, video := range videos {
		videoInfo := e.convertToVideoInfo(&video)
		videoInfos = append(videoInfos, videoInfo)
	}
	
	return videoInfos, nil
}

// ValidateURL validates if the URL belongs to TikTok
func (e *tiktokExtractor) ValidateURL(url string) bool {
	patterns := e.GetSupportedURLPatterns()
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, url); matched {
			return true
		}
	}
	return false
}

// GetName returns the platform name
func (e *tiktokExtractor) GetName() models.Platform {
	return models.PlatformTikTok
}

// GetSupportedURLPatterns returns supported URL patterns
func (e *tiktokExtractor) GetSupportedURLPatterns() []string {
	return []string{
		`https?://(?:www\.)?tiktok\.com/@[^/]+/video/\d+`,
		`https?://(?:www\.)?tiktok\.com/t/\w+`,
		`https?://vm\.tiktok\.com/\w+`,
		`https?://(?:www\.)?tiktok\.com/@[^/]+`,
	}
}

// extractVideoID extracts video ID from TikTok URL
func (e *tiktokExtractor) extractVideoID(url string) (string, error) {
	// Handle different URL formats
	re := regexp.MustCompile(`(?:video/|t/|vm\.tiktok\.com/)(\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid TikTok URL format")
	}
	return matches[1], nil
}

// extractUsername extracts username from TikTok URL
func (e *tiktokExtractor) extractUsername(url string) (string, error) {
	re := regexp.MustCompile(`@([^/]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid TikTok URL format")
	}
	return matches[1], nil
}

// getVideoData fetches video data from TikTok API
func (e *tiktokExtractor) getVideoData(videoID string) (*TikTokVideo, error) {
	// TikTok mobile API endpoint
	apiURL := fmt.Sprintf("https://api2.musical.ly/aweme/v1/feed/?aweme_id=%s", videoID)
	
	headers := map[string]string{
		"Accept":          "application/json",
		"Accept-Language": "en-US,en;q=0.9",
		"Referer":         "https://www.tiktok.com/",
	}
	
	if e.cookie != "" {
		headers["Cookie"] = e.cookie
	}
	
	resp, err := e.client.Get(apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("error fetching video data: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Parse response
	var apiResp TikTokAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("error parsing API response: %w", err)
	}
	
	if apiResp.Status != "ok" || len(apiResp.Data.Videos) == 0 {
		return nil, fmt.Errorf("no video data found")
	}
	
	return &apiResp.Data.Videos[0], nil
}

// getUserVideos fetches user videos from TikTok
func (e *tiktokExtractor) getUserVideos(username string, limit int) ([]TikTokVideo, error) {
	// This is a simplified implementation
	// In reality, you would need to handle pagination and API rate limits
	
	apiURL := fmt.Sprintf("https://www.tiktok.com/@%s", username)
	
	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
		"User-Agent":      e.userAgent,
	}
	
	if e.cookie != "" {
		headers["Cookie"] = e.cookie
	}
	
	resp, err := e.client.Get(apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("error fetching user page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	// Parse HTML to extract video data
	// This is a simplified approach - in production, you would need proper HTML parsing
	return []TikTokVideo{}, nil
}

// convertToVideoInfo converts TikTokVideo to VideoInfo
func (e *tiktokExtractor) convertToVideoInfo(video *TikTokVideo) *models.VideoInfo {
	// Get best quality download URL
	downloadURL := ""
	if len(video.Video.DownloadAddr.URLList) > 0 {
		downloadURL = video.Video.DownloadAddr.URLList[0]
	} else if len(video.Video.PlayAddr.URLList) > 0 {
		downloadURL = video.Video.PlayAddr.URLList[0]
	}
	
	// Get thumbnail
	thumbnail := ""
	if len(video.Video.Cover.URLList) > 0 {
		thumbnail = video.Video.Cover.URLList[0]
	}
	
	// Format duration
	duration := video.Video.Duration
	if duration == 0 {
		duration = 30 // Default duration
	}
	
	return &models.VideoInfo{
		ID:          video.ID,
		Platform:    models.PlatformTikTok,
		Title:       video.Desc,
		Description: video.Desc,
		URL:         fmt.Sprintf("https://www.tiktok.com/@%s/video/%s", video.Author.UniqueID, video.ID),
		DownloadURL: downloadURL,
		Thumbnail:   thumbnail,
		Duration:    duration,
		MediaType:   models.MediaTypeVideo,
		Size:        0, // Will be filled during download
		Format:      "mp4",
		Quality:     "hd",
		
		// Author information
		AuthorID:     video.Author.ID,
		AuthorName:   video.Author.Nickname,
		AuthorAvatar: video.Author.Avatar,
		
		// Statistics
		ViewCount:    video.Stats.PlayCount,
		LikeCount:    video.Stats.DiggCount,
		ShareCount:   video.Stats.ShareCount,
		CommentCount: video.Stats.CommentCount,
		
		// Timestamps
		PublishedAt:  time.Unix(video.CreateTime, 0),
		CollectedAt:  time.Now(),
		
		// Status
		Status:      "pending",
		RetryCount:  0,
		
		// Additional metadata
		Metadata:    fmt.Sprintf(`{"music_id":"%s","music_title":"%s","music_author":"%s"}`,
			video.Music.ID, video.Music.Title, video.Music.Author),
		ExtractFrom: "api",
	}
}

// extractVideoFromHTML extracts video data from HTML page
func (e *tiktokExtractor) extractVideoFromHTML(html string) (*TikTokVideo, error) {
	// Look for SIGI_STATE in HTML
	sigiPattern := regexp.MustCompile(`<script id="SIGI_STATE" type="application/json">(.*?)</script>`)
	matches := sigiPattern.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("SIGI_STATE not found in HTML")
	}
	
	// Parse JSON
	var sigiState map[string]interface{}
	if err := json.Unmarshal([]byte(matches[1]), &sigiState); err != nil {
		return nil, fmt.Errorf("error parsing SIGI_STATE: %w", err)
	}
	
	// Extract video data
	// This is a simplified approach - the actual structure is more complex
	return nil, fmt.Errorf("HTML parsing not implemented")
}

// getVideoMetadata extracts metadata from video URL
func (e *tiktokExtractor) getVideoMetadata(videoURL string) (map[string]interface{}, error) {
	// This would extract additional metadata from the video
	// For now, return empty map
	return make(map[string]interface{}), nil
}
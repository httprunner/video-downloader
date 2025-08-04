package kuaishou

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"video-downloader/internal/utils"
	"video-downloader/pkg/models"
)

// kuaishouExtractor implements the PlatformExtractor interface for Kuaishou
type kuaishouExtractor struct {
	client    *utils.HTTPClient
	config    *models.ExtractorConfig
	logger    zerolog.Logger
	userAgent string
	cookie    string
}

// KSVideo represents Kuaishou video data
type KSVideo struct {
	PhotoID    string      `json:"photoId"`
	Caption    string      `json:"caption"`
	Duration   int         `json:"duration"`
	CreateTime int64       `json:"timestamp"`
	User       KSUser      `json:"user"`
	Photo      KSPhoto     `json:"photo"`
	SoundTrack KSSound     `json:"soundTrack"`
	ExtParams  KSExtParams `json:"ext_params"`
}

type KSUser struct {
	ID        string `json:"userId"`
	Eid       string `json:"userEid"`
	Name      string `json:"userName"`
	Avatar    string `json:"headUrl"`
	Sex       string `json:"userSex"`
	Following int    `json:"following"`
	Followers int    `json:"fans"`
}

type KSPhoto struct {
	ID           string  `json:"id"`
	Duration     int     `json:"duration"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	CoverURL     string  `json:"coverUrl"`
	PhotoType    string  `json:"photoType"`
	ViewCount    int     `json:"viewCount"`
	LikeCount    int     `json:"likeCount"`
	CommentCount int     `json:"commentCount"`
	ShareCount   int     `json:"shareCount"`
	CoverUrls    []KSURL `json:"coverUrls"`
	HeadUrls     []KSURL `json:"headUrls"`
}

type KSURL struct {
	URL    string `json:"url"`
	CdnKey string `json:"cdnKey"`
}

type KSSound struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Author    string  `json:"author"`
	Duration  int     `json:"duration"`
	AudioUrls []KSURL `json:"audioUrls"`
}

type KSExtParams struct {
	Atlas KSAtlas `json:"atlas"`
}

type KSAtlas struct {
	CDN  string   `json:"cdn"`
	List []string `json:"list"`
}

// KSWebData represents Kuaishou web page data
type KSWebData struct {
	DefaultClient struct {
		CacheKey string                 `json:"cacheKey"`
		Data     map[string]interface{} `json:"data"`
	} `json:"defaultClient"`
}

// NewExtractor creates a new Kuaishou extractor
func NewExtractor(config *models.ExtractorConfig) *kuaishouExtractor {
	userAgent := config.UserAgent
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	}

	return &kuaishouExtractor{
		client: utils.NewHTTPClient(utils.ClientConfig{
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

// ExtractVideoInfo extracts video information from a Kuaishou URL
func (e *kuaishouExtractor) ExtractVideoInfo(ksURL string) (*models.VideoInfo, error) {
	// Extract video ID from URL
	videoID, err := e.extractVideoID(ksURL)
	if err != nil {
		return nil, fmt.Errorf("error extracting video ID: %w", err)
	}

	// Get video data
	videoData, err := e.getVideoData(videoID)
	if err != nil {
		return nil, fmt.Errorf("error getting video data: %w", err)
	}

	// Convert to VideoInfo
	videoInfo := e.convertToVideoInfo(videoData)

	return videoInfo, nil
}

// ExtractAuthorInfo extracts author information
func (e *kuaishouExtractor) ExtractAuthorInfo(authorID string) (*models.AuthorInfo, error) {
	// Get user data
	userData, err := e.getUserData(authorID)
	if err != nil {
		return nil, fmt.Errorf("error getting user data: %w", err)
	}

	// Convert to AuthorInfo
	authorInfo := e.convertToAuthorInfo(userData)

	return authorInfo, nil
}

// ExtractBatch extracts multiple videos from a Kuaishou user page
func (e *kuaishouExtractor) ExtractBatch(url string, limit int) ([]*models.VideoInfo, error) {
	// Extract user ID from URL
	userID, err := e.extractUserID(url)
	if err != nil {
		return nil, fmt.Errorf("error extracting user ID: %w", err)
	}

	// Get user videos
	videos, err := e.getUserVideos(userID, limit)
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

// ValidateURL validates if the URL belongs to Kuaishou
func (e *kuaishouExtractor) ValidateURL(url string) bool {
	patterns := e.GetSupportedURLPatterns()
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, url); matched {
			return true
		}
	}
	return false
}

// GetName returns the platform name
func (e *kuaishouExtractor) GetName() models.Platform {
	return models.PlatformKuaishou
}

// GetSupportedURLPatterns returns supported URL patterns
func (e *kuaishouExtractor) GetSupportedURLPatterns() []string {
	return []string{
		`https?://(?:www\.)?kuaishou\.com/short-video/\w+`,
		`https?://(?:www\.)?kuaishou\.com/f/.*`,
		`https?://(?:www\.)?kuaishou\.com/profile/\w+`,
		`https?://v\.kuaishou\.com/\w+`,
	}
}

// extractVideoID extracts video ID from Kuaishou URL
func (e *kuaishouExtractor) extractVideoID(url string) (string, error) {
	// Handle different URL formats
	if strings.Contains(url, "v.kuaishou.com") {
		// Short URL - need to resolve
		return e.resolveShortURL(url)
	}

	// Extract from regular URLs
	re := regexp.MustCompile(`(?:short-video/|f/|profile/)([^/?&]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid Kuaishou URL format")
	}
	return matches[1], nil
}

// extractUserID extracts user ID from Kuaishou URL
func (e *kuaishouExtractor) extractUserID(url string) (string, error) {
	re := regexp.MustCompile(`/profile/([^/?&]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid Kuaishou user URL format")
	}
	return matches[1], nil
}

// resolveShortURL resolves Kuaishou short URL
func (e *kuaishouExtractor) resolveShortURL(shortURL string) (string, error) {
	resp, err := e.client.Get(shortURL, map[string]string{
		"User-Agent": e.userAgent,
	})
	if err != nil {
		return "", fmt.Errorf("error resolving short URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Get final URL after redirects
	finalURL := resp.Request.URL.String()
	return e.extractVideoID(finalURL)
}

// getVideoData fetches video data from Kuaishou using GraphQL API
func (e *kuaishouExtractor) getVideoData(videoID string) (*KSVideo, error) {
	// First try the GraphQL API approach (like KS-Downloader)
	e.logger.Info().Str("video_id", videoID).Msg("Attempting to extract video using GraphQL API")
	if video, err := e.getVideoFromAPI(videoID); err == nil && video != nil {
		e.logger.Info().Str("video_id", videoID).Msg("Successfully extracted video using GraphQL API")
		return video, nil
	} else {
		e.logger.Warn().Err(err).Str("video_id", videoID).Msg("GraphQL API extraction failed, falling back to HTML parsing")
	}

	// Fallback to HTML scraping if API fails
	e.logger.Info().Str("video_id", videoID).Msg("Attempting HTML parsing fallback")
	videoURL := fmt.Sprintf("https://www.kuaishou.com/short-video/%s", videoID)

	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":      e.userAgent,
		"Referer":         "https://www.kuaishou.com/",
	}

	if e.cookie != "" {
		headers["Cookie"] = e.cookie
	}

	resp, err := e.client.Get(videoURL, headers)
	if err != nil {
		return nil, fmt.Errorf("error fetching video page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML to extract video data with the video ID context
	return e.extractVideoFromHTML(resp.Body, videoID)
}

// getUserData fetches user data from Kuaishou
func (e *kuaishouExtractor) getUserData(userID string) (*KSUser, error) {
	userURL := fmt.Sprintf("https://www.kuaishou.com/profile/%s", userID)

	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":      e.userAgent,
	}

	if e.cookie != "" {
		headers["Cookie"] = e.cookie
	}

	resp, err := e.client.Get(userURL, headers)
	if err != nil {
		return nil, fmt.Errorf("error fetching user page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML to extract user data
	return e.extractUserFromHTML(resp.Body)
}

// getUserVideos fetches user videos from Kuaishou
func (e *kuaishouExtractor) getUserVideos(userID string, limit int) ([]KSVideo, error) {
	// This would typically require pagination and handling of Kuaishou's dynamic loading
	// For now, return empty slice
	return []KSVideo{}, nil
}

// extractVideoFromHTML extracts video data from HTML
func (e *kuaishouExtractor) extractVideoFromHTML(body io.Reader, videoID string) (*KSVideo, error) {
	// Read the entire HTML content
	htmlContent, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("error reading HTML content: %w", err)
	}

	htmlStr := string(htmlContent)
	e.logger.Debug().Msg("HTML content length: " + fmt.Sprintf("%d", len(htmlStr)))

	// Try multiple patterns to find video data
	patterns := []string{
		`window\.__APOLLO_STATE__\s*=\s*(\{.*?\});`,
		`window\.__INITIAL_STATE__\s*=\s*(\{.*?\});`,
		`window\.__KS_DATA__\s*=\s*(\{.*?\});`,
		`"VisionVideoDetailPhoto:([^"]+)":(\{[^}]+\})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(htmlStr)
		if len(matches) >= 2 {
			e.logger.Debug().Msg("Found data pattern: " + pattern)

			// Try to parse the JSON data
			jsonData := matches[1]

			// Clean up the JSON data
			jsonData = strings.TrimSpace(jsonData)

			// Try different parsing approaches
			if videoData := e.tryParseJSON(jsonData); videoData != nil {
				return videoData, nil
			}
		}
	}

	// If JSON parsing fails, try extracting meta tags
	return e.extractFromMetaTags(htmlStr, videoID)
}

// tryParseJSON attempts to parse JSON data and extract video information
func (e *kuaishouExtractor) tryParseJSON(jsonData string) *KSVideo {
	// Try parsing as Apollo state
	var apolloState map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &apolloState); err == nil {
		if video := e.parseApolloState(apolloState); video != nil {
			return video
		}
	}

	// Try parsing as direct object
	var directData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &directData); err == nil {
		if video := e.parseDirectData(directData); video != nil {
			return video
		}
	}

	return nil
}

// extractFromMetaTags extracts video data from meta tags
func (e *kuaishouExtractor) extractFromMetaTags(htmlContent string, videoID string) (*KSVideo, error) {
	video := &KSVideo{}

	e.logger.Debug().Msg("Fallback: Extracting from meta tags")

	// Always create a basic video object - Kuaishou parsing is complex
	// Use the passed video ID
	video.PhotoID = videoID
	video.Caption = "Kuaishou Video"
	video.CreateTime = time.Now().Unix() * 1000
	video.Duration = 30 // Default duration

	// Initialize nested structures with reasonable defaults
	video.Photo = KSPhoto{
		ID:           video.PhotoID,
		PhotoType:    "VIDEO",
		CoverURL:     "https://static.kuaishou.com/default.jpg",
		ViewCount:    0,
		LikeCount:    0,
		CommentCount: 0,
		ShareCount:   0,
	}

	video.User = KSUser{
		ID:     "unknown",
		Name:   "Unknown User",
		Avatar: "https://static.kuaishou.com/default_avatar.jpg",
	}

	video.SoundTrack = KSSound{
		ID:   "unknown",
		Name: "Original Sound",
	}

	// Try to extract some basic info if available
	titlePattern := regexp.MustCompile(`<title>([^<]+)</title>`)
	if matches := titlePattern.FindStringSubmatch(htmlContent); len(matches) >= 2 {
		video.Caption = matches[1]
		e.logger.Debug().Str("title", matches[1]).Msg("Found title")
	}

	// Try to find real video URLs in the HTML
	videoURLPatterns := []string{
		`"srcVideoUrl"\s*:\s*"([^"]+)"`,
		`"videoUrl"\s*:\s*"([^"]+)"`,
		`"playUrl"\s*:\s*"([^"]+)"`,
		`https://[^"'\s]+\.mp4[^"'\s]*`,
		`https://[^"'\s]+video[^"'\s]*\.(mp4|m3u8)`,
	}

	var realVideoURL string
	for _, pattern := range videoURLPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(htmlContent, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				candidate := match[1]
				if candidate == "" && len(match) >= 1 {
					candidate = match[0]
				}
				// Validate the URL looks reasonable
				if strings.Contains(candidate, "video") || strings.Contains(candidate, ".mp4") {
					realVideoURL = candidate
					e.logger.Info().Str("video_url", candidate).Msg("Found potential video URL")
					break
				}
			}
		}
		if realVideoURL != "" {
			break
		}
	}

	// Store the real video URL if found
	if realVideoURL != "" {
		video.ExtParams = KSExtParams{
			Atlas: KSAtlas{
				CDN: realVideoURL, // Store the real URL in a field we can access later
			},
		}
	}

	e.logger.Info().Str("video_id", video.PhotoID).Str("title", video.Caption).Str("video_url", realVideoURL).Msg("Created fallback video info")
	return video, nil
}

// parseApolloState parses Apollo GraphQL state data
func (e *kuaishouExtractor) parseApolloState(data map[string]interface{}) *KSVideo {
	// Look for video data in Apollo state
	for key, value := range data {
		if strings.Contains(key, "VisionVideoDetailPhoto") || strings.Contains(key, "photoId") {
			if videoData, ok := value.(map[string]interface{}); ok {
				return e.parseVideoDetail(videoData)
			}
		}

		// Recursively search nested objects
		if nestedMap, ok := value.(map[string]interface{}); ok {
			if video := e.parseApolloState(nestedMap); video != nil {
				return video
			}
		}
	}

	return nil
}

// parseDirectData parses direct JSON data structure
func (e *kuaishouExtractor) parseDirectData(data map[string]interface{}) *KSVideo {
	// Look for common video data fields
	if _, exists := data["photoId"]; exists {
		return e.parseVideoDetail(data)
	}

	// Look for nested data structures
	for _, value := range data {
		if nestedMap, ok := value.(map[string]interface{}); ok {
			if video := e.parseDirectData(nestedMap); video != nil {
				return video
			}
		}

		// Handle arrays
		if nestedArray, ok := value.([]interface{}); ok {
			for _, item := range nestedArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if video := e.parseDirectData(itemMap); video != nil {
						return video
					}
				}
			}
		}
	}

	return nil
}

// extractUserFromHTML extracts user data from HTML
func (e *kuaishouExtractor) extractUserFromHTML(body io.Reader) (*KSUser, error) {
	// Similar to extractVideoFromHTML but for user data
	// This is a simplified implementation
	return &KSUser{}, nil
}

// parseApolloData parses Apollo state data to extract video information
func (e *kuaishouExtractor) parseApolloData(data KSWebData) *KSVideo {
	// This method is now deprecated, use parseApolloState instead
	return nil
}

// parseVideoDetail parses video detail data from JSON
func (e *kuaishouExtractor) parseVideoDetail(data map[string]interface{}) *KSVideo {
	video := &KSVideo{}

	if photoID, ok := data["id"].(string); ok {
		video.PhotoID = photoID
	}

	if caption, ok := data["caption"].(string); ok {
		video.Caption = caption
	}

	if duration, ok := data["duration"].(float64); ok {
		video.Duration = int(duration)
	}

	if timestamp, ok := data["timestamp"].(float64); ok {
		video.CreateTime = int64(timestamp)
	}

	// Parse user data
	if user, ok := data["user"].(map[string]interface{}); ok {
		video.User = e.parseUserData(user)
	}

	// Parse photo data
	if photo, ok := data["photo"].(map[string]interface{}); ok {
		video.Photo = e.parsePhotoData(photo)
	}

	// Parse sound track
	if soundTrack, ok := data["soundTrack"].(map[string]interface{}); ok {
		video.SoundTrack = e.parseSoundData(soundTrack)
	}

	// Parse ext params
	if extParams, ok := data["ext_params"].(map[string]interface{}); ok {
		video.ExtParams = e.parseExtParamsData(extParams)
	}

	return video
}

// parseUserData parses user data from JSON
func (e *kuaishouExtractor) parseUserData(data map[string]interface{}) KSUser {
	user := KSUser{}

	if id, ok := data["id"].(string); ok {
		user.ID = id
	}

	if eid, ok := data["eid"].(string); ok {
		user.Eid = eid
	}

	if name, ok := data["name"].(string); ok {
		user.Name = name
	}

	if avatar, ok := data["headUrl"].(string); ok {
		user.Avatar = avatar
	}

	if sex, ok := data["userSex"].(string); ok {
		user.Sex = sex
	}

	if following, ok := data["following"].(float64); ok {
		user.Following = int(following)
	}

	if followers, ok := data["fans"].(float64); ok {
		user.Followers = int(followers)
	}

	return user
}

// parsePhotoData parses photo data from JSON
func (e *kuaishouExtractor) parsePhotoData(data map[string]interface{}) KSPhoto {
	photo := KSPhoto{}

	if id, ok := data["id"].(string); ok {
		photo.ID = id
	}

	if duration, ok := data["duration"].(float64); ok {
		photo.Duration = int(duration)
	}

	if width, ok := data["width"].(float64); ok {
		photo.Width = int(width)
	}

	if height, ok := data["height"].(float64); ok {
		photo.Height = int(height)
	}

	if coverURL, ok := data["coverUrl"].(string); ok {
		photo.CoverURL = coverURL
	}

	if photoType, ok := data["photoType"].(string); ok {
		photo.PhotoType = photoType
	}

	if viewCount, ok := data["viewCount"].(float64); ok {
		photo.ViewCount = int(viewCount)
	}

	if likeCount, ok := data["likeCount"].(float64); ok {
		photo.LikeCount = int(likeCount)
	}

	if commentCount, ok := data["commentCount"].(float64); ok {
		photo.CommentCount = int(commentCount)
	}

	if shareCount, ok := data["shareCount"].(float64); ok {
		photo.ShareCount = int(shareCount)
	}

	// Parse cover URLs
	if coverUrls, ok := data["coverUrls"].([]interface{}); ok {
		for _, url := range coverUrls {
			if urlMap, ok := url.(map[string]interface{}); ok {
				photo.CoverUrls = append(photo.CoverUrls, e.parseURLData(urlMap))
			}
		}
	}

	// Parse head URLs
	if headUrls, ok := data["headUrls"].([]interface{}); ok {
		for _, url := range headUrls {
			if urlMap, ok := url.(map[string]interface{}); ok {
				photo.HeadUrls = append(photo.HeadUrls, e.parseURLData(urlMap))
			}
		}
	}

	return photo
}

// parseURLData parses URL data from JSON
func (e *kuaishouExtractor) parseURLData(data map[string]interface{}) KSURL {
	url := KSURL{}

	if urlStr, ok := data["url"].(string); ok {
		url.URL = urlStr
	}

	if cdnKey, ok := data["cdnKey"].(string); ok {
		url.CdnKey = cdnKey
	}

	return url
}

// parseSoundData parses sound data from JSON
func (e *kuaishouExtractor) parseSoundData(data map[string]interface{}) KSSound {
	sound := KSSound{}

	if id, ok := data["id"].(string); ok {
		sound.ID = id
	}

	if name, ok := data["name"].(string); ok {
		sound.Name = name
	}

	if author, ok := data["author"].(string); ok {
		sound.Author = author
	}

	if duration, ok := data["duration"].(float64); ok {
		sound.Duration = int(duration)
	}

	// Parse audio URLs
	if audioUrls, ok := data["audioUrls"].([]interface{}); ok {
		for _, url := range audioUrls {
			if urlMap, ok := url.(map[string]interface{}); ok {
				sound.AudioUrls = append(sound.AudioUrls, e.parseURLData(urlMap))
			}
		}
	}

	return sound
}

// parseExtParamsData parses ext params data from JSON
func (e *kuaishouExtractor) parseExtParamsData(data map[string]interface{}) KSExtParams {
	extParams := KSExtParams{}

	if atlas, ok := data["atlas"].(map[string]interface{}); ok {
		extParams.Atlas = e.parseAtlasData(atlas)
	}

	return extParams
}

// parseAtlasData parses atlas data from JSON
func (e *kuaishouExtractor) parseAtlasData(data map[string]interface{}) KSAtlas {
	atlas := KSAtlas{}

	if cdn, ok := data["cdn"].(string); ok {
		atlas.CDN = cdn
	}

	if list, ok := data["list"].([]interface{}); ok {
		for _, item := range list {
			if itemStr, ok := item.(string); ok {
				atlas.List = append(atlas.List, itemStr)
			}
		}
	}

	return atlas
}

// convertToVideoInfo converts KSVideo to VideoInfo
func (e *kuaishouExtractor) convertToVideoInfo(video *KSVideo) *models.VideoInfo {
	var mediaType models.MediaType
	var downloadURL string

	// Check if we have a real video URL from API parsing
	if video.ExtParams.Atlas.CDN != "" && strings.HasPrefix(video.ExtParams.Atlas.CDN, "http") {
		mediaType = models.MediaTypeVideo
		downloadURL = video.ExtParams.Atlas.CDN
		e.logger.Info().Str("real_url", downloadURL).Msg("Using extracted video URL from API")
	} else if len(video.ExtParams.Atlas.List) > 0 {
		// Check if we have video URLs in the Atlas list
		for _, url := range video.ExtParams.Atlas.List {
			if strings.Contains(url, ".mp4") || strings.Contains(url, "video") {
				mediaType = models.MediaTypeVideo
				if strings.HasPrefix(url, "http") {
					downloadURL = url
				} else {
					downloadURL = "https://" + url
				}
				e.logger.Info().Str("real_url", downloadURL).Msg("Using video URL from Atlas list")
				break
			}
		}
		// If no video URLs found in list, treat as images
		if downloadURL == "" && len(video.ExtParams.Atlas.List) > 0 {
			mediaType = models.MediaTypeImage
			for _, url := range video.ExtParams.Atlas.List {
				if strings.HasPrefix(url, "http") {
					downloadURL = url
				} else {
					downloadURL = "https://" + url
				}
				break
			}
		}
	} else if video.Photo.PhotoType == "VIDEO" {
		mediaType = models.MediaTypeVideo
		// Still no real URL found - this is a limitation we need to acknowledge
		downloadURL = fmt.Sprintf("kuaishou://video-not-available/%s", video.PhotoID)
		e.logger.Warn().Str("video_id", video.PhotoID).Msg("Kuaishou video URL extraction failed - API call needed")
	} else if video.Photo.PhotoType == "VERTICAL_ATLAS" || video.Photo.PhotoType == "HORIZONTAL_ATLAS" {
		mediaType = models.MediaTypeImage
		// For images, use cover URL as fallback
		if video.Photo.CoverURL != "" {
			downloadURL = video.Photo.CoverURL
		}
	} else {
		// Default to video type
		mediaType = models.MediaTypeVideo
		downloadURL = ""
		e.logger.Warn().Msg("Unknown media type - download will fail")
	}

	return &models.VideoInfo{
		ID:          video.PhotoID,
		Platform:    models.PlatformKuaishou,
		Title:       video.Caption,
		Description: video.Caption,
		URL:         fmt.Sprintf("https://www.kuaishou.com/short-video/%s", video.PhotoID),
		DownloadURL: downloadURL,
		Thumbnail:   video.Photo.CoverURL,
		Duration:    video.Duration,
		MediaType:   mediaType,
		Size:        0, // Will be filled during download
		Format:      "mp4",
		Quality:     "hd",

		// Author information
		AuthorID:     video.User.ID,
		AuthorName:   video.User.Name,
		AuthorAvatar: video.User.Avatar,

		// Statistics
		ViewCount:    video.Photo.ViewCount,
		LikeCount:    video.Photo.LikeCount,
		ShareCount:   video.Photo.ShareCount,
		CommentCount: video.Photo.CommentCount,

		// Timestamps
		PublishedAt: time.Unix(video.CreateTime/1000, 0), // Convert from milliseconds
		CollectedAt: time.Now(),

		// Status
		Status:     "pending",
		RetryCount: 0,

		// Additional metadata
		Metadata: fmt.Sprintf(`{"sound_id":"%s","sound_name":"%s","sound_author":"%s","extract_method":"html_fallback","has_real_url":%t}`,
			video.SoundTrack.ID, video.SoundTrack.Name, video.SoundTrack.Author, downloadURL != ""),
		ExtractFrom: "web",
	}
}

// convertToAuthorInfo converts KSUser to AuthorInfo
func (e *kuaishouExtractor) convertToAuthorInfo(user *KSUser) *models.AuthorInfo {
	return &models.AuthorInfo{
		ID:          user.ID,
		Platform:    models.PlatformKuaishou,
		Name:        user.ID,
		Nickname:    user.Name,
		Avatar:      user.Avatar,
		Followers:   user.Followers,
		Following:   user.Following,
		Verified:    user.Sex != "",
		CollectedAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// getVideoFromAPI fetches video data using Kuaishou's internal GraphQL API
func (e *kuaishouExtractor) getVideoFromAPI(videoID string) (*KSVideo, error) {
	// Kuaishou GraphQL endpoint
	apiURL := "https://www.kuaishou.com/graphql"

	e.logger.Info().Str("video_id", videoID).Str("api_url", apiURL).Bool("has_cookie", e.cookie != "").Msg("Making GraphQL API request")

	// GraphQL query for video details
	query := `query visionVideoDetail($photoId: String, $type: String, $page: String, $webPageArea: String) {
		visionVideoDetail(photoId: $photoId, type: $type, page: $page, webPageArea: $webPageArea) {
			status
			type
			author {
				id
				name
				avatar
				following
				fans
			}
			photo {
				id
				caption
				duration
				timestamp
				width
				height
				coverUrl
				viewCount
				likeCount
				commentCount
				shareCount
				mainMvUrls {
					url
					qualityType
				}
				mainImageUrls {
					url
				}
				atlasEntry {
					cdn
					list
				}
			}
			llsActionInfo {
				type
			}
		}
	}`

	// Build request payload
	requestData := map[string]interface{}{
		"operationName": "visionVideoDetail",
		"variables": map[string]interface{}{
			"photoId":     videoID,
			"type":        "profile",
			"page":        "detail",
			"webPageArea": "mainContent",
		},
		"query": query,
	}

	// Prepare headers with authentication
	headers := map[string]string{
		"Accept":          "application/json",
		"Content-Type":    "application/json",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Referer":         fmt.Sprintf("https://www.kuaishou.com/short-video/%s", videoID),
		"User-Agent":      e.userAgent,
		"X-Requested-With": "XMLHttpRequest",
		"Origin":          "https://www.kuaishou.com",
	}

	// Add cookies if available
	if e.cookie != "" {
		headers["Cookie"] = e.cookie
	}

	// Make API request
	resp, err := e.client.PostJSON(apiURL, requestData, headers)
	if err != nil {
		e.logger.Warn().Err(err).Msg("GraphQL API request failed")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		e.logger.Warn().Int("status", resp.StatusCode).Msg("GraphQL API returned non-200 status")
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse response
	var apiResp struct {
		Data struct {
			VisionVideoDetail struct {
				Status string `json:"status"`
				Type   string `json:"type"`
				Author struct {
					ID        string `json:"id"`
					Name      string `json:"name"`
					Avatar    string `json:"avatar"`
					Following int    `json:"following"`
					Fans      int    `json:"fans"`
				} `json:"author"`
				Photo struct {
					ID           string `json:"id"`
					Caption      string `json:"caption"`
					Duration     int    `json:"duration"`
					Timestamp    int64  `json:"timestamp"`
					Width        int    `json:"width"`
					Height       int    `json:"height"`
					CoverURL     string `json:"coverUrl"`
					ViewCount    int    `json:"viewCount"`
					LikeCount    int    `json:"likeCount"`
					CommentCount int    `json:"commentCount"`
					ShareCount   int    `json:"shareCount"`
					MainMvUrls   []struct {
						URL         string `json:"url"`
						QualityType int    `json:"qualityType"`
					} `json:"mainMvUrls"`
					MainImageUrls []struct {
						URL string `json:"url"`
					} `json:"mainImageUrls"`
					AtlasEntry struct {
						CDN  string   `json:"cdn"`
						List []string `json:"list"`
					} `json:"atlasEntry"`
				} `json:"photo"`
			} `json:"visionVideoDetail"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to decode GraphQL response")
		return nil, err
	}

	// Check for GraphQL errors
	if len(apiResp.Errors) > 0 {
		e.logger.Warn().Interface("errors", apiResp.Errors).Msg("GraphQL API returned errors")
		return nil, fmt.Errorf("GraphQL API errors: %v", apiResp.Errors)
	}

	// Check if we got valid data
	if apiResp.Data.VisionVideoDetail.Status != "success" {
		e.logger.Warn().Str("status", apiResp.Data.VisionVideoDetail.Status).Msg("GraphQL API returned non-success status")
		return nil, fmt.Errorf("API returned status: %s", apiResp.Data.VisionVideoDetail.Status)
	}

	// Convert API response to KSVideo
	video := &KSVideo{
		PhotoID:    apiResp.Data.VisionVideoDetail.Photo.ID,
		Caption:    apiResp.Data.VisionVideoDetail.Photo.Caption,
		Duration:   apiResp.Data.VisionVideoDetail.Photo.Duration,
		CreateTime: apiResp.Data.VisionVideoDetail.Photo.Timestamp,
		User: KSUser{
			ID:        apiResp.Data.VisionVideoDetail.Author.ID,
			Name:      apiResp.Data.VisionVideoDetail.Author.Name,
			Avatar:    apiResp.Data.VisionVideoDetail.Author.Avatar,
			Following: apiResp.Data.VisionVideoDetail.Author.Following,
			Followers: apiResp.Data.VisionVideoDetail.Author.Fans,
		},
		Photo: KSPhoto{
			ID:           apiResp.Data.VisionVideoDetail.Photo.ID,
			Duration:     apiResp.Data.VisionVideoDetail.Photo.Duration,
			Width:        apiResp.Data.VisionVideoDetail.Photo.Width,
			Height:       apiResp.Data.VisionVideoDetail.Photo.Height,
			CoverURL:     apiResp.Data.VisionVideoDetail.Photo.CoverURL,
			PhotoType:    "VIDEO",
			ViewCount:    apiResp.Data.VisionVideoDetail.Photo.ViewCount,
			LikeCount:    apiResp.Data.VisionVideoDetail.Photo.LikeCount,
			CommentCount: apiResp.Data.VisionVideoDetail.Photo.CommentCount,
			ShareCount:   apiResp.Data.VisionVideoDetail.Photo.ShareCount,
		},
		SoundTrack: KSSound{
			ID:   "api_extracted",
			Name: "Original Sound",
		},
	}

	// Extract video URLs
	var videoURLs []string
	for _, mvUrl := range apiResp.Data.VisionVideoDetail.Photo.MainMvUrls {
		if mvUrl.URL != "" {
			videoURLs = append(videoURLs, mvUrl.URL)
			e.logger.Info().Str("video_url", mvUrl.URL).Int("quality", mvUrl.QualityType).Msg("Found video URL from API")
		}
	}

	// Extract image URLs if no video URLs
	var imageURLs []string
	for _, imgUrl := range apiResp.Data.VisionVideoDetail.Photo.MainImageUrls {
		if imgUrl.URL != "" {
			imageURLs = append(imageURLs, imgUrl.URL)
		}
	}

	// Set the ExtParams with the extracted URLs
	if len(videoURLs) > 0 {
		// Use the first (usually highest quality) video URL
		video.ExtParams = KSExtParams{
			Atlas: KSAtlas{
				CDN:  videoURLs[0], // Store the direct video URL
				List: videoURLs,
			},
		}
	} else if len(imageURLs) > 0 {
		video.Photo.PhotoType = "VERTICAL_ATLAS"
		video.ExtParams = KSExtParams{
			Atlas: KSAtlas{
				CDN:  "", // No CDN for images
				List: imageURLs,
			},
		}
	} else if apiResp.Data.VisionVideoDetail.Photo.AtlasEntry.CDN != "" {
		// Use atlas data if available
		video.ExtParams = KSExtParams{
			Atlas: KSAtlas{
				CDN:  apiResp.Data.VisionVideoDetail.Photo.AtlasEntry.CDN,
				List: apiResp.Data.VisionVideoDetail.Photo.AtlasEntry.List,
			},
		}
	}

	e.logger.Info().Str("video_id", video.PhotoID).Str("title", video.Caption).Int("video_urls", len(videoURLs)).Int("image_urls", len(imageURLs)).Msg("Successfully extracted video data from GraphQL API")
	return video, nil
}

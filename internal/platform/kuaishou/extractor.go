package kuaishou

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	
	"github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	
	"video-downloader/internal/utils"
	"video-downloader/pkg/models"
)

// kuaishouExtractor implements the PlatformExtractor interface for Kuaishou
type kuaishouExtractor struct {
	client     *utils.HTTPClient
	config     *models.ExtractorConfig
	logger     *logrus.Logger
	userAgent  string
	cookie     string
}

// KSVideo represents Kuaishou video data
type KSVideo struct {
	PhotoID     string      `json:"photoId"`
	Caption     string      `json:"caption"`
	Duration    int         `json:"duration"`
	CreateTime  int64       `json:"timestamp"`
	User        KSUser      `json:"user"`
	Photo       KSPhoto     `json:"photo"`
	SoundTrack  KSSound     `json:"soundTrack"`
	ExtParams   KSExtParams `json:"ext_params"`
}

type KSUser struct {
	ID          string `json:"userId"`
	Eid         string `json:"userEid"`
	Name        string `json:"userName"`
	Avatar      string `json:"headUrl"`
	Sex         string `json:"userSex"`
	Following   int    `json:"following"`
	Followers   int    `json:"fans"`
}

type KSPhoto struct {
	ID          string    `json:"id"`
	Duration    int       `json:"duration"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	CoverURL    string    `json:"coverUrl"`
	PhotoType   string    `json:"photoType"`
	ViewCount   int       `json:"viewCount"`
	LikeCount   int       `json:"likeCount"`
	CommentCount int      `json:"commentCount"`
	ShareCount  int       `json:"shareCount"`
	CoverUrls   []KSURL   `json:"coverUrls"`
	HeadUrls    []KSURL   `json:"headUrls"`
}

type KSURL struct {
	URL    string `json:"url"`
	CdnKey string `json:"cdnKey"`
}

type KSSound struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Author    string `json:"author"`
	Duration  int    `json:"duration"`
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
		CacheKey string `json:"cacheKey"`
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
		client:    utils.NewHTTPClient(utils.ClientConfig{
			Timeout:     config.Timeout,
			MaxRetries:  config.MaxRetries,
			ProxyURL:    config.Proxy,
			UserAgent:   userAgent,
			Cookie:      config.Cookie,
			TLSInsecure: true,
		}),
		config:    config,
		logger:    logrus.New(),
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

// getVideoData fetches video data from Kuaishou
func (e *kuaishouExtractor) getVideoData(videoID string) (*KSVideo, error) {
	// Kuaishou doesn't have a public API, so we need to scrape the web page
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
	
	// Parse HTML to extract video data
	return e.extractVideoFromHTML(resp.Body)
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
func (e *kuaishouExtractor) extractVideoFromHTML(body io.Reader) (*KSVideo, error) {
	// Parse HTML
	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}
	
	// Look for script tags containing data
	var videoData *KSVideo
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			if n.FirstChild != nil {
				scriptContent := n.FirstChild.Data
				// Look for window.__APOLLO_STATE__
				if strings.Contains(scriptContent, "window.__APOLLO_STATE__") {
					// Extract JSON data
					start := strings.Index(scriptContent, "{")
					end := strings.LastIndex(scriptContent, "}")
					if start != -1 && end != -1 {
						jsonData := scriptContent[start : end+1]
						var apolloData KSWebData
						if err := json.Unmarshal([]byte(jsonData), &apolloData); err == nil {
							videoData = e.parseApolloData(apolloData)
							return
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	
	if videoData == nil {
		return nil, fmt.Errorf("video data not found in HTML")
	}
	
	return videoData, nil
}

// extractUserFromHTML extracts user data from HTML
func (e *kuaishouExtractor) extractUserFromHTML(body io.Reader) (*KSUser, error) {
	// Similar to extractVideoFromHTML but for user data
	// This is a simplified implementation
	return &KSUser{}, nil
}

// parseApolloData parses Apollo state data to extract video information
func (e *kuaishouExtractor) parseApolloData(data KSWebData) *KSVideo {
	// Navigate through the JSON structure to find video data
	// The structure is complex and nested, so we need to search for the video object
	
	// Look for video data in the cache
	if cacheData, ok := data.DefaultClient.Data[data.DefaultClient.CacheKey]; ok {
		if videoMap, ok := cacheData.(map[string]interface{}); ok {
			// Look for video data in the map
			for key, value := range videoMap {
				if strings.Contains(key, "VisionVideoDetailPhoto:") {
					if videoDetail, ok := value.(map[string]interface{}); ok {
						return e.parseVideoDetail(videoDetail)
					}
				}
			}
		}
	}
	
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
	
	if video.Photo.PhotoType == "VIDEO" {
		mediaType = models.MediaTypeVideo
		// For videos, we need to extract the actual download URL
		// This might be in a different format or require additional processing
		downloadURL = video.Photo.CoverURL // Placeholder
	} else if video.Photo.PhotoType == "VERTICAL_ATLAS" || video.Photo.PhotoType == "HORIZONTAL_ATLAS" {
		mediaType = models.MediaTypeImage
		// For images, construct URLs from atlas data
		if len(video.ExtParams.Atlas.List) > 0 {
			// Use first image URL
			downloadURL = fmt.Sprintf("https://%s%s", video.ExtParams.Atlas.CDN, video.ExtParams.Atlas.List[0])
		}
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
		PublishedAt:  time.Unix(video.CreateTime/1000, 0), // Convert from milliseconds
		CollectedAt:  time.Now(),
		
		// Status
		Status:      "pending",
		RetryCount:  0,
		
		// Additional metadata
		Metadata:    fmt.Sprintf(`{"sound_id":"%s","sound_name":"%s","sound_author":"%s"}`,
			video.SoundTrack.ID, video.SoundTrack.Name, video.SoundTrack.Author),
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
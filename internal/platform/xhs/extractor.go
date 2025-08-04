package xhs

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
	"golang.org/x/net/html"

	"video-downloader/internal/utils"
	"video-downloader/pkg/models"
)

// xhsExtractor implements the PlatformExtractor interface for XHS (Xiaohongshu)
type xhsExtractor struct {
	client    *utils.HTTPClient
	config    *models.ExtractorConfig
	logger    zerolog.Logger
	userAgent string
	cookie    string
}

// XHSNote represents XHS note data
type XHSNote struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Desc         string       `json:"desc"`
	Type         string       `json:"type"`
	CreateTime   int64        `json:"create_time"`
	User         XHSUser      `json:"user"`
	Images       []XHSImage   `json:"images"`
	Video        XHSVideo     `json:"video"`
	Tags         []string     `json:"tags"`
	InteractInfo InteractInfo `json:"interact_info"`
}

type XHSUser struct {
	ID         string `json:"id"`
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar"`
	Desc       string `json:"desc"`
	Gender     string `json:"gender"`
	Level      int    `json:"level"`
	Followers  int    `json:"fans"`
	Following  int    `json:"follows"`
	NotesCount int    `json:"notes_count"`
	IPLocation string `json:"ip_location"`
}

type XHSImage struct {
	URL        string `json:"url"`
	URLDefault string `json:"url_default"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FileSize   int64  `json:"file_size"`
}

type XHSVideo struct {
	PlayAddr string `json:"play_addr"`
	Duration int    `json:"duration"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Cover    string `json:"cover"`
}

type InteractInfo struct {
	LikeCount    int `json:"liked_count"`
	CollectCount int `json:"collected_count"`
	CommentCount int `json:"comment_count"`
	ShareCount   int `json:"share_count"`
}

// XHSAPIResponse represents XHS API response
type XHSAPIResponse struct {
	Success bool    `json:"success"`
	Data    XHSNote `json:"data"`
	Message string  `json:"message"`
}

// NewExtractor creates a new XHS extractor
func NewExtractor(config *models.ExtractorConfig) *xhsExtractor {
	userAgent := config.UserAgent
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	}

	return &xhsExtractor{
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

// ExtractVideoInfo extracts video information from an XHS URL
func (e *xhsExtractor) ExtractVideoInfo(xhsURL string) (*models.VideoInfo, error) {
	// Extract note ID from URL
	noteID, err := e.extractNoteID(xhsURL)
	if err != nil {
		return nil, fmt.Errorf("error extracting note ID: %w", err)
	}

	// Get note data
	noteData, err := e.getNoteData(noteID)
	if err != nil {
		return nil, fmt.Errorf("error getting note data: %w", err)
	}

	// Convert to VideoInfo
	videoInfo := e.convertToVideoInfo(noteData)

	return videoInfo, nil
}

// ExtractAuthorInfo extracts author information
func (e *xhsExtractor) ExtractAuthorInfo(authorID string) (*models.AuthorInfo, error) {
	// Get user data
	userData, err := e.getUserData(authorID)
	if err != nil {
		return nil, fmt.Errorf("error getting user data: %w", err)
	}

	// Convert to AuthorInfo
	authorInfo := e.convertToAuthorInfo(userData)

	return authorInfo, nil
}

// ExtractBatch extracts multiple notes from an XHS user page
func (e *xhsExtractor) ExtractBatch(url string, limit int) ([]*models.VideoInfo, error) {
	// Extract user ID from URL
	userID, err := e.extractUserID(url)
	if err != nil {
		return nil, fmt.Errorf("error extracting user ID: %w", err)
	}

	// Get user notes
	notes, err := e.getUserNotes(userID, limit)
	if err != nil {
		return nil, fmt.Errorf("error getting user notes: %w", err)
	}

	// Convert to VideoInfo
	var videoInfos []*models.VideoInfo
	for _, note := range notes {
		videoInfo := e.convertToVideoInfo(&note)
		videoInfos = append(videoInfos, videoInfo)
	}

	return videoInfos, nil
}

// ValidateURL validates if the URL belongs to XHS
func (e *xhsExtractor) ValidateURL(url string) bool {
	patterns := e.GetSupportedURLPatterns()
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, url); matched {
			return true
		}
	}
	return false
}

// GetName returns the platform name
func (e *xhsExtractor) GetName() models.Platform {
	return models.PlatformXHS
}

// GetSupportedURLPatterns returns supported URL patterns
func (e *xhsExtractor) GetSupportedURLPatterns() []string {
	return []string{
		`https?://(?:www\.)?xiaohongshu\.com/explore/[^/]+`,
		`https?://(?:www\.)?xiaohongshu\.com/discovery/item/[^/]+`,
		`https?://(?:www\.)?xiaohongshu\.com/user/profile/[^/]+`,
		`https?://xhslink\.com/[^/]+`,
	}
}

// extractNoteID extracts note ID from XHS URL
func (e *xhsExtractor) extractNoteID(url string) (string, error) {
	// Handle short URLs first
	if strings.Contains(url, "xhslink.com") {
		return e.resolveShortURL(url)
	}

	// Extract from explore/discovery URLs
	re := regexp.MustCompile(`/(?:explore|discovery/item)/([^/?&]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid XHS URL format")
	}
	return matches[1], nil
}

// extractUserID extracts user ID from XHS URL
func (e *xhsExtractor) extractUserID(url string) (string, error) {
	re := regexp.MustCompile(`/user/profile/([^/?&]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid XHS user URL format")
	}
	return matches[1], nil
}

// resolveShortURL resolves XHS short URL
func (e *xhsExtractor) resolveShortURL(shortURL string) (string, error) {
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
	return e.extractNoteID(finalURL)
}

// getNoteData fetches note data from XHS
func (e *xhsExtractor) getNoteData(noteID string) (*XHSNote, error) {
	// XHS doesn't have a public API, so we need to scrape the web page
	noteURL := fmt.Sprintf("https://www.xiaohongshu.com/explore/%s", noteID)

	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":      e.userAgent,
		"Referer":         "https://www.xiaohongshu.com/",
	}

	if e.cookie != "" {
		headers["Cookie"] = e.cookie
	}

	resp, err := e.client.Get(noteURL, headers)
	if err != nil {
		return nil, fmt.Errorf("error fetching note page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML to extract note data
	return e.extractNoteFromHTML(resp.Body)
}

// getUserData fetches user data from XHS
func (e *xhsExtractor) getUserData(userID string) (*XHSUser, error) {
	userURL := fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s", userID)

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

// getUserNotes fetches user notes from XHS
func (e *xhsExtractor) getUserNotes(userID string, limit int) ([]XHSNote, error) {
	// This would typically require pagination and handling of XHS's dynamic loading
	// For now, return empty slice
	return []XHSNote{}, nil
}

// extractNoteFromHTML extracts note data from HTML
func (e *xhsExtractor) extractNoteFromHTML(body io.Reader) (*XHSNote, error) {
	// Parse HTML
	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}

	// Look for script tags containing data
	var noteData *XHSNote
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			for _, attr := range n.Attr {
				if attr.Key == "id" && attr.Val == "__NEXT_DATA__" {
					if n.FirstChild != nil {
						var nextData map[string]interface{}
						if err := json.Unmarshal([]byte(n.FirstChild.Data), &nextData); err == nil {
							// Extract note data from __NEXT_DATA__
							noteData = e.parseNextData(nextData)
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

	if noteData == nil {
		return nil, fmt.Errorf("note data not found in HTML")
	}

	return noteData, nil
}

// extractUserFromHTML extracts user data from HTML
func (e *xhsExtractor) extractUserFromHTML(body io.Reader) (*XHSUser, error) {
	// Similar to extractNoteFromHTML but for user data
	// This is a simplified implementation
	return &XHSUser{}, nil
}

// parseNextData parses __NEXT_DATA__ to extract note information
func (e *xhsExtractor) parseNextData(data map[string]interface{}) *XHSNote {
	// Navigate through the JSON structure to find note data
	// This is a simplified approach - the actual structure is more complex
	if props, ok := data["props"].(map[string]interface{}); ok {
		if pageProps, ok := props["pageProps"].(map[string]interface{}); ok {
			if note, ok := pageProps["note"].(map[string]interface{}); ok {
				return e.parseNoteData(note)
			}
		}
	}
	return nil
}

// parseNoteData parses note data from JSON
func (e *xhsExtractor) parseNoteData(data map[string]interface{}) *XHSNote {
	note := &XHSNote{}

	if id, ok := data["id"].(string); ok {
		note.ID = id
	}

	if title, ok := data["title"].(string); ok {
		note.Title = title
	}

	if desc, ok := data["desc"].(string); ok {
		note.Desc = desc
	}

	if noteType, ok := data["type"].(string); ok {
		note.Type = noteType
	}

	if createTime, ok := data["create_time"].(float64); ok {
		note.CreateTime = int64(createTime)
	}

	// Parse user data
	if user, ok := data["user"].(map[string]interface{}); ok {
		note.User = e.parseUserData(user)
	}

	// Parse images
	if images, ok := data["images"].([]interface{}); ok {
		for _, img := range images {
			if imgMap, ok := img.(map[string]interface{}); ok {
				note.Images = append(note.Images, e.parseImageData(imgMap))
			}
		}
	}

	// Parse video
	if video, ok := data["video"].(map[string]interface{}); ok {
		note.Video = e.parseVideoData(video)
	}

	return note
}

// parseUserData parses user data from JSON
func (e *xhsExtractor) parseUserData(data map[string]interface{}) XHSUser {
	user := XHSUser{}

	if id, ok := data["id"].(string); ok {
		user.ID = id
	}

	if nickname, ok := data["nickname"].(string); ok {
		user.Nickname = nickname
	}

	if avatar, ok := data["avatar"].(string); ok {
		user.Avatar = avatar
	}

	if desc, ok := data["desc"].(string); ok {
		user.Desc = desc
	}

	if followers, ok := data["fans"].(float64); ok {
		user.Followers = int(followers)
	}

	return user
}

// parseImageData parses image data from JSON
func (e *xhsExtractor) parseImageData(data map[string]interface{}) XHSImage {
	img := XHSImage{}

	if url, ok := data["url"].(string); ok {
		img.URL = url
	}

	if urlDefault, ok := data["url_default"].(string); ok {
		img.URLDefault = urlDefault
	}

	if width, ok := data["width"].(float64); ok {
		img.Width = int(width)
	}

	if height, ok := data["height"].(float64); ok {
		img.Height = int(height)
	}

	return img
}

// parseVideoData parses video data from JSON
func (e *xhsExtractor) parseVideoData(data map[string]interface{}) XHSVideo {
	video := XHSVideo{}

	if playAddr, ok := data["play_addr"].(string); ok {
		video.PlayAddr = playAddr
	}

	if duration, ok := data["duration"].(float64); ok {
		video.Duration = int(duration)
	}

	if width, ok := data["width"].(float64); ok {
		video.Width = int(width)
	}

	if height, ok := data["height"].(float64); ok {
		video.Height = int(height)
	}

	if cover, ok := data["cover"].(string); ok {
		video.Cover = cover
	}

	return video
}

// convertToVideoInfo converts XHSNote to VideoInfo
func (e *xhsExtractor) convertToVideoInfo(note *XHSNote) *models.VideoInfo {
	var mediaType models.MediaType
	var downloadURL string
	var duration int

	if note.Type == "video" && note.Video.PlayAddr != "" {
		mediaType = models.MediaTypeVideo
		downloadURL = note.Video.PlayAddr
		duration = note.Video.Duration
	} else if len(note.Images) > 0 {
		mediaType = models.MediaTypeImage
		// For images, we'll use the first image URL
		downloadURL = note.Images[0].URL
		duration = 0
	}

	return &models.VideoInfo{
		ID:          note.ID,
		Platform:    models.PlatformXHS,
		Title:       note.Title,
		Description: note.Desc,
		URL:         fmt.Sprintf("https://www.xiaohongshu.com/explore/%s", note.ID),
		DownloadURL: downloadURL,
		Thumbnail:   note.Video.Cover,
		Duration:    duration,
		MediaType:   mediaType,
		Size:        0, // Will be filled during download
		Format:      "mp4",
		Quality:     "hd",

		// Author information
		AuthorID:     note.User.ID,
		AuthorName:   note.User.Nickname,
		AuthorAvatar: note.User.Avatar,

		// Statistics
		LikeCount:    note.InteractInfo.LikeCount,
		ShareCount:   note.InteractInfo.ShareCount,
		CommentCount: note.InteractInfo.CommentCount,

		// Timestamps
		PublishedAt: time.Unix(note.CreateTime, 0),
		CollectedAt: time.Now(),

		// Status
		Status:     "pending",
		RetryCount: 0,

		// Additional metadata
		Metadata:    fmt.Sprintf(`{"tags":%s}`, strings.Join(note.Tags, ",")),
		ExtractFrom: "web",
	}
}

// convertToAuthorInfo converts XHSUser to AuthorInfo
func (e *xhsExtractor) convertToAuthorInfo(user *XHSUser) *models.AuthorInfo {
	return &models.AuthorInfo{
		ID:          user.ID,
		Platform:    models.PlatformXHS,
		Name:        user.ID,
		Nickname:    user.Nickname,
		Avatar:      user.Avatar,
		Description: user.Desc,
		Followers:   user.Followers,
		Following:   user.Following,
		VideoCount:  user.NotesCount,
		Verified:    user.Level > 0,
		CollectedAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
}

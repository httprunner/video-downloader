package comment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"video-downloader/internal/utils"
	"video-downloader/pkg/models"
)

// CommentExtractor extracts comments from video platforms
type CommentExtractor struct {
	client *utils.HTTPClient
	logger zerolog.Logger
}

// Comment represents a single comment
type Comment struct {
	ID           string          `json:"id"`
	VideoID      string          `json:"video_id"`
	Platform     models.Platform `json:"platform"`
	AuthorID     string          `json:"author_id"`
	AuthorName   string          `json:"author_name"`
	AuthorAvatar string          `json:"author_avatar"`
	Content      string          `json:"content"`
	LikeCount    int             `json:"like_count"`
	ReplyCount   int             `json:"reply_count"`
	CreatedAt    time.Time       `json:"created_at"`
	ParentID     string          `json:"parent_id"` // For replies
	Level        int             `json:"level"`     // Comment depth
	Mentions     []string        `json:"mentions"`  // Mentioned users
	IsVerified   bool            `json:"is_verified"`
	IsLiked      bool            `json:"is_liked"`
	IPLocation   string          `json:"ip_location"`
}

// CommentThread represents a comment with its replies
type CommentThread struct {
	Comment *Comment   `json:"comment"`
	Replies []*Comment `json:"replies"`
	Total   int        `json:"total"`
	HasMore bool       `json:"has_more"`
}

// CommentExtractConfig holds configuration for comment extraction
type CommentExtractConfig struct {
	VideoID        string
	Platform       models.Platform
	Limit          int
	IncludeReplies bool
	SortBy         string // time, popularity
	Cookie         string
	UserAgent      string
}

// NewCommentExtractor creates a new comment extractor
func NewCommentExtractor() *CommentExtractor {
	return &CommentExtractor{
		client: utils.NewHTTPClient(utils.ClientConfig{
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		}),
		logger: zerolog.New(nil).With().Str("component", "comment_extractor").Logger(),
	}
}

// ExtractComments extracts comments from a video
func (ce *CommentExtractor) ExtractComments(config CommentExtractConfig) ([]*CommentThread, error) {
	switch config.Platform {
	case models.PlatformTikTok:
		return ce.extractTikTokComments(config)
	case models.PlatformXHS:
		return ce.extractXHSComments(config)
	case models.PlatformKuaishou:
		return ce.extractKuaishouComments(config)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", config.Platform)
	}
}

// extractTikTokComments extracts comments from TikTok
func (ce *CommentExtractor) extractTikTokComments(config CommentExtractConfig) ([]*CommentThread, error) {
	// TikTok comment API endpoint
	apiURL := fmt.Sprintf("https://www.tiktok.com/api/comment/list/?aweme_id=%s&count=%d&cursor=0",
		config.VideoID, config.Limit)

	headers := map[string]string{
		"Accept":          "application/json",
		"Accept-Language": "en-US,en;q=0.9",
		"Referer":         fmt.Sprintf("https://www.tiktok.com/@user/video/%s", config.VideoID),
		"User-Agent":      config.UserAgent,
	}

	if config.Cookie != "" {
		headers["Cookie"] = config.Cookie
	}

	resp, err := ce.client.Get(apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch TikTok comments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TikTok API returned status %d", resp.StatusCode)
	}

	var apiResp struct {
		Comments []struct {
			CID  string `json:"cid"`
			Text string `json:"text"`
			User struct {
				UID      string `json:"uid"`
				Nickname string `json:"nickname"`
				Avatar   string `json:"avatar_thumb"`
			} `json:"user"`
			DiggCount     int   `json:"digg_count"`
			ReplyCount    int   `json:"reply_comment_total"`
			CreateTime    int64 `json:"create_time"`
			ReplyComments []struct {
				CID  string `json:"cid"`
				Text string `json:"text"`
				User struct {
					UID      string `json:"uid"`
					Nickname string `json:"nickname"`
					Avatar   string `json:"avatar_thumb"`
				} `json:"user"`
				DiggCount  int   `json:"digg_count"`
				CreateTime int64 `json:"create_time"`
			} `json:"reply_comments"`
		} `json:"comments"`
		Total   int  `json:"total"`
		HasMore bool `json:"has_more"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode TikTok comment response: %w", err)
	}

	var threads []*CommentThread
	for _, comment := range apiResp.Comments {
		thread := &CommentThread{
			Comment: &Comment{
				ID:           comment.CID,
				VideoID:      config.VideoID,
				Platform:     models.PlatformTikTok,
				AuthorID:     comment.User.UID,
				AuthorName:   comment.User.Nickname,
				AuthorAvatar: comment.User.Avatar,
				Content:      comment.Text,
				LikeCount:    comment.DiggCount,
				ReplyCount:   comment.ReplyCount,
				CreatedAt:    time.Unix(comment.CreateTime, 0),
				Level:        0,
				Mentions:     ce.extractMentions(comment.Text),
			},
			Replies: make([]*Comment, 0),
			Total:   comment.ReplyCount,
			HasMore: comment.ReplyCount > len(comment.ReplyComments),
		}

		// Add replies if requested
		if config.IncludeReplies {
			for _, reply := range comment.ReplyComments {
				replyComment := &Comment{
					ID:           reply.CID,
					VideoID:      config.VideoID,
					Platform:     models.PlatformTikTok,
					AuthorID:     reply.User.UID,
					AuthorName:   reply.User.Nickname,
					AuthorAvatar: reply.User.Avatar,
					Content:      reply.Text,
					LikeCount:    reply.DiggCount,
					CreatedAt:    time.Unix(reply.CreateTime, 0),
					ParentID:     comment.CID,
					Level:        1,
					Mentions:     ce.extractMentions(reply.Text),
				}
				thread.Replies = append(thread.Replies, replyComment)
			}
		}

		threads = append(threads, thread)
	}

	return threads, nil
}

// extractXHSComments extracts comments from XHS
func (ce *CommentExtractor) extractXHSComments(config CommentExtractConfig) ([]*CommentThread, error) {
	// XHS comment API endpoint
	apiURL := fmt.Sprintf("https://edith.xiaohongshu.com/api/sns/web/v2/comment/page?note_id=%s&num=%d&cursor=",
		config.VideoID, config.Limit)

	headers := map[string]string{
		"Accept":           "application/json",
		"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8",
		"Referer":          fmt.Sprintf("https://www.xiaohongshu.com/explore/%s", config.VideoID),
		"User-Agent":       config.UserAgent,
		"X-Requested-With": "XMLHttpRequest",
	}

	if config.Cookie != "" {
		headers["Cookie"] = config.Cookie
	}

	resp, err := ce.client.Get(apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch XHS comments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("XHS API returned status %d", resp.StatusCode)
	}

	var apiResp struct {
		Data struct {
			Comments []struct {
				ID      string `json:"id"`
				Content string `json:"content"`
				User    struct {
					UserID   string `json:"user_id"`
					Nickname string `json:"nickname"`
					Avatar   string `json:"avatar"`
				} `json:"user"`
				LikedCount      int   `json:"liked_count"`
				SubCommentCount int   `json:"sub_comment_count"`
				CreateTime      int64 `json:"create_time"`
				SubComments     []struct {
					ID      string `json:"id"`
					Content string `json:"content"`
					User    struct {
						UserID   string `json:"user_id"`
						Nickname string `json:"nickname"`
						Avatar   string `json:"avatar"`
					} `json:"user"`
					LikedCount int   `json:"liked_count"`
					CreateTime int64 `json:"create_time"`
				} `json:"sub_comments"`
			} `json:"comments"`
		} `json:"data"`
		Success bool `json:"success"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode XHS comment response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("XHS API request failed")
	}

	var threads []*CommentThread
	for _, comment := range apiResp.Data.Comments {
		thread := &CommentThread{
			Comment: &Comment{
				ID:           comment.ID,
				VideoID:      config.VideoID,
				Platform:     models.PlatformXHS,
				AuthorID:     comment.User.UserID,
				AuthorName:   comment.User.Nickname,
				AuthorAvatar: comment.User.Avatar,
				Content:      comment.Content,
				LikeCount:    comment.LikedCount,
				ReplyCount:   comment.SubCommentCount,
				CreatedAt:    time.Unix(comment.CreateTime/1000, 0), // XHS uses milliseconds
				Level:        0,
				Mentions:     ce.extractMentions(comment.Content),
			},
			Replies: make([]*Comment, 0),
			Total:   comment.SubCommentCount,
			HasMore: comment.SubCommentCount > len(comment.SubComments),
		}

		// Add replies if requested
		if config.IncludeReplies {
			for _, reply := range comment.SubComments {
				replyComment := &Comment{
					ID:           reply.ID,
					VideoID:      config.VideoID,
					Platform:     models.PlatformXHS,
					AuthorID:     reply.User.UserID,
					AuthorName:   reply.User.Nickname,
					AuthorAvatar: reply.User.Avatar,
					Content:      reply.Content,
					LikeCount:    reply.LikedCount,
					CreatedAt:    time.Unix(reply.CreateTime/1000, 0),
					ParentID:     comment.ID,
					Level:        1,
					Mentions:     ce.extractMentions(reply.Content),
				}
				thread.Replies = append(thread.Replies, replyComment)
			}
		}

		threads = append(threads, thread)
	}

	return threads, nil
}

// extractKuaishouComments extracts comments from Kuaishou
func (ce *CommentExtractor) extractKuaishouComments(config CommentExtractConfig) ([]*CommentThread, error) {
	// Kuaishou comment API endpoint
	apiURL := fmt.Sprintf("https://www.kuaishou.com/graphql")

	requestData := map[string]interface{}{
		"operationName": "commentListQuery",
		"variables": map[string]interface{}{
			"photoId": config.VideoID,
			"pcursor": "",
			"count":   config.Limit,
		},
		"query": `query commentListQuery($photoId: String, $pcursor: String, $count: Int) {
			visionCommentList(photoId: $photoId, pcursor: $pcursor, count: $count) {
				pcursor
				comments {
					commentId
					content
					user {
						id
						name
						avatar
					}
					likeCount
					subCommentCount
					createTime
					subComments {
						commentId
						content
						user {
							id
							name
							avatar
						}
						likeCount
						createTime
					}
				}
			}
		}`,
	}

	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
		"Referer":      fmt.Sprintf("https://www.kuaishou.com/short-video/%s", config.VideoID),
		"User-Agent":   config.UserAgent,
	}

	if config.Cookie != "" {
		headers["Cookie"] = config.Cookie
	}

	resp, err := ce.client.PostJSON(apiURL, requestData, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Kuaishou comments: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Kuaishou API returned status %d", resp.StatusCode)
	}

	var apiResp struct {
		Data struct {
			VisionCommentList struct {
				Comments []struct {
					CommentID string `json:"commentId"`
					Content   string `json:"content"`
					User      struct {
						ID     string `json:"id"`
						Name   string `json:"name"`
						Avatar string `json:"avatar"`
					} `json:"user"`
					LikeCount       int   `json:"likeCount"`
					SubCommentCount int   `json:"subCommentCount"`
					CreateTime      int64 `json:"createTime"`
					SubComments     []struct {
						CommentID string `json:"commentId"`
						Content   string `json:"content"`
						User      struct {
							ID     string `json:"id"`
							Name   string `json:"name"`
							Avatar string `json:"avatar"`
						} `json:"user"`
						LikeCount  int   `json:"likeCount"`
						CreateTime int64 `json:"createTime"`
					} `json:"subComments"`
				} `json:"comments"`
			} `json:"visionCommentList"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode Kuaishou comment response: %w", err)
	}

	var threads []*CommentThread
	for _, comment := range apiResp.Data.VisionCommentList.Comments {
		thread := &CommentThread{
			Comment: &Comment{
				ID:           comment.CommentID,
				VideoID:      config.VideoID,
				Platform:     models.PlatformKuaishou,
				AuthorID:     comment.User.ID,
				AuthorName:   comment.User.Name,
				AuthorAvatar: comment.User.Avatar,
				Content:      comment.Content,
				LikeCount:    comment.LikeCount,
				ReplyCount:   comment.SubCommentCount,
				CreatedAt:    time.Unix(comment.CreateTime/1000, 0), // Kuaishou uses milliseconds
				Level:        0,
				Mentions:     ce.extractMentions(comment.Content),
			},
			Replies: make([]*Comment, 0),
			Total:   comment.SubCommentCount,
			HasMore: comment.SubCommentCount > len(comment.SubComments),
		}

		// Add replies if requested
		if config.IncludeReplies {
			for _, reply := range comment.SubComments {
				replyComment := &Comment{
					ID:           reply.CommentID,
					VideoID:      config.VideoID,
					Platform:     models.PlatformKuaishou,
					AuthorID:     reply.User.ID,
					AuthorName:   reply.User.Name,
					AuthorAvatar: reply.User.Avatar,
					Content:      reply.Content,
					LikeCount:    reply.LikeCount,
					CreatedAt:    time.Unix(reply.CreateTime/1000, 0),
					ParentID:     comment.CommentID,
					Level:        1,
					Mentions:     ce.extractMentions(reply.Content),
				}
				thread.Replies = append(thread.Replies, replyComment)
			}
		}

		threads = append(threads, thread)
	}

	return threads, nil
}

// extractMentions extracts mentioned usernames from comment content
func (ce *CommentExtractor) extractMentions(content string) []string {
	// Common mention patterns: @username, @用户名
	re := regexp.MustCompile(`@([a-zA-Z0-9_\u4e00-\u9fa5]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	var mentions []string
	for _, match := range matches {
		if len(match) > 1 {
			mentions = append(mentions, match[1])
		}
	}

	return mentions
}

// ExportComments exports comments to different formats
func (ce *CommentExtractor) ExportComments(threads []*CommentThread, format string, filePath string) error {
	switch format {
	case "json":
		return ce.exportToJSON(threads, filePath)
	case "csv":
		return ce.exportToCSV(threads, filePath)
	case "txt":
		return ce.exportToTXT(threads, filePath)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportToJSON exports comments to JSON format
func (ce *CommentExtractor) exportToJSON(threads []*CommentThread, filePath string) error {
	data := struct {
		ExportedAt time.Time        `json:"exported_at"`
		Total      int              `json:"total"`
		Threads    []*CommentThread `json:"threads"`
	}{
		ExportedAt: time.Now(),
		Total:      len(threads),
		Threads:    threads,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal comments to JSON: %w", err)
	}

	return ce.writeToFile(filePath, jsonData)
}

// exportToCSV exports comments to CSV format
func (ce *CommentExtractor) exportToCSV(threads []*CommentThread, filePath string) error {
	var lines []string

	// CSV header
	header := "ID,VideoID,Platform,AuthorID,AuthorName,Content,LikeCount,ReplyCount,CreatedAt,ParentID,Level,Mentions"
	lines = append(lines, header)

	// Add comments and replies
	for _, thread := range threads {
		// Main comment
		mentions := strings.Join(thread.Comment.Mentions, ";")
		line := fmt.Sprintf(`"%s","%s","%s","%s","%s","%s",%d,%d,"%s","","0","%s"`,
			thread.Comment.ID,
			thread.Comment.VideoID,
			thread.Comment.Platform,
			thread.Comment.AuthorID,
			thread.Comment.AuthorName,
			strings.ReplaceAll(thread.Comment.Content, `"`, `""`),
			thread.Comment.LikeCount,
			thread.Comment.ReplyCount,
			thread.Comment.CreatedAt.Format("2006-01-02 15:04:05"),
			mentions,
		)
		lines = append(lines, line)

		// Replies
		for _, reply := range thread.Replies {
			replyMentions := strings.Join(reply.Mentions, ";")
			replyLine := fmt.Sprintf(`"%s","%s","%s","%s","%s","%s",%d,0,"%s","%s",%d,"%s"`,
				reply.ID,
				reply.VideoID,
				reply.Platform,
				reply.AuthorID,
				reply.AuthorName,
				strings.ReplaceAll(reply.Content, `"`, `""`),
				reply.LikeCount,
				reply.CreatedAt.Format("2006-01-02 15:04:05"),
				reply.ParentID,
				reply.Level,
				replyMentions,
			)
			lines = append(lines, replyLine)
		}
	}

	csvData := strings.Join(lines, "\n")
	return ce.writeToFile(filePath, []byte(csvData))
}

// exportToTXT exports comments to plain text format
func (ce *CommentExtractor) exportToTXT(threads []*CommentThread, filePath string) error {
	var content strings.Builder

	content.WriteString("Comments Export\n")
	content.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("Total Comments: %d\n", len(threads)))
	content.WriteString(strings.Repeat("=", 50) + "\n\n")

	for i, thread := range threads {
		// Main comment
		content.WriteString(fmt.Sprintf("Comment %d:\n", i+1))
		content.WriteString(fmt.Sprintf("  Author: %s (%s)\n", thread.Comment.AuthorName, thread.Comment.AuthorID))
		content.WriteString(fmt.Sprintf("  Content: %s\n", thread.Comment.Content))
		content.WriteString(fmt.Sprintf("  Likes: %d, Replies: %d\n", thread.Comment.LikeCount, thread.Comment.ReplyCount))
		content.WriteString(fmt.Sprintf("  Created: %s\n", thread.Comment.CreatedAt.Format("2006-01-02 15:04:05")))
		if len(thread.Comment.Mentions) > 0 {
			content.WriteString(fmt.Sprintf("  Mentions: %s\n", strings.Join(thread.Comment.Mentions, ", ")))
		}

		// Replies
		if len(thread.Replies) > 0 {
			content.WriteString("  Replies:\n")
			for j, reply := range thread.Replies {
				content.WriteString(fmt.Sprintf("    Reply %d:\n", j+1))
				content.WriteString(fmt.Sprintf("      Author: %s (%s)\n", reply.AuthorName, reply.AuthorID))
				content.WriteString(fmt.Sprintf("      Content: %s\n", reply.Content))
				content.WriteString(fmt.Sprintf("      Likes: %d\n", reply.LikeCount))
				content.WriteString(fmt.Sprintf("      Created: %s\n", reply.CreatedAt.Format("2006-01-02 15:04:05")))
				if len(reply.Mentions) > 0 {
					content.WriteString(fmt.Sprintf("      Mentions: %s\n", strings.Join(reply.Mentions, ", ")))
				}
			}
		}

		content.WriteString("\n" + strings.Repeat("-", 30) + "\n\n")
	}

	return ce.writeToFile(filePath, []byte(content.String()))
}

// writeToFile writes data to a file
func (ce *CommentExtractor) writeToFile(filePath string, data []byte) error {
	return nil // Implementation would write to file
}

// GetCommentStats returns statistics about comments
func (ce *CommentExtractor) GetCommentStats(threads []*CommentThread) map[string]interface{} {
	totalComments := len(threads)
	totalReplies := 0
	totalLikes := 0
	authors := make(map[string]int)

	for _, thread := range threads {
		totalLikes += thread.Comment.LikeCount
		authors[thread.Comment.AuthorID]++

		for _, reply := range thread.Replies {
			totalReplies++
			totalLikes += reply.LikeCount
			authors[reply.AuthorID]++
		}
	}

	return map[string]interface{}{
		"total_comments":        totalComments,
		"total_replies":         totalReplies,
		"total_likes":           totalLikes,
		"unique_authors":        len(authors),
		"avg_likes_per_comment": float64(totalLikes) / float64(totalComments+totalReplies),
	}
}

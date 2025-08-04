package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"video-downloader/pkg/models"
)

// ExportFormat represents different export formats
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatXLSX ExportFormat = "xlsx"
	FormatJSON ExportFormat = "json"
	FormatTXT  ExportFormat = "txt"
)

// ExportConfig holds configuration for data export
type ExportConfig struct {
	Format        ExportFormat
	FilePath      string
	Columns       []string
	DateFormat    string
	Encoding      string
	Delimiter     rune
	IncludeHeader bool
}

// DataExporter handles data export to different formats
type DataExporter struct {
	config ExportConfig
}

// NewDataExporter creates a new data exporter
func NewDataExporter(config ExportConfig) *DataExporter {
	// Set defaults
	if config.DateFormat == "" {
		config.DateFormat = "2006-01-02 15:04:05"
	}
	if config.Encoding == "" {
		config.Encoding = "UTF-8"
	}
	if config.Delimiter == 0 {
		config.Delimiter = ','
	}
	if len(config.Columns) == 0 {
		config.Columns = getDefaultColumns()
	}
	config.IncludeHeader = true

	return &DataExporter{
		config: config,
	}
}

// ExportVideos exports video data to the specified format
func (de *DataExporter) ExportVideos(videos []*models.VideoInfo) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(de.config.FilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	switch de.config.Format {
	case FormatCSV:
		return de.exportToCSV(videos)
	case FormatXLSX:
		return de.exportToXLSX(videos)
	case FormatJSON:
		return de.exportToJSON(videos)
	case FormatTXT:
		return de.exportToTXT(videos)
	default:
		return fmt.Errorf("unsupported export format: %s", de.config.Format)
	}
}

// exportToCSV exports data to CSV format
func (de *DataExporter) exportToCSV(videos []*models.VideoInfo) error {
	file, err := os.Create(de.config.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = de.config.Delimiter
	defer writer.Flush()

	// Write header
	if de.config.IncludeHeader {
		if err := writer.Write(de.config.Columns); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
	}

	// Write data rows
	for _, video := range videos {
		row := de.videoToRow(video)
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// exportToXLSX exports data to Excel format
func (de *DataExporter) exportToXLSX(videos []*models.VideoInfo) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Videos"
	f.SetSheetName("Sheet1", sheetName)

	// Set header style
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 12,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E6E6FA"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create header style: %w", err)
	}

	// Write headers
	for i, column := range de.config.Columns {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, column)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Set column widths
	columnWidths := map[string]float64{
		"A": 20, // ID
		"B": 15, // Platform
		"C": 40, // Title
		"D": 30, // Author
		"E": 60, // URL
		"F": 15, // Duration
		"G": 15, // File Size
		"H": 20, // Published At
		"I": 20, // Downloaded At
		"J": 15, // Status
	}

	for col, width := range columnWidths {
		f.SetColWidth(sheetName, col, col, width)
	}

	// Write data rows
	for i, video := range videos {
		row := de.videoToRow(video)
		for j, value := range row {
			cell := fmt.Sprintf("%c%d", 'A'+j, i+2)
			f.SetCellValue(sheetName, cell, value)
		}
	}

	// Auto-filter
	endRange := fmt.Sprintf("%c%d", 'A'+len(de.config.Columns)-1, len(videos)+1)
	f.AutoFilter(sheetName, "A1:"+endRange, []excelize.AutoFilterOptions{})

	// Freeze first row
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze: true,
		Split:  false,
		XSplit: 0,
		YSplit: 1,
	})

	// Save file
	if err := f.SaveAs(de.config.FilePath); err != nil {
		return fmt.Errorf("failed to save XLSX file: %w", err)
	}

	return nil
}

// exportToJSON exports data to JSON format
func (de *DataExporter) exportToJSON(videos []*models.VideoInfo) error {
	// Create export data structure
	exportData := struct {
		ExportedAt time.Time           `json:"exported_at"`
		Count      int                 `json:"count"`
		Videos     []*models.VideoInfo `json:"videos"`
	}{
		ExportedAt: time.Now(),
		Count:      len(videos),
		Videos:     videos,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(de.config.FilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}

// exportToTXT exports data to plain text format
func (de *DataExporter) exportToTXT(videos []*models.VideoInfo) error {
	file, err := os.Create(de.config.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create TXT file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "Video Download Report\n")
	fmt.Fprintf(file, "Generated: %s\n", time.Now().Format(de.config.DateFormat))
	fmt.Fprintf(file, "Total Videos: %d\n", len(videos))
	fmt.Fprintf(file, "%s\n\n", strings.Repeat("=", 50))

	// Write video entries
	for i, video := range videos {
		fmt.Fprintf(file, "Video %d:\n", i+1)
		fmt.Fprintf(file, "  ID: %s\n", video.ID)
		fmt.Fprintf(file, "  Platform: %s\n", video.Platform)
		fmt.Fprintf(file, "  Title: %s\n", video.Title)
		fmt.Fprintf(file, "  Author: %s\n", video.AuthorName)
		fmt.Fprintf(file, "  URL: %s\n", video.URL)
		fmt.Fprintf(file, "  Duration: %d seconds\n", video.Duration)
		if video.Size > 0 {
			fmt.Fprintf(file, "  File Size: %s\n", formatBytes(video.Size))
		}
		fmt.Fprintf(file, "  Published: %s\n", video.PublishedAt.Format(de.config.DateFormat))
		if !video.DownloadedAt.IsZero() {
			fmt.Fprintf(file, "  Downloaded: %s\n", video.DownloadedAt.Format(de.config.DateFormat))
		}
		fmt.Fprintf(file, "  Status: %s\n", video.Status)
		fmt.Fprintf(file, "\n")
	}

	return nil
}

// videoToRow converts a VideoInfo to a row of strings
func (de *DataExporter) videoToRow(video *models.VideoInfo) []string {
	row := make([]string, len(de.config.Columns))

	for i, column := range de.config.Columns {
		switch strings.ToLower(column) {
		case "id", "video_id":
			row[i] = video.ID
		case "platform":
			row[i] = string(video.Platform)
		case "title":
			row[i] = video.Title
		case "description":
			row[i] = video.Description
		case "author", "author_name":
			row[i] = video.AuthorName
		case "author_id":
			row[i] = video.AuthorID
		case "url":
			row[i] = video.URL
		case "download_url":
			row[i] = video.DownloadURL
		case "thumbnail":
			row[i] = video.Thumbnail
		case "duration":
			row[i] = fmt.Sprintf("%d", video.Duration)
		case "media_type":
			row[i] = string(video.MediaType)
		case "size", "file_size":
			if video.Size > 0 {
				row[i] = fmt.Sprintf("%d", video.Size)
			} else {
				row[i] = ""
			}
		case "format":
			row[i] = video.Format
		case "quality":
			row[i] = video.Quality
		case "view_count":
			row[i] = fmt.Sprintf("%d", video.ViewCount)
		case "like_count":
			row[i] = fmt.Sprintf("%d", video.LikeCount)
		case "share_count":
			row[i] = fmt.Sprintf("%d", video.ShareCount)
		case "comment_count":
			row[i] = fmt.Sprintf("%d", video.CommentCount)
		case "published_at":
			row[i] = video.PublishedAt.Format(de.config.DateFormat)
		case "collected_at":
			row[i] = video.CollectedAt.Format(de.config.DateFormat)
		case "downloaded_at":
			if !video.DownloadedAt.IsZero() {
				row[i] = video.DownloadedAt.Format(de.config.DateFormat)
			} else {
				row[i] = ""
			}
		case "file_path":
			row[i] = video.FilePath
		case "download_path":
			row[i] = video.DownloadPath
		case "status":
			row[i] = video.Status
		case "retry_count":
			row[i] = fmt.Sprintf("%d", video.RetryCount)
		case "error_message":
			row[i] = video.ErrorMessage
		default:
			row[i] = ""
		}
	}

	return row
}

// getDefaultColumns returns default column names
func getDefaultColumns() []string {
	return []string{
		"ID",
		"Platform",
		"Title",
		"Author",
		"URL",
		"Duration",
		"File Size",
		"Published At",
		"Downloaded At",
		"Status",
	}
}

// formatBytes formats bytes to human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ExportAuthors exports author data
func (de *DataExporter) ExportAuthors(authors []*models.AuthorInfo) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(de.config.FilePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	switch de.config.Format {
	case FormatCSV:
		return de.exportAuthorsToCSV(authors)
	case FormatXLSX:
		return de.exportAuthorsToXLSX(authors)
	case FormatJSON:
		return de.exportAuthorsToJSON(authors)
	default:
		return fmt.Errorf("unsupported export format for authors: %s", de.config.Format)
	}
}

// exportAuthorsToCSV exports authors to CSV
func (de *DataExporter) exportAuthorsToCSV(authors []*models.AuthorInfo) error {
	file, err := os.Create(de.config.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = de.config.Delimiter
	defer writer.Flush()

	// Write header
	headers := []string{"ID", "Platform", "Name", "Nickname", "Avatar", "Description", "Followers", "Following", "Video Count", "Verified", "Collected At"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, author := range authors {
		row := []string{
			author.ID,
			string(author.Platform),
			author.Name,
			author.Nickname,
			author.Avatar,
			author.Description,
			fmt.Sprintf("%d", author.Followers),
			fmt.Sprintf("%d", author.Following),
			fmt.Sprintf("%d", author.VideoCount),
			fmt.Sprintf("%t", author.Verified),
			author.CollectedAt.Format(de.config.DateFormat),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// exportAuthorsToXLSX exports authors to Excel
func (de *DataExporter) exportAuthorsToXLSX(authors []*models.AuthorInfo) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Authors"
	f.SetSheetName("Sheet1", sheetName)

	// Headers
	headers := []string{"ID", "Platform", "Name", "Nickname", "Avatar", "Description", "Followers", "Following", "Video Count", "Verified", "Collected At"}

	// Write headers
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	// Write data
	for i, author := range authors {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), author.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), string(author.Platform))
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), author.Name)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), author.Nickname)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), author.Avatar)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), author.Description)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), author.Followers)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), author.Following)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), author.VideoCount)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), author.Verified)
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), author.CollectedAt.Format(de.config.DateFormat))
	}

	return f.SaveAs(de.config.FilePath)
}

// exportAuthorsToJSON exports authors to JSON
func (de *DataExporter) exportAuthorsToJSON(authors []*models.AuthorInfo) error {
	exportData := struct {
		ExportedAt time.Time            `json:"exported_at"`
		Count      int                  `json:"count"`
		Authors    []*models.AuthorInfo `json:"authors"`
	}{
		ExportedAt: time.Now(),
		Count:      len(authors),
		Authors:    authors,
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return os.WriteFile(de.config.FilePath, data, 0644)
}

// ExportTemplate creates a template file for bulk import
func (de *DataExporter) ExportTemplate() error {
	switch de.config.Format {
	case FormatCSV:
		return de.createCSVTemplate()
	case FormatXLSX:
		return de.createXLSXTemplate()
	default:
		return fmt.Errorf("template creation not supported for format: %s", de.config.Format)
	}
}

// createCSVTemplate creates a CSV template
func (de *DataExporter) createCSVTemplate() error {
	file, err := os.Create(de.config.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV template: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write headers with example data
	headers := []string{"URL", "Platform", "Quality", "Format", "Custom_Name"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write example rows
	examples := [][]string{
		{"https://www.tiktok.com/@user/video/123456", "tiktok", "hd", "mp4", ""},
		{"https://www.xiaohongshu.com/explore/abc123", "xhs", "hd", "mp4", "custom_filename"},
		{"https://www.kuaishou.com/short-video/def456", "kuaishou", "hd", "mp4", ""},
	}

	for _, example := range examples {
		if err := writer.Write(example); err != nil {
			return fmt.Errorf("failed to write example row: %w", err)
		}
	}

	return nil
}

// createXLSXTemplate creates an Excel template
func (de *DataExporter) createXLSXTemplate() error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Template"
	f.SetSheetName("Sheet1", sheetName)

	// Headers
	headers := []string{"URL", "Platform", "Quality", "Format", "Custom_Name"}

	// Set headers
	for i, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue(sheetName, cell, header)
	}

	// Example data
	examples := [][]string{
		{"https://www.tiktok.com/@user/video/123456", "tiktok", "hd", "mp4", ""},
		{"https://www.xiaohongshu.com/explore/abc123", "xhs", "hd", "mp4", "custom_filename"},
		{"https://www.kuaishou.com/short-video/def456", "kuaishou", "hd", "mp4", ""},
	}

	for i, example := range examples {
		row := i + 2
		for j, value := range example {
			cell := fmt.Sprintf("%c%d", 'A'+j, row)
			f.SetCellValue(sheetName, cell, value)
		}
	}

	// Add data validation for Platform column
	validation := excelize.DataValidation{
		Type:     "list",
		Sqref:    "B:B",
		Formula1: "tiktok,xhs,kuaishou",
	}
	f.AddDataValidation(sheetName, &validation)

	// Add data validation for Quality column
	qualityValidation := excelize.DataValidation{
		Type:     "list",
		Formula1: "hd,sd,auto",
		Sqref:    "C:C",
	}
	f.AddDataValidation(sheetName, &qualityValidation)

	return f.SaveAs(de.config.FilePath)
}

// GetSupportedFormats returns list of supported export formats
func GetSupportedFormats() []ExportFormat {
	return []ExportFormat{FormatCSV, FormatXLSX, FormatJSON, FormatTXT}
}

// ValidateConfig validates export configuration
func ValidateConfig(config ExportConfig) error {
	if config.FilePath == "" {
		return fmt.Errorf("file path is required")
	}

	supported := false
	for _, format := range GetSupportedFormats() {
		if config.Format == format {
			supported = true
			break
		}
	}

	if !supported {
		return fmt.Errorf("unsupported format: %s", config.Format)
	}

	return nil
}

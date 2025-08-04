package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"video-downloader/internal/config"
	"video-downloader/internal/downloader"
	"video-downloader/internal/server"
	"video-downloader/internal/storage"
	"video-downloader/internal/utils"
	"video-downloader/pkg/models"
)

var (
	configPath string
	outputPath string
	format     string
	quality    string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "video-downloader",
	Short: "A unified video downloader for TikTok, XHS, and Kuaishou",
	Long: `Video Downloader is a powerful tool to download videos from multiple platforms
including TikTok, Xiaohongshu (XHS), and Kuaishou.

Features:
- Support for multiple video platforms
- High-quality video downloads
- Batch downloading
- Progress tracking
- Customizable output formats
- Proxy support`,
	Version: "1.0.0",
}

var downloadCmd = &cobra.Command{
	Use:   "download [url]",
	Short: "Download a video from URL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		// Load configuration
		configManager := config.NewManager()
		cfg, err := configManager.Load(configPath)
		if err != nil {
			return fmt.Errorf("error loading configuration: %w", err)
		}

		// Initialize storage
		storage, err := storage.NewSQLite(cfg.Database.Path)
		if err != nil {
			return fmt.Errorf("error initializing storage: %w", err)
		}
		defer storage.Close()

		// Create download manager
		dm := downloader.NewManager(cfg, storage)
		if err := dm.Start(); err != nil {
			return fmt.Errorf("error starting download manager: %w", err)
		}
		defer dm.Stop()

		// Set up signal handling
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Download options
		options := &downloader.DownloadOptions{
			OutputPath: outputPath,
			Format:     format,
			Quality:    quality,
			Progress:   true,
		}

		// Start download
		fmt.Printf("Downloading video from: %s\n", url)
		result, err := dm.Download(url, options)
		if err != nil {
			return fmt.Errorf("error downloading video: %w", err)
		}

		if result.Success {
			fmt.Printf("✅ Download completed: %s\n", result.Video.FilePath)
			fmt.Printf("   Size: %s\n", utils.FormatBytes(result.Video.FileSize))
			fmt.Printf("   Duration: %s\n", utils.FormatDuration(time.Duration(result.Video.Duration)*time.Second))
		} else {
			fmt.Printf("❌ Download failed: %s\n", result.Error)
		}

		return nil
	},
}

var batchCmd = &cobra.Command{
	Use:   "batch [urls-file]",
	Short: "Download multiple videos from a file containing URLs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		urlsFile := args[0]

		// Read URLs from file
		urls, err := readURLsFromFile(urlsFile)
		if err != nil {
			return fmt.Errorf("error reading URLs file: %w", err)
		}

		if len(urls) == 0 {
			fmt.Println("No URLs found in file")
			return nil
		}

		fmt.Printf("Found %d URLs to download\n", len(urls))

		// Load configuration
		configManager := config.NewManager()
		cfg, err := configManager.Load(configPath)
		if err != nil {
			return fmt.Errorf("error loading configuration: %w", err)
		}

		// Initialize storage
		storage, err := storage.NewSQLite(cfg.Database.Path)
		if err != nil {
			return fmt.Errorf("error initializing storage: %w", err)
		}
		defer storage.Close()

		// Create download manager
		dm := downloader.NewManager(cfg, storage)
		if err := dm.Start(); err != nil {
			return fmt.Errorf("error starting download manager: %w", err)
		}
		defer dm.Stop()

		// Download options
		options := &downloader.DownloadOptions{
			OutputPath: outputPath,
			Format:     format,
			Quality:    quality,
			Progress:   true,
		}

		// Batch download
		results, err := dm.DownloadBatch(urls, options)
		if err != nil {
			fmt.Printf("⚠️  Some downloads failed: %v\n", err)
		}

		// Print results
		success := 0
		failed := 0
		for _, result := range results {
			if result != nil && result.Success {
				success++
				fmt.Printf("✅ %s\n", result.Video.FilePath)
			} else {
				failed++
				if result != nil {
					fmt.Printf("❌ Failed: %v\n", result.Error)
				}
			}
		}

		fmt.Printf("\nDownload summary: %d success, %d failed\n", success, failed)
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info [url]",
	Short: "Get video information without downloading",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		// Load configuration
		configManager := config.NewManager()
		cfg, err := configManager.Load(configPath)
		if err != nil {
			return fmt.Errorf("error loading configuration: %w", err)
		}

		// Initialize storage
		storage, err := storage.NewSQLite(cfg.Database.Path)
		if err != nil {
			return fmt.Errorf("error initializing storage: %w", err)
		}
		defer storage.Close()

		// Create download manager
		dm := downloader.NewManager(cfg, storage)
		if err := dm.Start(); err != nil {
			return fmt.Errorf("error starting download manager: %w", err)
		}
		defer dm.Stop()

		// Get video info
		videoInfo, err := dm.GetVideoInfo(url)
		if err != nil {
			return fmt.Errorf("error getting video info: %w", err)
		}

		if videoInfo == nil {
			fmt.Println("Video not found")
			return nil
		}

		// Print video information
		fmt.Printf("📹 Video Information\n")
		fmt.Printf("   Title: %s\n", videoInfo.Title)
		fmt.Printf("   Platform: %s\n", videoInfo.Platform)
		fmt.Printf("   Author: %s\n", videoInfo.AuthorName)
		fmt.Printf("   Duration: %s\n", utils.FormatDuration(time.Duration(videoInfo.Duration)*time.Second))
		fmt.Printf("   Views: %d\n", videoInfo.ViewCount)
		fmt.Printf("   Likes: %d\n", videoInfo.LikeCount)
		fmt.Printf("   Published: %s\n", videoInfo.PublishedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("   URL: %s\n", videoInfo.URL)

		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List downloaded videos",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		configManager := config.NewManager()
		cfg, err := configManager.Load(configPath)
		if err != nil {
			return fmt.Errorf("error loading configuration: %w", err)
		}

		// Initialize storage
		storage, err := storage.NewSQLite(cfg.Database.Path)
		if err != nil {
			return fmt.Errorf("error initializing storage: %w", err)
		}
		defer storage.Close()

		// List videos
		videos, err := storage.ListVideos(models.VideoFilter{
			Limit:     50,
			OrderBy:   "collected_at",
			OrderDesc: true,
		})
		if err != nil {
			return fmt.Errorf("error listing videos: %w", err)
		}

		if len(videos) == 0 {
			fmt.Println("No videos found")
			return nil
		}

		// Print videos
		fmt.Printf("📚 Downloaded Videos (%d)\n", len(videos))
		for i, video := range videos {
			fmt.Printf("\n%d. %s\n", i+1, video.Title)
			fmt.Printf("   Platform: %s | Author: %s\n", video.Platform, video.AuthorName)
			fmt.Printf("   Status: %s | Size: %s\n", video.Status, utils.FormatBytes(video.FileSize))
			if video.DownloadedAt != nil {
				fmt.Printf("   Downloaded: %s\n", video.DownloadedAt.Format("2006-01-02 15:04:05"))
			}
		}

		return nil
	},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		configManager := config.NewManager()
		cfg, err := configManager.Load(configPath)
		if err != nil {
			return fmt.Errorf("error loading configuration: %w", err)
		}

		// Initialize storage
		storage, err := storage.NewSQLite(cfg.Database.Path)
		if err != nil {
			return fmt.Errorf("error initializing storage: %w", err)
		}
		defer storage.Close()

		// Create and start server
		srv := server.NewServer(cfg, storage)
		if err := srv.Run(); err != nil {
			return fmt.Errorf("error running server: %w", err)
		}

		fmt.Printf("🚀 Server started on http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
		fmt.Println("Press Ctrl+C to stop the server")

		// The server handles its own signal internally
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
}

var initConfigCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		configManager := config.NewManager()
		_, err := configManager.Load(configPath)
		if err != nil {
			return fmt.Errorf("error initializing configuration: %w", err)
		}
		fmt.Println("Configuration file created successfully")
		return nil
	},
}

var showConfigCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		configManager := config.NewManager()
		cfg, err := configManager.Load(configPath)
		if err != nil {
			return fmt.Errorf("error loading configuration: %w", err)
		}

		fmt.Printf("📋 Current Configuration\n")
		fmt.Printf("   Server Host: %s\n", cfg.Server.Host)
		fmt.Printf("   Server Port: %d\n", cfg.Server.Port)
		fmt.Printf("   Download Path: %s\n", cfg.Download.SavePath)
		fmt.Printf("   Max Workers: %d\n", cfg.Download.MaxWorkers)
		fmt.Printf("   Database Path: %s\n", cfg.Database.Path)
		fmt.Printf("   Log Level: %s\n", cfg.Log.Level)
		fmt.Printf("   Proxy Enabled: %v\n", cfg.Proxy.Enabled)

		return nil
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().StringVarP(&outputPath, "output", "o", "", "Output directory")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "", "Output format (mp4, mp3, etc.)")
	rootCmd.PersistentFlags().StringVarP(&quality, "quality", "q", "", "Video quality (hd, sd, etc.)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Add commands
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(batchCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(configCmd)

	// Config subcommands
	configCmd.AddCommand(initConfigCmd)
	configCmd.AddCommand(showConfigCmd)
}

func readURLsFromFile(filename string) ([]string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Split by lines and filter empty lines
	lines := strings.Split(string(content), "\n")
	var urls []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}

	return urls, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

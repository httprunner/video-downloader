package registry

import (
	"fmt"
	"regexp"
	"strings"
	
	"video-downloader/internal/platform"
	"video-downloader/pkg/models"
)

// Registry manages platform extractors and provides dynamic selection
type Registry struct {
	extractors map[models.Platform]models.PlatformExtractor
	patterns   map[string]models.Platform
	logger     interface{} // Could be logrus.Logger or any logger interface
}

// NewRegistry creates a new platform registry
func NewRegistry() *Registry {
	return &Registry{
		extractors: make(map[models.Platform]models.PlatformExtractor),
		patterns:   make(map[string]models.Platform),
	}
}

// RegisterExtractor registers a platform extractor
func (r *Registry) RegisterExtractor(platform models.Platform, extractor models.PlatformExtractor, patterns []string) error {
	if extractor == nil {
		return fmt.Errorf("extractor cannot be nil")
	}
	
	// Register the extractor
	r.extractors[platform] = extractor
	
	// Register URL patterns for this platform
	for _, pattern := range patterns {
		r.patterns[pattern] = platform
	}
	
	return nil
}

// RegisterDefaultPlatforms registers all supported platforms with default configurations
func (r *Registry) RegisterDefaultPlatforms(config *models.Config) error {
	// Register TikTok
	if config.Platforms.TikTok.Enabled {
		tiktokExtractor := platform.NewTikTokExtractor(&models.ExtractorConfig{
			Timeout:    0, // Will be set by the extractor
			Proxy:      "",
			UserAgent:  config.Platforms.TikTok.UserAgent,
			Cookie:     config.Platforms.TikTok.Cookie,
			MaxRetries: 3,
		})
		
		tiktokPatterns := []string{
			`https?://(?:www\.)?tiktok\.com/@[^/]+/video/\d+`,
			`https?://(?:www\.)?tiktok\.com/t/\w+`,
			`https?://vm\.tiktok\.com/\w+`,
			`https?://(?:www\.)?tiktok\.com/@[^/]+`,
		}
		
		if err := r.RegisterExtractor(models.PlatformTikTok, tiktokExtractor, tiktokPatterns); err != nil {
			return fmt.Errorf("error registering TikTok extractor: %w", err)
		}
	}
	
	// Register XHS
	if config.Platforms.XHS.Enabled {
		xhsExtractor := platform.NewXHSExtractor(&models.ExtractorConfig{
			Timeout:    0,
			Proxy:      "",
			UserAgent:  config.Platforms.XHS.UserAgent,
			Cookie:     config.Platforms.XHS.Cookie,
			MaxRetries: 3,
		})
		
		xhsPatterns := []string{
			`https?://(?:www\.)?xiaohongshu\.com/explore/[^/]+`,
			`https?://(?:www\.)?xiaohongshu\.com/discovery/item/[^/]+`,
			`https?://(?:www\.)?xiaohongshu\.com/user/profile/[^/]+`,
			`https?://xhslink\.com/[^/]+`,
		}
		
		if err := r.RegisterExtractor(models.PlatformXHS, xhsExtractor, xhsPatterns); err != nil {
			return fmt.Errorf("error registering XHS extractor: %w", err)
		}
	}
	
	// Register Kuaishou
	if config.Platforms.Kuaishou.Enabled {
		kuaishouExtractor := platform.NewKuaishouExtractor(&models.ExtractorConfig{
			Timeout:    0,
			Proxy:      "",
			UserAgent:  config.Platforms.Kuaishou.UserAgent,
			Cookie:     config.Platforms.Kuaishou.Cookie,
			MaxRetries: 3,
		})
		
		kuaishouPatterns := []string{
			`https?://(?:www\.)?kuaishou\.com/short-video/[^/]+`,
			`https?://(?:www\.)?kuaishou\.com/profile/[^/]+`,
			`https?://v\.kuaishou\.com/[^/]+`,
		}
		
		if err := r.RegisterExtractor(models.PlatformKuaishou, kuaishouExtractor, kuaishouPatterns); err != nil {
			return fmt.Errorf("error registering Kuaishou extractor: %w", err)
		}
	}
	
	return nil
}

// GetExtractor returns an extractor for the given platform
func (r *Registry) GetExtractor(platform models.Platform) (models.PlatformExtractor, error) {
	extractor, exists := r.extractors[platform]
	if !exists {
		return nil, fmt.Errorf("no extractor registered for platform: %s", platform)
	}
	
	return extractor, nil
}

// GetExtractorForURL returns an extractor for the given URL
func (r *Registry) GetExtractorForURL(url string) (models.PlatformExtractor, models.Platform, error) {
	platform, err := r.DetectPlatform(url)
	if err != nil {
		return nil, "", err
	}
	
	extractor, err := r.GetExtractor(platform)
	if err != nil {
		return nil, "", err
	}
	
	return extractor, platform, nil
}

// DetectPlatform detects the platform from URL
func (r *Registry) DetectPlatform(url string) (models.Platform, error) {
	// Try to match against registered patterns
	for pattern, platform := range r.patterns {
		if matched, _ := regexp.MatchString(pattern, url); matched {
			return platform, nil
		}
	}
	
	// Fallback to domain-based detection
	return r.detectPlatformByDomain(url)
}

// detectPlatformByDomain detects platform by domain name
func (r *Registry) detectPlatformByDomain(url string) (models.Platform, error) {
	lowerURL := strings.ToLower(url)
	
	switch {
	case strings.Contains(lowerURL, "tiktok.com"):
		return models.PlatformTikTok, nil
	case strings.Contains(lowerURL, "xiaohongshu.com") || strings.Contains(lowerURL, "xhslink.com"):
		return models.PlatformXHS, nil
	case strings.Contains(lowerURL, "kuaishou.com"):
		return models.PlatformKuaishou, nil
	default:
		return "", fmt.Errorf("unsupported platform for URL: %s", url)
	}
}

// ListPlatforms returns all registered platforms
func (r *Registry) ListPlatforms() []models.Platform {
	platforms := make([]models.Platform, 0, len(r.extractors))
	for platform := range r.extractors {
		platforms = append(platforms, platform)
	}
	return platforms
}

// IsPlatformSupported checks if a platform is supported
func (r *Registry) IsPlatformSupported(platform models.Platform) bool {
	_, exists := r.extractors[platform]
	return exists
}

// GetPlatformPatterns returns all URL patterns for a platform
func (r *Registry) GetPlatformPatterns(platform models.Platform) []string {
	var patterns []string
	for pattern, p := range r.patterns {
		if p == platform {
			patterns = append(patterns, pattern)
		}
	}
	return patterns
}

// GetAllPatterns returns all registered URL patterns
func (r *Registry) GetAllPatterns() map[string]models.Platform {
	// Return a copy to prevent external modification
	result := make(map[string]models.Platform)
	for pattern, platform := range r.patterns {
		result[pattern] = platform
	}
	return result
}

// ValidateURL validates if the URL is supported by any registered extractor
func (r *Registry) ValidateURL(url string) bool {
	_, err := r.DetectPlatform(url)
	return err == nil
}

// GetExtractorCount returns the number of registered extractors
func (r *Registry) GetExtractorCount() int {
	return len(r.extractors)
}

// Clear clears all registered extractors and patterns
func (r *Registry) Clear() {
	r.extractors = make(map[models.Platform]models.PlatformExtractor)
	r.patterns = make(map[string]models.Platform)
}

// UpdateExtractorConfig updates the configuration for a platform's extractor
func (r *Registry) UpdateExtractorConfig(platform models.Platform, config *models.ExtractorConfig) error {
	_, exists := r.extractors[platform]
	if !exists {
		return fmt.Errorf("no extractor registered for platform: %s", platform)
	}
	
	// Note: This is a simplified approach. In a real implementation,
	// you might need to recreate the extractor with the new config
	// or provide a method to update the config on the existing extractor.
	
	// For now, we'll log that config update is not implemented
	if r.logger != nil {
		// This would be: r.logger.Debugf("Config update for platform %s not implemented", platform)
	}
	
	return nil
}

// SetLogger sets the logger for the registry
func (r *Registry) SetLogger(logger interface{}) {
	r.logger = logger
}

// Factory functions for creating extractors with different configurations

// CreateTikTokExtractor creates a TikTok extractor with custom configuration
func CreateTikTokExtractor(config *models.ExtractorConfig) models.PlatformExtractor {
	return platform.NewTikTokExtractor(config)
}

// CreateXHSExtractor creates an XHS extractor with custom configuration
func CreateXHSExtractor(config *models.ExtractorConfig) models.PlatformExtractor {
	return platform.NewXHSExtractor(config)
}

// CreateKuaishouExtractor creates a Kuaishou extractor with custom configuration
func CreateKuaishouExtractor(config *models.ExtractorConfig) models.PlatformExtractor {
	return platform.NewKuaishouExtractor(config)
}

// PlatformInfo contains information about a registered platform
type PlatformInfo struct {
	Name        models.Platform
	Enabled      bool
	Patterns    []string
	Description string
}

// GetPlatformInfo returns information about all registered platforms
func (r *Registry) GetPlatformInfo() []PlatformInfo {
	var info []PlatformInfo
	
	for platform := range r.extractors {
		patterns := r.GetPlatformPatterns(platform)
		
		platformInfo := PlatformInfo{
			Name:     platform,
			Enabled:  true,
			Patterns: patterns,
		}
		
		// Add description based on platform
		switch platform {
		case models.PlatformTikTok:
			platformInfo.Description = "TikTok video downloader"
		case models.PlatformXHS:
			platformInfo.Description = "Xiaohongshu (XHS) content downloader"
		case models.PlatformKuaishou:
			platformInfo.Description = "Kuaishou video downloader"
		default:
			platformInfo.Description = "Unknown platform"
		}
		
		info = append(info, platformInfo)
	}
	
	return info
}
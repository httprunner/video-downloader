package platform

import (
	"video-downloader/pkg/models"
	"video-downloader/internal/platform/tiktok"
	"video-downloader/internal/platform/xhs"
	"video-downloader/internal/platform/kuaishou"
)

// NewTikTokExtractor creates a new TikTok extractor
func NewTikTokExtractor(config *models.ExtractorConfig) models.PlatformExtractor {
	return tiktok.NewExtractor(config)
}

// NewXHSExtractor creates a new XHS extractor
func NewXHSExtractor(config *models.ExtractorConfig) models.PlatformExtractor {
	return xhs.NewExtractor(config)
}

// NewKuaishouExtractor creates a new Kuaishou extractor
func NewKuaishouExtractor(config *models.ExtractorConfig) models.PlatformExtractor {
	return kuaishou.NewExtractor(config)
}
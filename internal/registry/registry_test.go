package registry

import (
	"testing"

	"video-downloader/pkg/models"
)

// MockExtractor implements the PlatformExtractor interface for testing
type MockExtractor struct {
	platform models.Platform
}

func (m *MockExtractor) ExtractVideoInfo(url string) (*models.VideoInfo, error) {
	return &models.VideoInfo{
		ID:        "test_id",
		Platform:  m.platform,
		Title:     "Test Video",
		URL:       url,
		Status:    "pending",
		MediaType: models.MediaTypeVideo,
	}, nil
}

func (m *MockExtractor) ExtractAuthorInfo(authorID string) (*models.AuthorInfo, error) {
	return &models.AuthorInfo{
		ID:       authorID,
		Platform: m.platform,
		Name:     authorID,
	}, nil
}

func (m *MockExtractor) ExtractBatch(url string, limit int) ([]*models.VideoInfo, error) {
	return []*models.VideoInfo{
		{
			ID:        "batch_id_1",
			Platform:  m.platform,
			Title:     "Batch Video 1",
			URL:       url,
			Status:    "pending",
			MediaType: models.MediaTypeVideo,
		},
	}, nil
}

func (m *MockExtractor) ValidateURL(url string) bool {
	return true
}

func (m *MockExtractor) GetName() models.Platform {
	return m.platform
}

func (m *MockExtractor) GetSupportedURLPatterns() []string {
	return []string{
		"https?://example\\.com/.*",
	}
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Error("Expected registry to be created, got nil")
	}

	if len(registry.extractors) != 0 {
		t.Errorf("Expected empty extractors map, got %d extractors", len(registry.extractors))
	}

	if len(registry.patterns) != 0 {
		t.Errorf("Expected empty patterns map, got %d patterns", len(registry.patterns))
	}
}

func TestRegisterExtractor(t *testing.T) {
	registry := NewRegistry()

	mockExtractor := &MockExtractor{platform: models.PlatformTikTok}
	patterns := []string{
		"https?://tiktok\\.com/.*",
		"https?://vm\\.tiktok\\.com/.*",
	}

	err := registry.RegisterExtractor(models.PlatformTikTok, mockExtractor, patterns)
	if err != nil {
		t.Errorf("Expected no error registering extractor, got %v", err)
	}

	// Verify extractor was registered
	extractor, err := registry.GetExtractor(models.PlatformTikTok)
	if err != nil {
		t.Errorf("Expected to get registered extractor, got error: %v", err)
	}

	if extractor == nil {
		t.Error("Expected extractor, got nil")
	}

	// Verify patterns were registered
	if len(registry.patterns) != len(patterns) {
		t.Errorf("Expected %d patterns, got %d", len(patterns), len(registry.patterns))
	}
}

func TestRegisterExtractorWithNilExtractor(t *testing.T) {
	registry := NewRegistry()

	err := registry.RegisterExtractor(models.PlatformTikTok, nil, []string{"https?://tiktok\\.com/.*"})
	if err == nil {
		t.Error("Expected error when registering nil extractor, got nil")
	}
}

func TestGetExtractorForNonExistentPlatform(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.GetExtractor(models.PlatformTikTok)
	if err == nil {
		t.Error("Expected error getting non-existent extractor, got nil")
	}
}

func TestDetectPlatform(t *testing.T) {
	registry := NewRegistry()

	// Register some extractors with patterns
	tikTokExtractor := &MockExtractor{platform: models.PlatformTikTok}
	tikTokPatterns := []string{
		`https?://(?:www\.)?tiktok\.com/@[^/]+/video/\d+`,
		`https?://vm\.tiktok\.com/\w+`,
	}

	xhsExtractor := &MockExtractor{platform: models.PlatformXHS}
	xhsPatterns := []string{
		`https?://(?:www\.)?xiaohongshu\.com/explore/[^/]+`,
		`https?://xhslink\.com/[^/]+`,
	}

	registry.RegisterExtractor(models.PlatformTikTok, tikTokExtractor, tikTokPatterns)
	registry.RegisterExtractor(models.PlatformXHS, xhsExtractor, xhsPatterns)

	tests := []struct {
		url      string
		expected models.Platform
		err      bool
	}{
		{
			url:      "https://www.tiktok.com/@user/video/1234567890",
			expected: models.PlatformTikTok,
			err:      false,
		},
		{
			url:      "https://vm.tiktok.com/XYZ123",
			expected: models.PlatformTikTok,
			err:      false,
		},
		{
			url:      "https://www.xiaohongshu.com/explore/abcdef",
			expected: models.PlatformXHS,
			err:      false,
		},
		{
			url:      "https://xhslink.com/abcdef",
			expected: models.PlatformXHS,
			err:      false,
		},
		{
			url:      "https://example.com/video",
			expected: "",
			err:      true,
		},
	}

	for _, test := range tests {
		platform, err := registry.DetectPlatform(test.url)

		if test.err {
			if err == nil {
				t.Errorf("Expected error for URL %s, got nil", test.url)
			}
		} else {
			if err != nil {
				t.Errorf("Expected no error for URL %s, got %v", test.url, err)
			}
			if platform != test.expected {
				t.Errorf("Expected platform %s for URL %s, got %s", test.expected, test.url, platform)
			}
		}
	}
}

func TestDetectPlatformByDomain(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		url      string
		expected models.Platform
		err      bool
	}{
		{
			url:      "https://www.tiktok.com/@user/video/123",
			expected: models.PlatformTikTok,
			err:      false,
		},
		{
			url:      "https://www.xiaohongshu.com/explore/abc",
			expected: models.PlatformXHS,
			err:      false,
		},
		{
			url:      "https://www.kuaishou.com/short-video/abc",
			expected: models.PlatformKuaishou,
			err:      false,
		},
		{
			url:      "https://example.com/video",
			expected: "",
			err:      true,
		},
	}

	for _, test := range tests {
		platform, err := registry.detectPlatformByDomain(test.url)

		if test.err {
			if err == nil {
				t.Errorf("Expected error for URL %s, got nil", test.url)
			}
		} else {
			if err != nil {
				t.Errorf("Expected no error for URL %s, got %v", test.url, err)
			}
			if platform != test.expected {
				t.Errorf("Expected platform %s for URL %s, got %s", test.expected, test.url, platform)
			}
		}
	}
}

func TestGetExtractorForURL(t *testing.T) {
	registry := NewRegistry()

	mockExtractor := &MockExtractor{platform: models.PlatformTikTok}
	patterns := []string{`https?://tiktok\.com/.*`}

	registry.RegisterExtractor(models.PlatformTikTok, mockExtractor, patterns)

	extractor, platform, err := registry.GetExtractorForURL("https://tiktok.com/video/123")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if platform != models.PlatformTikTok {
		t.Errorf("Expected platform TikTok, got %s", platform)
	}

	if extractor == nil {
		t.Error("Expected extractor, got nil")
	}
}

func TestValidateURL(t *testing.T) {
	registry := NewRegistry()

	mockExtractor := &MockExtractor{platform: models.PlatformTikTok}
	patterns := []string{`https?://tiktok\.com/.*`}

	registry.RegisterExtractor(models.PlatformTikTok, mockExtractor, patterns)

	tests := []struct {
		url   string
		valid bool
	}{
		{
			url:   "https://tiktok.com/video/123",
			valid: true,
		},
		{
			url:   "https://example.com/video",
			valid: false,
		},
	}

	for _, test := range tests {
		valid := registry.ValidateURL(test.url)
		if valid != test.valid {
			t.Errorf("Expected URL %s to be valid %t, got %t", test.url, test.valid, valid)
		}
	}
}

func TestListPlatforms(t *testing.T) {
	registry := NewRegistry()

	mockExtractor1 := &MockExtractor{platform: models.PlatformTikTok}
	mockExtractor2 := &MockExtractor{platform: models.PlatformXHS}

	registry.RegisterExtractor(models.PlatformTikTok, mockExtractor1, []string{"tiktok"})
	registry.RegisterExtractor(models.PlatformXHS, mockExtractor2, []string{"xhs"})

	platforms := registry.ListPlatforms()

	if len(platforms) != 2 {
		t.Errorf("Expected 2 platforms, got %d", len(platforms))
	}

	// Check that both platforms are present
	foundTikTok := false
	foundXHS := false

	for _, platform := range platforms {
		if platform == models.PlatformTikTok {
			foundTikTok = true
		}
		if platform == models.PlatformXHS {
			foundXHS = true
		}
	}

	if !foundTikTok {
		t.Error("Expected to find TikTok platform")
	}

	if !foundXHS {
		t.Error("Expected to find XHS platform")
	}
}

func TestIsPlatformSupported(t *testing.T) {
	registry := NewRegistry()

	mockExtractor := &MockExtractor{platform: models.PlatformTikTok}
	registry.RegisterExtractor(models.PlatformTikTok, mockExtractor, []string{"tiktok"})

	if !registry.IsPlatformSupported(models.PlatformTikTok) {
		t.Error("Expected TikTok to be supported")
	}

	if registry.IsPlatformSupported(models.PlatformXHS) {
		t.Error("Expected XHS not to be supported")
	}
}

func TestGetPlatformPatterns(t *testing.T) {
	registry := NewRegistry()

	mockExtractor := &MockExtractor{platform: models.PlatformTikTok}
	patterns := []string{
		`https?://tiktok\.com/.*`,
		`https?://vm\.tiktok\.com/.*`,
	}

	registry.RegisterExtractor(models.PlatformTikTok, mockExtractor, patterns)

	retrievedPatterns := registry.GetPlatformPatterns(models.PlatformTikTok)

	if len(retrievedPatterns) != len(patterns) {
		t.Errorf("Expected %d patterns, got %d", len(patterns), len(retrievedPatterns))
	}
}

func TestClear(t *testing.T) {
	registry := NewRegistry()

	mockExtractor := &MockExtractor{platform: models.PlatformTikTok}
	registry.RegisterExtractor(models.PlatformTikTok, mockExtractor, []string{"tiktok"})

	if len(registry.extractors) != 1 {
		t.Errorf("Expected 1 extractor before clear, got %d", len(registry.extractors))
	}

	registry.Clear()

	if len(registry.extractors) != 0 {
		t.Errorf("Expected 0 extractors after clear, got %d", len(registry.extractors))
	}

	if len(registry.patterns) != 0 {
		t.Errorf("Expected 0 patterns after clear, got %d", len(registry.patterns))
	}
}

func TestGetExtractorCount(t *testing.T) {
	registry := NewRegistry()

	if registry.GetExtractorCount() != 0 {
		t.Errorf("Expected 0 extractors initially, got %d", registry.GetExtractorCount())
	}

	mockExtractor := &MockExtractor{platform: models.PlatformTikTok}
	registry.RegisterExtractor(models.PlatformTikTok, mockExtractor, []string{"tiktok"})

	if registry.GetExtractorCount() != 1 {
		t.Errorf("Expected 1 extractor after registration, got %d", registry.GetExtractorCount())
	}
}

func TestGetPlatformInfo(t *testing.T) {
	registry := NewRegistry()

	mockExtractor1 := &MockExtractor{platform: models.PlatformTikTok}
	mockExtractor2 := &MockExtractor{platform: models.PlatformXHS}

	registry.RegisterExtractor(models.PlatformTikTok, mockExtractor1, []string{"tiktok"})
	registry.RegisterExtractor(models.PlatformXHS, mockExtractor2, []string{"xhs"})

	info := registry.GetPlatformInfo()

	if len(info) != 2 {
		t.Errorf("Expected 2 platform info entries, got %d", len(info))
	}

	// Check that we have info for both platforms
	foundTikTok := false
	foundXHS := false

	for _, pi := range info {
		if pi.Name == models.PlatformTikTok {
			foundTikTok = true
			if !pi.Enabled {
				t.Error("Expected TikTok to be enabled")
			}
			if pi.Description != "TikTok video downloader" {
				t.Errorf("Expected TikTok description, got %s", pi.Description)
			}
		}
		if pi.Name == models.PlatformXHS {
			foundXHS = true
			if !pi.Enabled {
				t.Error("Expected XHS to be enabled")
			}
			if pi.Description != "Xiaohongshu (XHS) content downloader" {
				t.Errorf("Expected XHS description, got %s", pi.Description)
			}
		}
	}

	if !foundTikTok {
		t.Error("Expected to find TikTok platform info")
	}

	if !foundXHS {
		t.Error("Expected to find XHS platform info")
	}
}

package cookie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// CookieManager manages browser cookies for different platforms
type CookieManager struct {
	logger zerolog.Logger
	cache  map[string]*CookieCache
}

// CookieCache stores cached cookies for a platform
type CookieCache struct {
	Platform  string
	Cookies   []*http.Cookie
	UpdatedAt time.Time
	ExpiresAt time.Time
}

// BrowserType represents different browser types
type BrowserType string

const (
	BrowserChrome  BrowserType = "chrome"
	BrowserFirefox BrowserType = "firefox"
	BrowserSafari  BrowserType = "safari"
	BrowserEdge    BrowserType = "edge"
	BrowserOpera   BrowserType = "opera"
	BrowserBrave   BrowserType = "brave"
)

// PlatformDomain maps platforms to their domains
var PlatformDomains = map[string][]string{
	"tiktok":   {"tiktok.com", "www.tiktok.com"},
	"xhs":      {"xiaohongshu.com", "www.xiaohongshu.com"},
	"kuaishou": {"kuaishou.com", "www.kuaishou.com"},
}

// NewCookieManager creates a new cookie manager
func NewCookieManager() *CookieManager {
	return &CookieManager{
		logger: zerolog.New(nil).With().Str("component", "cookie_manager").Logger(),
		cache:  make(map[string]*CookieCache),
	}
}

// GetCookiesForPlatform returns cookies for a specific platform
func (cm *CookieManager) GetCookiesForPlatform(platform string) ([]*http.Cookie, error) {
	// Check cache first
	if cached, exists := cm.cache[platform]; exists && time.Now().Before(cached.ExpiresAt) {
		return cached.Cookies, nil
	}

	// Extract cookies from browsers
	cookies, err := cm.extractCookiesFromBrowsers(platform)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cookies for %s: %w", platform, err)
	}

	// Cache the result
	cm.cache[platform] = &CookieCache{
		Platform:  platform,
		Cookies:   cookies,
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(30 * time.Minute), // Cache for 30 minutes
	}

	return cookies, nil
}

// GetCookieStringForPlatform returns cookie string for a platform
func (cm *CookieManager) GetCookieStringForPlatform(platform string) (string, error) {
	cookies, err := cm.GetCookiesForPlatform(platform)
	if err != nil {
		return "", err
	}

	var cookieStrings []string
	for _, cookie := range cookies {
		cookieStrings = append(cookieStrings, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}

	return strings.Join(cookieStrings, "; "), nil
}

// extractCookiesFromBrowsers extracts cookies from all available browsers
func (cm *CookieManager) extractCookiesFromBrowsers(platform string) ([]*http.Cookie, error) {
	domains, exists := PlatformDomains[platform]
	if !exists {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	var allCookies []*http.Cookie

	// Try different browsers
	browsers := []BrowserType{BrowserChrome, BrowserFirefox, BrowserSafari, BrowserEdge}

	for _, browser := range browsers {
		for _, domain := range domains {
			cookies, err := cm.extractCookiesFromBrowser(browser, domain)
			if err != nil {
				cm.logger.Debug().Err(err).Str("browser", string(browser)).Str("domain", domain).Msg("Failed to extract cookies")
				continue
			}
			allCookies = append(allCookies, cookies...)
		}
	}

	return cm.deduplicateCookies(allCookies), nil
}

// extractCookiesFromBrowser extracts cookies from a specific browser
func (cm *CookieManager) extractCookiesFromBrowser(browser BrowserType, domain string) ([]*http.Cookie, error) {
	switch runtime.GOOS {
	case "windows":
		return cm.extractCookiesWindows(browser, domain)
	case "darwin":
		return cm.extractCookiesMacOS(browser, domain)
	case "linux":
		return cm.extractCookiesLinux(browser, domain)
	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// extractCookiesWindows extracts cookies on Windows
func (cm *CookieManager) extractCookiesWindows(browser BrowserType, domain string) ([]*http.Cookie, error) {
	var cookiePath string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	switch browser {
	case BrowserChrome:
		cookiePath = filepath.Join(homeDir, "AppData", "Local", "Google", "Chrome", "User Data", "Default", "Cookies")
	case BrowserEdge:
		cookiePath = filepath.Join(homeDir, "AppData", "Local", "Microsoft", "Edge", "User Data", "Default", "Cookies")
	case BrowserFirefox:
		return cm.extractFirefoxCookiesWindows(domain)
	default:
		return nil, fmt.Errorf("unsupported browser on Windows: %s", browser)
	}

	return cm.extractChromiumCookies(cookiePath, domain)
}

// extractCookiesMacOS extracts cookies on macOS
func (cm *CookieManager) extractCookiesMacOS(browser BrowserType, domain string) ([]*http.Cookie, error) {
	var cookiePath string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	switch browser {
	case BrowserChrome:
		cookiePath = filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome", "Default", "Cookies")
	case BrowserSafari:
		cookiePath = filepath.Join(homeDir, "Library", "Cookies", "Cookies.binarycookies")
		return cm.extractSafariCookies(cookiePath, domain)
	case BrowserFirefox:
		return cm.extractFirefoxCookiesMacOS(domain)
	default:
		return nil, fmt.Errorf("unsupported browser on macOS: %s", browser)
	}

	return cm.extractChromiumCookies(cookiePath, domain)
}

// extractCookiesLinux extracts cookies on Linux
func (cm *CookieManager) extractCookiesLinux(browser BrowserType, domain string) ([]*http.Cookie, error) {
	var cookiePath string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	switch browser {
	case BrowserChrome:
		cookiePath = filepath.Join(homeDir, ".config", "google-chrome", "Default", "Cookies")
	case BrowserFirefox:
		return cm.extractFirefoxCookiesLinux(domain)
	default:
		return nil, fmt.Errorf("unsupported browser on Linux: %s", browser)
	}

	return cm.extractChromiumCookies(cookiePath, domain)
}

// extractChromiumCookies extracts cookies from Chromium-based browsers
func (cm *CookieManager) extractChromiumCookies(cookiePath, domain string) ([]*http.Cookie, error) {
	// Check if cookie file exists
	if _, err := os.Stat(cookiePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cookie file not found: %s", cookiePath)
	}

	// In a real implementation, you would:
	// 1. Open the SQLite database
	// 2. Query cookies for the domain
	// 3. Decrypt the cookie values (Chrome encrypts cookies)
	// 4. Convert to http.Cookie format

	// For now, return empty slice as this requires platform-specific decryption
	cm.logger.Debug().Str("path", cookiePath).Str("domain", domain).Msg("Chromium cookie extraction not fully implemented")
	return []*http.Cookie{}, nil
}

// extractSafariCookies extracts cookies from Safari
func (cm *CookieManager) extractSafariCookies(cookiePath, domain string) ([]*http.Cookie, error) {
	// Safari uses binary cookie format
	// This would require parsing the binary format
	cm.logger.Debug().Str("path", cookiePath).Str("domain", domain).Msg("Safari cookie extraction not implemented")
	return []*http.Cookie{}, nil
}

// extractFirefoxCookiesWindows extracts Firefox cookies on Windows
func (cm *CookieManager) extractFirefoxCookiesWindows(domain string) ([]*http.Cookie, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	profilesPath := filepath.Join(homeDir, "AppData", "Roaming", "Mozilla", "Firefox", "Profiles")
	return cm.extractFirefoxCookies(profilesPath, domain)
}

// extractFirefoxCookiesMacOS extracts Firefox cookies on macOS
func (cm *CookieManager) extractFirefoxCookiesMacOS(domain string) ([]*http.Cookie, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	profilesPath := filepath.Join(homeDir, "Library", "Application Support", "Firefox", "Profiles")
	return cm.extractFirefoxCookies(profilesPath, domain)
}

// extractFirefoxCookiesLinux extracts Firefox cookies on Linux
func (cm *CookieManager) extractFirefoxCookiesLinux(domain string) ([]*http.Cookie, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	profilesPath := filepath.Join(homeDir, ".mozilla", "firefox")
	return cm.extractFirefoxCookies(profilesPath, domain)
}

// extractFirefoxCookies extracts cookies from Firefox profiles
func (cm *CookieManager) extractFirefoxCookies(profilesPath, domain string) ([]*http.Cookie, error) {
	// Firefox stores cookies in cookies.sqlite
	// This would require SQLite access to extract cookies
	cm.logger.Debug().Str("path", profilesPath).Str("domain", domain).Msg("Firefox cookie extraction not fully implemented")
	return []*http.Cookie{}, nil
}

// deduplicateCookies removes duplicate cookies
func (cm *CookieManager) deduplicateCookies(cookies []*http.Cookie) []*http.Cookie {
	seen := make(map[string]*http.Cookie)

	for _, cookie := range cookies {
		key := fmt.Sprintf("%s_%s_%s", cookie.Name, cookie.Domain, cookie.Path)
		if existing, exists := seen[key]; !exists || cookie.Expires.After(existing.Expires) {
			seen[key] = cookie
		}
	}

	var result []*http.Cookie
	for _, cookie := range seen {
		result = append(result, cookie)
	}

	return result
}

// SetCookiesFromString sets cookies from a cookie string
func (cm *CookieManager) SetCookiesFromString(platform, cookieString string) error {
	if cookieString == "" {
		return fmt.Errorf("empty cookie string")
	}

	var cookies []*http.Cookie

	// Parse cookie string
	pairs := strings.Split(cookieString, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		cookie := &http.Cookie{
			Name:  strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
		}

		// Set domain based on platform
		if domains, exists := PlatformDomains[platform]; exists && len(domains) > 0 {
			cookie.Domain = domains[0]
		}

		cookies = append(cookies, cookie)
	}

	// Cache the cookies
	cm.cache[platform] = &CookieCache{
		Platform:  platform,
		Cookies:   cookies,
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // Manual cookies last longer
	}

	return nil
}

// SaveCookiesToFile saves cookies to a JSON file
func (cm *CookieManager) SaveCookiesToFile(platform, filepath string) error {
	cookies, err := cm.GetCookiesForPlatform(platform)
	if err != nil {
		return fmt.Errorf("failed to get cookies: %w", err)
	}

	// Convert cookies to JSON-serializable format
	var cookieData []map[string]interface{}
	for _, cookie := range cookies {
		cookieData = append(cookieData, map[string]interface{}{
			"name":     cookie.Name,
			"value":    cookie.Value,
			"domain":   cookie.Domain,
			"path":     cookie.Path,
			"expires":  cookie.Expires.Unix(),
			"secure":   cookie.Secure,
			"httpOnly": cookie.HttpOnly,
		})
	}

	// Write to file
	data, err := json.MarshalIndent(cookieData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cookie file: %w", err)
	}

	return nil
}

// LoadCookiesFromFile loads cookies from a JSON file
func (cm *CookieManager) LoadCookiesFromFile(platform, filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read cookie file: %w", err)
	}

	var cookieData []map[string]interface{}
	if err := json.Unmarshal(data, &cookieData); err != nil {
		return fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	var cookies []*http.Cookie
	for _, item := range cookieData {
		cookie := &http.Cookie{
			Name:     item["name"].(string),
			Value:    item["value"].(string),
			Domain:   item["domain"].(string),
			Path:     item["path"].(string),
			Secure:   item["secure"].(bool),
			HttpOnly: item["httpOnly"].(bool),
		}

		if expires, ok := item["expires"].(float64); ok {
			cookie.Expires = time.Unix(int64(expires), 0)
		}

		cookies = append(cookies, cookie)
	}

	// Cache the cookies
	cm.cache[platform] = &CookieCache{
		Platform:  platform,
		Cookies:   cookies,
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	return nil
}

// ValidateCookies validates cookies by making a test request
func (cm *CookieManager) ValidateCookies(platform string) (bool, error) {
	cookieString, err := cm.GetCookieStringForPlatform(platform)
	if err != nil {
		return false, err
	}

	if cookieString == "" {
		return false, nil
	}

	// Get test URL for platform
	var testURL string
	switch platform {
	case "tiktok":
		testURL = "https://www.tiktok.com"
	case "xhs":
		testURL = "https://www.xiaohongshu.com"
	case "kuaishou":
		testURL = "https://www.kuaishou.com"
	default:
		return false, fmt.Errorf("unsupported platform: %s", platform)
	}

	// Make test request
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Cookie", cookieString)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Check if request was successful
	return resp.StatusCode == http.StatusOK, nil
}

// ClearCache clears the cookie cache
func (cm *CookieManager) ClearCache() {
	cm.cache = make(map[string]*CookieCache)
}

// GetPlatformDomains returns domains for a platform
func (cm *CookieManager) GetPlatformDomains(platform string) []string {
	if domains, exists := PlatformDomains[platform]; exists {
		return domains
	}
	return []string{}
}

// UpdatePlatformCookies updates cookies for a platform from HTTP response
func (cm *CookieManager) UpdatePlatformCookies(platform string, resp *http.Response) {
	if resp == nil {
		return
	}

	var newCookies []*http.Cookie
	for _, cookie := range resp.Cookies() {
		// Check if cookie belongs to platform domains
		domains := cm.GetPlatformDomains(platform)
		for _, domain := range domains {
			if strings.Contains(cookie.Domain, domain) || cookie.Domain == "" {
				if cookie.Domain == "" {
					// Set domain from response URL
					if resp.Request != nil && resp.Request.URL != nil {
						cookie.Domain = resp.Request.URL.Host
					}
				}
				newCookies = append(newCookies, cookie)
				break
			}
		}
	}

	if len(newCookies) > 0 {
		// Update cache with new cookies
		if cached, exists := cm.cache[platform]; exists {
			// Merge with existing cookies
			cached.Cookies = cm.mergeCookies(cached.Cookies, newCookies)
			cached.UpdatedAt = time.Now()
		} else {
			// Create new cache entry
			cm.cache[platform] = &CookieCache{
				Platform:  platform,
				Cookies:   newCookies,
				UpdatedAt: time.Now(),
				ExpiresAt: time.Now().Add(30 * time.Minute),
			}
		}
	}
}

// mergeCookies merges two cookie slices, preferring newer cookies
func (cm *CookieManager) mergeCookies(existing, new []*http.Cookie) []*http.Cookie {
	cookieMap := make(map[string]*http.Cookie)

	// Add existing cookies
	for _, cookie := range existing {
		key := fmt.Sprintf("%s_%s_%s", cookie.Name, cookie.Domain, cookie.Path)
		cookieMap[key] = cookie
	}

	// Override with new cookies
	for _, cookie := range new {
		key := fmt.Sprintf("%s_%s_%s", cookie.Name, cookie.Domain, cookie.Path)
		cookieMap[key] = cookie
	}

	// Convert back to slice
	var result []*http.Cookie
	for _, cookie := range cookieMap {
		result = append(result, cookie)
	}

	return result
}

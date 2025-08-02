package utils

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// HTTPClient represents a configurable HTTP client
type HTTPClient struct {
	client    *http.Client
	transport *http.Transport
	logger    *logrus.Logger
}

// ClientConfig represents HTTP client configuration
type ClientConfig struct {
	Timeout       time.Duration
	MaxIdleConns  int
	IdleConnTimeout time.Duration
	ProxyURL      string
	UserAgent     string
	Cookie        string
	TLSInsecure   bool
	MaxRetries    int
	RetryDelay    time.Duration
}

// NewHTTPClient creates a new HTTP client with the given configuration
func NewHTTPClient(config ClientConfig) *HTTPClient {
	// Create transport
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		IdleConnTimeout:     config.IdleConnTimeout,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 10,
	}
	
	// Configure proxy if provided
	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err == nil {
			switch proxyURL.Scheme {
			case "http", "https":
				transport.Proxy = http.ProxyURL(proxyURL)
			case "socks5":
				dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
				if err == nil {
					transport.DialContext = dialer.(proxy.ContextDialer).DialContext
				}
			}
		}
	}
	
	// Configure TLS
	if config.TLSInsecure {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	
	// Create client
	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}
	
	return &HTTPClient{
		client:    client,
		transport: transport,
		logger:    logrus.New(),
	}
}

// Get performs a GET request
func (c *HTTPClient) Get(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	return c.Do(req, headers)
}

// Post performs a POST request
func (c *HTTPClient) Post(url, contentType string, body strings.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	
	return c.Do(req, headers)
}

// Do performs an HTTP request with custom headers
func (c *HTTPClient) Do(req *http.Request, headers map[string]string) (*http.Response, error) {
	// Set default headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	
	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	// Log request
	c.logger.WithFields(logrus.Fields{
		"method": req.Method,
		"url":    req.URL.String(),
	}).Debug("Making HTTP request")
	
	return c.client.Do(req)
}

// GetWithRetry performs a GET request with retry logic
func (c *HTTPClient) GetWithRetry(url string, headers map[string]string, maxRetries int, retryDelay time.Duration) (*http.Response, error) {
	var resp *http.Response
	var err error
	
	for i := 0; i < maxRetries; i++ {
		resp, err = c.Get(url, headers)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		
		if i < maxRetries-1 {
			c.logger.WithFields(logrus.Fields{
				"attempt": i + 1,
				"max":     maxRetries,
				"url":     url,
				"error":   err,
			}).Warn("Request failed, retrying...")
			
			time.Sleep(retryDelay)
		}
	}
	
	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, err)
}

// Head performs a HEAD request
func (c *HTTPClient) Head(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	return c.Do(req, headers)
}

// GetFileSize returns the size of a remote file
func (c *HTTPClient) GetFileSize(url string) (int64, error) {
	resp, err := c.Head(url, nil)
	if err != nil {
		return 0, fmt.Errorf("error getting file size: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	size, err := ParseContentLength(resp.Header.Get("Content-Length"))
	if err != nil {
		return 0, fmt.Errorf("error parsing content length: %w", err)
	}
	
	return size, nil
}

// SupportsResume checks if the server supports range requests
func (c *HTTPClient) SupportsResume(url string) bool {
	resp, err := c.Head(url, nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.Header.Get("Accept-Ranges") == "bytes"
}

// Close closes the HTTP client and cleans up resources
func (c *HTTPClient) Close() error {
	c.transport.CloseIdleConnections()
	return nil
}

// SetLogger sets the logger for the HTTP client
func (c *HTTPClient) SetLogger(logger *logrus.Logger) {
	c.logger = logger
}

// ParseContentLength parses content length from string
func ParseContentLength(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	
	var length int64
	_, err := fmt.Sscanf(s, "%d", &length)
	if err != nil {
		return 0, fmt.Errorf("error parsing content length: %w", err)
	}
	
	return length, nil
}

// ExtractCookies extracts cookies from response headers
func ExtractCookies(resp *http.Response) map[string]string {
	cookies := make(map[string]string)
	
	for _, cookie := range resp.Cookies() {
		cookies[cookie.Name] = cookie.Value
	}
	
	return cookies
}

// BuildURL builds a URL with query parameters
func BuildURL(baseURL string, params map[string]string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	
	q := u.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()
	
	return u.String()
}

// SanitizeFilename sanitizes a filename by removing invalid characters
func SanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	invalid := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	result := filename
	
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	
	// Remove leading/trailing whitespace and dots
	result = strings.TrimSpace(result)
	result = strings.Trim(result, ".")
	
	// Limit length
	if len(result) > 200 {
		result = result[:200]
	}
	
	return result
}

// FormatBytes formats bytes to human readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats duration to human readable string
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	} else if d < time.Minute {
		return d.Round(time.Second).String()
	} else if d < time.Hour {
		return fmt.Sprintf("%vm %vs", int(d.Minutes()), int(d.Seconds())%60)
	} else {
		return fmt.Sprintf("%vh %vm %vs", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}
}
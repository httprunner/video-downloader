package ratelimit

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

// RateLimiter represents a rate limiter
type RateLimiter struct {
	visitors map[string]*Visitor
	mu       sync.RWMutex
	logger   zerolog.Logger
}

// Visitor represents a visitor with rate limiting info
type Visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*Visitor),
		logger:   zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}
}

// Middleware creates a rate limiting middleware
func (rl *RateLimiter) Middleware(rps int, burst int) gin.HandlerFunc {
	go rl.cleanupVisitors()

	return func(c *gin.Context) {
		// Get identifier for rate limiting
		ip := c.ClientIP()

		// Check for API key in header
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			ip = "api_key:" + apiKey
		}

		// Get limiter for this visitor
		limiter := rl.getLimiter(ip, rps, burst)

		// Check if request is allowed
		if !limiter.Allow() {
			rl.logger.Warn().Str("ip", ip).Msg("Rate limit exceeded")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": "1s",
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rps))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(int(limiter.Tokens())))
		c.Header("X-RateLimit-Reset", "1s")

		c.Next()
	}
}

// getLimiter gets or creates a limiter for a visitor
func (rl *RateLimiter) getLimiter(ip string, rps int, burst int) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(rps), burst)
		rl.visitors[ip] = &Visitor{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors removes old visitors
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Hour)

		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > time.Hour {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Throttler represents a request throttler
type Throttler struct {
	requests chan struct{}
	logger   zerolog.Logger
}

// NewThrottler creates a new throttler
func NewThrottler(maxConcurrent int) *Throttler {
	return &Throttler{
		requests: make(chan struct{}, maxConcurrent),
		logger:   zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}
}

// Middleware creates a throttling middleware
func (t *Throttler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		select {
		case t.requests <- struct{}{}:
			defer func() { <-t.requests }()
			c.Next()
		default:
			t.logger.Warn().Msg("Server overloaded")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Server overloaded, please try again later",
			})
			c.Abort()
		}
	}
}

// AdaptiveRateLimiter adapts rate limits based on server load
type AdaptiveRateLimiter struct {
	baseRPS    int
	maxRPS     int
	minRPS     int
	currentRPS int
	loadAvg    float64
	mu         sync.RWMutex
	logger     zerolog.Logger
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(baseRPS, maxRPS, minRPS int) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		baseRPS:    baseRPS,
		maxRPS:     maxRPS,
		minRPS:     minRPS,
		currentRPS: baseRPS,
		logger:     zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}
}

// Middleware creates an adaptive rate limiting middleware
func (arl *AdaptiveRateLimiter) Middleware() gin.HandlerFunc {
	arl.updateLoadAverage()

	limiter := NewRateLimiter()

	return func(c *gin.Context) {
		arl.mu.RLock()
		currentRPS := arl.currentRPS
		arl.mu.RUnlock()

		// Use current RPS with some burst
		burst := currentRPS * 2
		if burst > 100 {
			burst = 100
		}

		// Apply rate limiting
		limiter.Middleware(currentRPS, burst)(c)
	}
}

// updateLoadAverage periodically updates the load average
func (arl *AdaptiveRateLimiter) updateLoadAverage() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Simulate load average (in real implementation, this would use actual metrics)
			load := arl.calculateLoad()

			arl.mu.Lock()
			arl.loadAvg = load

			// Adjust RPS based on load
			if load > 0.8 {
				// High load, reduce rate limit
				arl.currentRPS = arl.minRPS
			} else if load > 0.5 {
				// Medium load, use base rate
				arl.currentRPS = arl.baseRPS
			} else {
				// Low load, increase rate limit
				arl.currentRPS = arl.maxRPS
			}
			arl.mu.Unlock()

			arl.logger.Debug().
				Str("load", fmt.Sprintf("%f", load)).
				Str("current_rps", fmt.Sprintf("%d", arl.currentRPS)).
				Msg("Updated rate limit")
		}
	}()
}

// calculateLoad calculates the current system load
func (arl *AdaptiveRateLimiter) calculateLoad() float64 {
	// This is a simplified implementation
	// In a real application, you would use actual system metrics
	// like CPU usage, memory usage, request queue length, etc.
	return 0.3 // Simulated 30% load
}

// IPWhitelist represents a whitelist of IPs that bypass rate limiting
type IPWhitelist struct {
	ips map[string]bool
	mu  sync.RWMutex
}

// NewIPWhitelist creates a new IP whitelist
func NewIPWhitelist() *IPWhitelist {
	return &IPWhitelist{
		ips: make(map[string]bool),
	}
}

// Add adds an IP to the whitelist
func (w *IPWhitelist) Add(ip string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ips[ip] = true
}

// Remove removes an IP from the whitelist
func (w *IPWhitelist) Remove(ip string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.ips, ip)
}

// Contains checks if an IP is in the whitelist
func (w *IPWhitelist) Contains(ip string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.ips[ip]
}

// WhitelistMiddleware creates middleware that bypasses rate limiting for whitelisted IPs
func (w *IPWhitelist) WhitelistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if w.Contains(c.ClientIP()) {
			c.Next()
			return
		}
		c.Abort()
	}
}

// Config represents rate limiting configuration
type Config struct {
	Enabled           bool     `mapstructure:"enabled" yaml:"enabled"`
	RequestsPerSecond int      `mapstructure:"requests_per_second" yaml:"requests_per_second"`
	Burst             int      `mapstructure:"burst" yaml:"burst"`
	MaxConcurrent     int      `mapstructure:"max_concurrent" yaml:"max_concurrent"`
	Adaptive          bool     `mapstructure:"adaptive" yaml:"adaptive"`
	WhitelistedIPs    []string `mapstructure:"whitelisted_ips" yaml:"whitelisted_ips"`
}

// Manager manages rate limiting and throttling
type Manager struct {
	rateLimiter     *RateLimiter
	throttler       *Throttler
	adaptiveLimiter *AdaptiveRateLimiter
	whitelist       *IPWhitelist
	config          *Config
	logger          zerolog.Logger
}

// NewManager creates a new rate limiting manager
func NewManager(config *Config) *Manager {
	m := &Manager{
		config:    config,
		whitelist: NewIPWhitelist(),
		logger:    zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}

	if config.Enabled {
		m.rateLimiter = NewRateLimiter()
		m.throttler = NewThrottler(config.MaxConcurrent)

		if config.Adaptive {
			m.adaptiveLimiter = NewAdaptiveRateLimiter(
				config.RequestsPerSecond,
				config.RequestsPerSecond*2,
				config.RequestsPerSecond/2,
			)
		}

		// Add whitelisted IPs
		for _, ip := range config.WhitelistedIPs {
			m.whitelist.Add(ip)
		}
	}

	return m
}

// Middleware returns the appropriate middleware based on configuration
func (m *Manager) Middleware() gin.HandlerFunc {
	if !m.config.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	if m.config.Adaptive {
		return m.adaptiveLimiter.Middleware()
	}

	// Combine rate limiting and throttling
	return gin.HandlerFunc(func(c *gin.Context) {
		// Check whitelist first
		if m.whitelist.Contains(c.ClientIP()) {
			c.Next()
			return
		}

		// Apply throttling
		m.throttler.Middleware()(c)
		if c.IsAborted() {
			return
		}

		// Apply rate limiting
		m.rateLimiter.Middleware(
			m.config.RequestsPerSecond,
			m.config.Burst,
		)(c)
	})
}

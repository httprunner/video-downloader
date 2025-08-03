package monitor

import (
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

// Metrics represents all the application metrics
type Metrics struct {
	// Download metrics
	DownloadsTotal      *prometheus.CounterVec
	DownloadsSuccess    *prometheus.CounterVec
	DownloadsFailed     *prometheus.CounterVec
	DownloadDuration    *prometheus.HistogramVec
	DownloadSize        *prometheus.HistogramVec
	
	// Platform metrics
	PlatformRequests    *prometheus.CounterVec
	PlatformErrors      *prometheus.CounterVec
	
	// System metrics
	Goroutines          prometheus.Gauge
	MemoryUsage         prometheus.Gauge
	
	// Storage metrics
	StorageOperations   *prometheus.CounterVec
	StorageDuration     *prometheus.HistogramVec
	
	// Active downloads
	ActiveDownloads     prometheus.Gauge
	QueueSize           prometheus.Gauge
	
	// Custom metrics
	CustomMetrics       map[string]prometheus.Metric
	mutex               sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		DownloadsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "video_downloader_downloads_total",
				Help: "Total number of download attempts",
			},
			[]string{"platform", "format"},
		),
		
		DownloadsSuccess: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "video_downloader_downloads_success_total",
				Help: "Total number of successful downloads",
			},
			[]string{"platform", "format"},
		),
		
		DownloadsFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "video_downloader_downloads_failed_total",
				Help: "Total number of failed downloads",
			},
			[]string{"platform", "format", "error_type"},
		),
		
		DownloadDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "video_downloader_download_duration_seconds",
				Help:    "Time spent downloading videos",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"platform", "format"},
		),
		
		DownloadSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "video_downloader_download_size_bytes",
				Help:    "Size of downloaded videos",
				Buckets: []float64{1e6, 1e7, 1e8, 1e9, 5e9, 1e10}, // 1MB to 10GB
			},
			[]string{"platform", "format"},
		),
		
		PlatformRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "video_downloader_platform_requests_total",
				Help: "Total requests to platform APIs",
			},
			[]string{"platform", "endpoint"},
		),
		
		PlatformErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "video_downloader_platform_errors_total",
				Help: "Total errors from platform APIs",
			},
			[]string{"platform", "endpoint", "error_type"},
		),
		
		Goroutines: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "video_downloader_goroutines",
			Help: "Number of goroutines",
		}),
		
		MemoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "video_downloader_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),
		
		StorageOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "video_downloader_storage_operations_total",
				Help: "Total storage operations",
			},
			[]string{"operation", "status"},
		),
		
		StorageDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "video_downloader_storage_duration_seconds",
				Help:    "Time spent on storage operations",
				Buckets: []float64{0.001, 0.01, 0.1, 1, 10},
			},
			[]string{"operation"},
		),
		
		ActiveDownloads: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "video_downloader_active_downloads",
			Help: "Number of active downloads",
		}),
		
		QueueSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "video_downloader_queue_size",
			Help: "Number of items in download queue",
		}),
		
		CustomMetrics: make(map[string]prometheus.Metric),
	}
}

// Monitor represents the monitoring system
type Monitor struct {
	metrics     *Metrics
	logger      zerolog.Logger
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewMonitor creates a new monitor instance
func NewMonitor() *Monitor {
	return &Monitor{
		metrics:  NewMetrics(),
		logger:   zerolog.New(os.Stdout).With().Timestamp().Logger(),
		stopChan: make(chan struct{}),
	}
}

// Start starts the monitoring system
func (m *Monitor) Start() {
	m.wg.Add(1)
	go m.collectSystemMetrics()
	
	m.logger.Info().Msg("Monitoring system started")
}

// Stop stops the monitoring system
func (m *Monitor) Stop() {
	close(m.stopChan)
	m.wg.Wait()
	
	m.logger.Info().Msg("Monitoring system stopped")
}

// collectSystemMetrics collects system metrics periodically
func (m *Monitor) collectSystemMetrics() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Update goroutine count
			m.metrics.Goroutines.Set(float64(runtime.NumGoroutine()))
			
			// Update memory usage
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			m.metrics.MemoryUsage.Set(float64(memStats.Alloc))
			
		case <-m.stopChan:
			return
		}
	}
}

// RecordDownloadStart records the start of a download
func (m *Monitor) RecordDownloadStart(platform, format string) {
	m.metrics.DownloadsTotal.WithLabelValues(platform, format).Inc()
	m.metrics.ActiveDownloads.Inc()
}

// RecordDownloadSuccess records a successful download
func (m *Monitor) RecordDownloadSuccess(platform, format string, duration time.Duration, size int64) {
	m.metrics.DownloadsSuccess.WithLabelValues(platform, format).Inc()
	m.metrics.DownloadDuration.WithLabelValues(platform, format).Observe(duration.Seconds())
	m.metrics.DownloadSize.WithLabelValues(platform, format).Observe(float64(size))
	m.metrics.ActiveDownloads.Dec()
}

// RecordDownloadFailure records a failed download
func (m *Monitor) RecordDownloadFailure(platform, format, errorType string, duration time.Duration) {
	m.metrics.DownloadsFailed.WithLabelValues(platform, format, errorType).Inc()
	m.metrics.DownloadDuration.WithLabelValues(platform, format).Observe(duration.Seconds())
	m.metrics.ActiveDownloads.Dec()
}

// RecordPlatformRequest records a platform API request
func (m *Monitor) RecordPlatformRequest(platform, endpoint string) {
	m.metrics.PlatformRequests.WithLabelValues(platform, endpoint).Inc()
}

// RecordPlatformError records a platform API error
func (m *Monitor) RecordPlatformError(platform, endpoint, errorType string) {
	m.metrics.PlatformErrors.WithLabelValues(platform, endpoint, errorType).Inc()
}

// RecordStorageOperation records a storage operation
func (m *Monitor) RecordStorageOperation(operation, status string, duration time.Duration) {
	m.metrics.StorageOperations.WithLabelValues(operation, status).Inc()
	m.metrics.StorageDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// UpdateQueueSize updates the queue size metric
func (m *Monitor) UpdateQueueSize(size int) {
	m.metrics.QueueSize.Set(float64(size))
}

// AddCustomMetric adds a custom metric
func (m *Monitor) AddCustomMetric(name string, metric prometheus.Metric) {
	m.metrics.mutex.Lock()
	defer m.metrics.mutex.Unlock()
	
	m.metrics.CustomMetrics[name] = metric
}

// GetMetrics returns all metrics
func (m *Monitor) GetMetrics() *Metrics {
	return m.metrics
}

// GetLogger returns the logger
func (m *Monitor) GetLogger() zerolog.Logger {
	return m.logger
}

// SetLogger sets the logger
func (m *Monitor) SetLogger(logger zerolog.Logger) {
	m.logger = logger
}

// HealthCheck performs a health check
func (m *Monitor) HealthCheck() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return map[string]interface{}{
		"goroutines":      runtime.NumGoroutine(),
		"memory_usage":    memStats.Alloc,
		"memory_sys":      memStats.Sys,
		"gc_cycles":       memStats.NumGC,
	}
}

// Middleware represents monitoring middleware for HTTP servers
type Middleware struct {
	monitor *Monitor
}

// NewMiddleware creates a new monitoring middleware
func NewMiddleware(monitor *Monitor) *Middleware {
	return &Middleware{
		monitor: monitor,
	}
}

// RecordHTTPRequest records an HTTP request
func (m *Middleware) RecordHTTPRequest(method, path, status string, duration time.Duration) {
	// This could be expanded to track HTTP-specific metrics
	m.monitor.RecordStorageOperation("http_"+method+"_"+path, status, duration)
}
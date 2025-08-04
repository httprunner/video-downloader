package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"video-downloader/pkg/models"
)

// Manager manages application configuration
type Manager struct {
	config *models.Config
	viper  *viper.Viper
	logger zerolog.Logger
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{
		config: &models.Config{},
		viper:  viper.New(),
		logger: zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}
}

// Load loads configuration from file and environment
func (m *Manager) Load(configPath string) (*models.Config, error) {
	// Set default values
	m.setDefaults()

	// Configure viper
	m.viper.SetConfigName("config")
	m.viper.SetConfigType("yaml")

	if configPath != "" {
		m.viper.AddConfigPath(configPath)
	} else {
		// Default config paths
		m.viper.AddConfigPath(".")
		m.viper.AddConfigPath("./config")
		m.viper.AddConfigPath("$HOME/.video-downloader")
		m.viper.AddConfigPath("/etc/video-downloader")
	}

	// Enable environment variable support
	m.viper.AutomaticEnv()
	m.viper.SetEnvPrefix("VD")

	// Read configuration
	if err := m.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, create default
		if err := m.createDefaultConfig(); err != nil {
			m.logger.Warn().Msgf("Failed to create default config: %v", err)
		}
	}

	// Unmarshal configuration
	if err := m.viper.Unmarshal(m.config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Ensure directories exist
	if err := m.ensureDirectories(); err != nil {
		return nil, fmt.Errorf("error ensuring directories: %w", err)
	}

	// Configure logger
	m.configureLogger()

	return m.config, nil
}

// Save saves configuration to file
func (m *Manager) Save(configPath string) error {
	m.viper.Set("config", m.config)

	configFile := filepath.Join(configPath, "config.yaml")
	if err := m.viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}

	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *models.Config {
	return m.config
}

// UpdateConfig updates specific configuration values
func (m *Manager) UpdateConfig(updates map[string]interface{}) error {
	for key, value := range updates {
		m.viper.Set(key, value)
	}

	return m.viper.Unmarshal(m.config)
}

// setDefaults sets default configuration values
func (m *Manager) setDefaults() {
	// Server defaults
	m.viper.SetDefault("server.host", "0.0.0.0")
	m.viper.SetDefault("server.port", 8080)
	m.viper.SetDefault("server.read_timeout", 30)
	m.viper.SetDefault("server.write_timeout", 30)

	// Download defaults
	m.viper.SetDefault("download.max_workers", 5)
	m.viper.SetDefault("download.chunk_size", 1024*1024) // 1MB
	m.viper.SetDefault("download.timeout", 300)
	m.viper.SetDefault("download.retry_count", 3)
	m.viper.SetDefault("download.save_path", "./downloads")
	m.viper.SetDefault("download.create_folder", true)
	m.viper.SetDefault("download.file_naming", "{platform}_{author}_{title}_{id}")

	// Database defaults
	m.viper.SetDefault("database.type", "sqlite")
	m.viper.SetDefault("database.path", "./data/video-downloader.db")
	m.viper.SetDefault("database.max_conns", 10)

	// Log defaults
	m.viper.SetDefault("log.level", "info")
	m.viper.SetDefault("log.format", "text")
	m.viper.SetDefault("log.output", "stdout")

	// Platform defaults
	m.viper.SetDefault("platforms.tiktok.enabled", true)
	m.viper.SetDefault("platforms.xhs.enabled", true)
	m.viper.SetDefault("platforms.kuaishou.enabled", true)

	// Auth defaults
	m.viper.SetDefault("auth.enabled", true)
	m.viper.SetDefault("auth.jwt_secret", "your-secret-key-change-this-in-production")
	m.viper.SetDefault("auth.token_expiry", 24)
	m.viper.SetDefault("auth.admin_password", "admin123")

	// Rate limit defaults
	m.viper.SetDefault("rate_limit.enabled", true)
	m.viper.SetDefault("rate_limit.requests_per_second", 10)
	m.viper.SetDefault("rate_limit.burst", 30)
	m.viper.SetDefault("rate_limit.max_concurrent", 100)
	m.viper.SetDefault("rate_limit.adaptive", true)
	m.viper.SetDefault("rate_limit.whitelisted_ips", []string{"127.0.0.1", "::1"})
}

// createDefaultConfig creates a default configuration file
func (m *Manager) createDefaultConfig() error {
	configDir := "./config"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")

	// Create default config content
	defaultConfig := `# Video Downloader Configuration

server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30
  write_timeout: 30

download:
  max_workers: 5
  chunk_size: 1048576  # 1MB
  timeout: 300
  retry_count: 3
  save_path: ./downloads
  create_folder: true
  file_naming: "{platform}_{author}_{title}_{id}"

database:
  type: sqlite
  path: ./data/video-downloader.db
  max_conns: 10

log:
  level: info
  format: text
  output: stdout

proxy:
  enabled: false
  type: http
  host: ""
  port: 0
  username: ""
  password: ""

platforms:
  tiktok:
    enabled: true
    api_key: ""
    cookie: ""
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

  xhs:
    enabled: true
    cookie: ""
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

  kuaishou:
    enabled: true
    cookie: ""
    user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

auth:
  enabled: true
  jwt_secret: "your-secret-key-change-this-in-production"
  token_expiry: 24
  admin_password: "admin123"

rate_limit:
  enabled: true
  requests_per_second: 10
  burst: 30
  max_concurrent: 100
  adaptive: true
  whitelisted_ips:
    - "127.0.0.1"
    - "::1"
`

	if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("error writing default config: %w", err)
	}

	m.logger.Info().Msgf("Created default config file at: %s", configFile)
	return nil
}

// ensureDirectories ensures all required directories exist
func (m *Manager) ensureDirectories() error {
	dirs := []string{
		m.config.Download.SavePath,
		filepath.Dir(m.config.Database.Path),
		"./logs",
		"./temp",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", dir, err)
		}
	}

	return nil
}

// configureLogger configures the logger based on settings
func (m *Manager) configureLogger() {
	// Set log level
	level, err := zerolog.ParseLevel(m.config.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set log format
	if m.config.Log.Format == "json" {
		// JSON format is default for zerolog
	} else {
		m.logger = m.logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Set log output
	if m.config.Log.Output != "stdout" {
		file, err := os.OpenFile(m.config.Log.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			m.logger = m.logger.Output(file)
		}
	}
}

// GetLogger returns the logger instance
func (m *Manager) GetLogger() zerolog.Logger {
	return m.logger
}

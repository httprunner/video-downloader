package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"video-downloader/internal/config"
	"video-downloader/internal/server"
	"video-downloader/internal/storage"
)

func main() {
	// Initialize zerolog logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Load configuration
	configManager := config.NewManager()
	cfg, err := configManager.Load("")
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading configuration")
	}

	// Initialize storage
	storage, err := storage.NewSQLite(cfg.Database.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("Error initializing storage")
	}
	defer storage.Close()

	// Create and run server
	srv := server.NewServer(cfg, storage)
	if err := srv.Run(); err != nil {
		log.Fatal().Err(err).Msg("Error running server")
	}
}

package main

import (
	"github.com/sirupsen/logrus"
	
	"video-downloader/internal/config"
	"video-downloader/internal/server"
	"video-downloader/internal/storage"
)

func main() {
	// Load configuration
	configManager := config.NewManager()
	cfg, err := configManager.Load("")
	if err != nil {
		logrus.WithError(err).Fatal("Error loading configuration")
	}
	
	// Initialize storage
	storage, err := storage.NewSQLite(cfg.Database.Path)
	if err != nil {
		logrus.WithError(err).Fatal("Error initializing storage")
	}
	defer storage.Close()
	
	// Create and run server
	srv := server.NewServer(cfg, storage)
	if err := srv.Run(); err != nil {
		logrus.WithError(err).Fatal("Error running server")
	}
}
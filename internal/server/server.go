package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"video-downloader/internal/auth"
	"video-downloader/internal/downloader"
	"video-downloader/internal/monitor"
	"video-downloader/internal/ratelimit"
	"video-downloader/internal/utils"
	"video-downloader/pkg/models"
)

// Server represents the API server
type Server struct {
	config       *models.Config
	storage      models.Storage
	downloader   *downloader.Manager
	monitor      *monitor.Monitor
	authService  *auth.AuthService
	rateLimitMgr *ratelimit.Manager
	httpServer   *http.Server
	logger       zerolog.Logger
}

// NewServer creates a new API server
func NewServer(cfg *models.Config, storage models.Storage) *Server {
	// Create download manager
	dm := downloader.NewManager(cfg, storage)
	if err := dm.Start(); err != nil {
		log.Fatal().Err(err).Msg("Error starting download manager")
	}

	// Create monitor
	mon := monitor.NewMonitor()
	mon.Start()

	// Create auth service
	authSvc := auth.NewAuthService(cfg.Auth.JWTSecret)
	authSvc.SetStorage(storage)

	// Create default admin user if none exists
	if _, err := storage.GetUserByUsername("admin"); err != nil {
		adminUser, err := authSvc.CreateUser("admin", cfg.Auth.AdminPassword, "admin")
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create admin user")
		} else {
			storage.SaveUser(adminUser)
			log.Info().Msg("Created default admin user")
		}
	}

	// Set Gin mode
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create rate limit manager
	rateLimitConfig := &ratelimit.Config{
		Enabled:           cfg.RateLimit.Enabled,
		RequestsPerSecond: cfg.RateLimit.RequestsPerSecond,
		Burst:             cfg.RateLimit.Burst,
		MaxConcurrent:     cfg.RateLimit.MaxConcurrent,
		Adaptive:          cfg.RateLimit.Adaptive,
		WhitelistedIPs:    cfg.RateLimit.WhitelistedIPs,
	}
	rateLimitMgr := ratelimit.NewManager(rateLimitConfig)

	return &Server{
		config:       cfg,
		storage:      storage,
		downloader:   dm,
		monitor:      mon,
		authService:  authSvc,
		rateLimitMgr: rateLimitMgr,
		logger:       zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}
}

// Start starts the API server
func (s *Server) Start() error {
	// Create Gin router
	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(s.corsMiddleware())

	// Setup routes
	s.setupRoutes(router)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(s.config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.Server.WriteTimeout) * time.Second,
	}

	// Start server
	go func() {
		s.logger.Info().Str("address", s.httpServer.Addr).Msg("Starting API server")
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatal().Err(err).Msg("Error starting server")
		}
	}()

	return nil
}

// Stop stops the API server
func (s *Server) Stop() error {
	s.logger.Info().Msg("Stopping API server...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop monitor
	s.monitor.Stop()

	// Stop download manager
	if err := s.downloader.Stop(); err != nil {
		s.logger.Error().Err(err).Msg("Error stopping download manager")
	}

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error().Err(err).Msg("Error shutting down server")
		return err
	}

	s.logger.Info().Msg("API server stopped")
	return nil
}

// setupRoutes sets up the API routes
func (s *Server) setupRoutes(router *gin.Engine) {
	// Create auth middleware
	authMiddleware := auth.NewAuthMiddleware(s.authService)

	// Apply rate limiting to all API routes
	api := router.Group("/api")
	api.Use(s.rateLimitMgr.Middleware())

	// Health check
	router.GET("/health", s.healthCheck)

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := api.Group("/v1")
	{
		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			// Less strict rate limiting for auth endpoints
			authLimiter := ratelimit.NewRateLimiter()
			auth.Use(authLimiter.Middleware(5, 10)) // 5 requests per second, burst 10

			auth.POST("/login", s.login)
			auth.POST("/register", s.register)
			auth.POST("/refresh", s.refreshToken)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(authMiddleware.Required())
		{
			// Video routes - stricter rate limiting for downloads
			videos := protected.Group("/videos")
			{
				downloadLimiter := ratelimit.NewRateLimiter()
				videos.Use(downloadLimiter.Middleware(2, 5)) // 2 downloads per second, burst 5

				videos.POST("/download", s.downloadVideo)
				videos.POST("/batch", s.batchDownload)
				videos.GET("/:id", s.getVideo)
				videos.GET("", s.listVideos)
				videos.POST("/info", s.getVideoInfo)
			}

			// Download routes - same strict limits
			downloads := protected.Group("/downloads")
			{
				downloadLimiter := ratelimit.NewRateLimiter()
				downloads.Use(downloadLimiter.Middleware(2, 5))

				downloads.GET("", s.getDownloads)
				downloads.GET("/:id", s.getDownload)
				downloads.DELETE("/:id", s.cancelDownload)
				downloads.POST("/:id/retry", s.retryDownload)
			}

			// Author routes - moderate rate limiting
			authors := protected.Group("/authors")
			{
				authorLimiter := ratelimit.NewRateLimiter()
				authors.Use(authorLimiter.Middleware(10, 20)) // 10 requests per second, burst 20

				authors.GET("/:platform/:id", s.getAuthor)
				authors.GET("/:platform/:id/videos", s.getAuthorVideos)
			}

			// Stats routes - moderate rate limiting
			stats := protected.Group("/stats")
			{
				authorLimiter := ratelimit.NewRateLimiter()
				stats.Use(authorLimiter.Middleware(10, 20))

				stats.GET("", s.getStats)
				stats.GET("/downloads", s.getDownloadStats)
				stats.GET("/system", s.getSystemStats)
			}

			// User management routes (admin only)
			admin := protected.Group("")
			admin.Use(authMiddleware.RoleRequired("admin"))
			{
				adminLimiter := ratelimit.NewRateLimiter()
				admin.Use(adminLimiter.Middleware(20, 50)) // 20 requests per second, burst 50

				admin.GET("/users", s.listUsers)
				admin.POST("/users", s.createUser)
				admin.GET("/users/:id", s.getUser)
				admin.PUT("/users/:id", s.updateUser)
				admin.DELETE("/users/:id", s.deleteUser)
			}

			// Session management
			protected.POST("/logout", s.logout)
			protected.GET("/sessions", s.getUserSessions)
			protected.DELETE("/sessions/:id", s.invalidateSession)
		}
	}

	// Static files
	router.Static("/downloads", s.config.Download.SavePath)
}

// Health check handler
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	})
}

// Download video handler
func (s *Server) downloadVideo(c *gin.Context) {
	var req struct {
		URL        string `json:"url" binding:"required"`
		OutputPath string `json:"output_path"`
		Format     string `json:"format"`
		Quality    string `json:"quality"`
		Download   bool   `json:"download"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Download options
	options := &downloader.DownloadOptions{
		OutputPath: req.OutputPath,
		Format:     req.Format,
		Quality:    req.Quality,
		Progress:   true,
	}

	// Start download
	result, err := s.downloader.Download(req.URL, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Prepare response
	response := gin.H{
		"success": result.Success,
		"message": result.Message,
	}

	if result.Video != nil {
		response["video"] = result.Video
	}

	if result.Error != nil {
		response["error"] = result.Error.Error()
	}

	c.JSON(http.StatusOK, response)
}

// Batch download handler
func (s *Server) batchDownload(c *gin.Context) {
	var req struct {
		URLs       []string `json:"urls" binding:"required"`
		OutputPath string   `json:"output_path"`
		Format     string   `json:"format"`
		Quality    string   `json:"quality"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Download options
	options := &downloader.DownloadOptions{
		OutputPath: req.OutputPath,
		Format:     req.Format,
		Quality:    req.Quality,
		Progress:   true,
	}

	// Start batch download
	results, err := s.downloader.DownloadBatch(req.URLs, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Prepare response
	response := gin.H{
		"total":   len(results),
		"results": results,
	}

	c.JSON(http.StatusOK, response)
}

// Get video handler
func (s *Server) getVideo(c *gin.Context) {
	id := c.Param("id")

	video, err := s.storage.GetVideoInfo(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if video == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	c.JSON(http.StatusOK, video)
}

// List videos handler
func (s *Server) listVideos(c *gin.Context) {
	// Parse query parameters
	filter := models.VideoFilter{
		Limit: 50,
	}

	if platform := c.Query("platform"); platform != "" {
		p := models.Platform(platform)
		filter.Platform = &p
	}

	if mediaType := c.Query("media_type"); mediaType != "" {
		mt := models.MediaType(mediaType)
		filter.MediaType = &mt
	}

	if status := c.Query("status"); status != "" {
		filter.Status = &status
	}

	if authorID := c.Query("author_id"); authorID != "" {
		filter.AuthorID = &authorID
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	// Get videos
	videos, err := s.storage.ListVideos(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"total":  len(videos),
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// Get video info handler
func (s *Server) getVideoInfo(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get video info
	videoInfo, err := s.downloader.GetVideoInfo(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if videoInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	c.JSON(http.StatusOK, videoInfo)
}

// Get downloads handler
func (s *Server) getDownloads(c *gin.Context) {
	status := s.downloader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// Get download handler
func (s *Server) getDownload(c *gin.Context) {
	id := c.Param("id")

	task, err := s.storage.GetDownloadTask(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Download task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// Cancel download handler
func (s *Server) cancelDownload(c *gin.Context) {
	id := c.Param("id")

	if err := s.downloader.CancelDownload(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Download cancelled"})
}

// Retry download handler
func (s *Server) retryDownload(c *gin.Context) {
	id := c.Param("id")

	if err := s.downloader.RetryDownload(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Download retry initiated"})
}

// Get author handler
func (s *Server) getAuthor(c *gin.Context) {
	platform := models.Platform(c.Param("platform"))
	id := c.Param("id")

	author, err := s.downloader.GetAuthorInfo(platform, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if author == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Author not found"})
		return
	}

	c.JSON(http.StatusOK, author)
}

// Get author videos handler
func (s *Server) getAuthorVideos(c *gin.Context) {
	platform := models.Platform(c.Param("platform"))
	authorID := c.Param("id")

	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	videos, err := s.storage.GetVideosByAuthor(authorID, platform, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"total":  len(videos),
	})
}

// Get stats handler
func (s *Server) getStats(c *gin.Context) {
	stats, err := s.storage.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Get download stats handler
func (s *Server) getDownloadStats(c *gin.Context) {
	// Get recent downloads
	recent, err := s.storage.GetRecentDownloads(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get failed downloads
	failed, err := s.storage.GetFailedDownloads()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recent_downloads": recent,
		"failed_downloads": failed,
		"active_downloads": len(s.downloader.GetStatus()["jobs"].([]*utils.DownloadJob)),
	})
}

// Get system stats handler
func (s *Server) getSystemStats(c *gin.Context) {
	stats := s.monitor.HealthCheck()
	c.JSON(http.StatusOK, stats)
}

// Login handler
func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Authenticate user
	token, user, err := s.authService.Authenticate(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Create session manager and session
	sessionManager := auth.NewSessionManager()
	session, err := sessionManager.CreateSession(user.ID, token, time.Now().Add(24*time.Hour))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Save session to storage
	if err := s.storage.SaveSession(session); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save session")
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
		"expires_at": session.ExpiresAt,
	})
}

// Register handler
func (s *Server) register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Email    string `json:"email"`
		Role     string `json:"role"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only allow admins to create users with specific roles
	if req.Role != "" && req.Role != "user" {
		user, exists := auth.GetUser(c)
		if !exists || user.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			return
		}
	}

	// Set default role if not specified
	if req.Role == "" {
		req.Role = "user"
	}

	// Create user
	user, err := s.authService.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Save user to storage
	if err := s.storage.SaveUser(user); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

// Refresh token handler
func (s *Server) refreshToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Refresh token
	newToken, err := s.authService.RefreshToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": newToken,
	})
}

// Logout handler
func (s *Server) logout(c *gin.Context) {
	user, exists := auth.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Invalidate all user sessions
	if err := s.storage.InvalidateAllUserSessions(user.ID); err != nil {
		s.logger.Error().Err(err).Msg("Failed to invalidate sessions")
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// List users handler (admin only)
func (s *Server) listUsers(c *gin.Context) {
	// This would typically include pagination
	// For simplicity, we'll return all users
	c.JSON(http.StatusOK, gin.H{
		"users":   []string{},
		"message": "User listing not implemented yet",
	})
}

// Create user handler (admin only)
func (s *Server) createUser(c *gin.Context) {
	s.register(c)
}

// Get user handler (admin only)
func (s *Server) getUser(c *gin.Context) {
	id := c.Param("id")

	user, err := s.storage.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"role":       user.Role,
		"active":     user.Active,
		"created_at": user.CreatedAt,
	})
}

// Update user handler (admin only)
func (s *Server) updateUser(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Active   *bool  `json:"active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing user
	user, err := s.storage.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update fields
	updates := make(map[string]interface{})
	if req.Username != "" {
		updates["username"] = req.Username
	}
	if req.Password != "" {
		updates["password"] = req.Password
	}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}

	// Update user
	if err := s.authService.UpdateUser(user.Username, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save to storage
	if err := s.storage.UpdateUser(user); err != nil {
		s.logger.Error().Err(err).Msg("Failed to update user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

// Delete user handler (admin only)
func (s *Server) deleteUser(c *gin.Context) {
	id := c.Param("id")

	// Get current user
	currentUser, exists := auth.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Prevent self-deletion
	if currentUser.ID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own account"})
		return
	}

	// Delete user
	if err := s.storage.DeleteUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// Get user sessions handler
func (s *Server) getUserSessions(c *gin.Context) {
	_, exists := auth.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": []string{},
		"message":  "Session listing not implemented yet",
	})
}

// Invalidate session handler
func (s *Server) invalidateSession(c *gin.Context) {
	sessionID := c.Param("id")

	// Get session
	session, err := s.storage.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Check if session belongs to current user
	user, exists := auth.GetUser(c)
	if !exists || user.ID != session.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Invalidate session
	if err := s.storage.InvalidateSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session invalidated successfully"})
}

// CORS middleware
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Run runs the server with signal handling
func (s *Server) Run() error {
	// Start server
	if err := s.Start(); err != nil {
		return err
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	<-sigChan

	// Stop server
	return s.Stop()
}

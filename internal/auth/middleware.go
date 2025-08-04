package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"video-downloader/pkg/models"
)

// AuthMiddleware handles authentication for protected routes
type AuthMiddleware struct {
	authService *AuthService
	logger      zerolog.Logger
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authService *AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		logger:      zerolog.New(os.Stdout).With().Timestamp().Logger(),
	}
}

// Required enforces authentication for routes
func (m *AuthMiddleware) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from Bearer format
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		// Validate token
		user, err := m.authService.ValidateToken(tokenString)
		if err != nil {
			m.logger.Warn().Err(err).Msg("Invalid token")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Next()
	}
}

// Optional allows optional authentication for routes
func (m *AuthMiddleware) Optional() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		// Extract token from Bearer format
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.Next()
			return
		}

		// Validate token
		user, err := m.authService.ValidateToken(tokenString)
		if err != nil {
			// Token is invalid, but we'll continue without authentication
			c.Next()
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Next()
	}
}

// RoleRequired enforces specific role for routes
func (m *AuthMiddleware) RoleRequired(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First authenticate the user
		m.Required()(c)
		if c.IsAborted() {
			return
		}

		// Check role
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		u := user.(*models.User)
		userRole := u.Role

		// Check if user has required role
		for _, role := range roles {
			if userRole == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}

// GetUser returns the authenticated user from context
func GetUser(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	u, ok := user.(*models.User)
	return u, ok
}

// GetUserID returns the authenticated user ID from context
func GetUserID(c *gin.Context) (string, bool) {
	user, exists := GetUser(c)
	if !exists {
		return "", false
	}
	return user.ID, true
}

// GetUsername returns the authenticated username from context
func GetUsername(c *gin.Context) (string, bool) {
	user, exists := GetUser(c)
	if !exists {
		return "", false
	}
	return user.Username, true
}

package auth

import (
	"errors"
	"time"
	
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	
	"video-downloader/pkg/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")
)

// AuthService handles authentication and authorization
type AuthService struct {
	storage    models.Storage
	jwtSecret  []byte
	logger     *logrus.Logger
}

// NewAuthService creates a new authentication service
func NewAuthService(jwtSecret string) *AuthService {
	return &AuthService{
		jwtSecret:  []byte(jwtSecret),
		logger:     logrus.New(),
	}
}

// SetStorage sets the storage implementation
func (s *AuthService) SetStorage(storage models.Storage) {
	s.storage = storage
}

// CreateUser creates a new user
func (s *AuthService) CreateUser(username, password, role string) (*models.User, error) {
	if s.storage == nil {
		return nil, errors.New("storage not set")
	}
	
	// Check if user already exists
	if existingUser, _ := s.storage.GetUserByUsername(username); existingUser != nil {
		return nil, errors.New("user already exists")
	}
	
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	
	// Create user
	user := &models.User{
		ID:        generateUserID(),
		Username:  username,
		Password:  string(hashedPassword),
		Role:      role,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	s.logger.WithField("username", username).Info("User created successfully")
	
	return user, nil
}

// Authenticate authenticates a user and returns a JWT token
func (s *AuthService) Authenticate(username, password string) (string, *models.User, error) {
	if s.storage == nil {
		return "", nil, errors.New("storage not set")
	}
	
	user, err := s.storage.GetUserByUsername(username)
	if err != nil {
		return "", nil, err
	}
	
	if user == nil {
		return "", nil, ErrUserNotFound
	}
	
	if !user.Active {
		return "", nil, errors.New("user account is inactive")
	}
	
	// Check password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", nil, ErrInvalidCredentials
	}
	
	// Generate JWT token
	token, err := s.generateToken(user)
	if err != nil {
		return "", nil, err
	}
	
	// Update last login
	now := time.Now()
	user.LastLogin = &now
	
	s.logger.WithField("username", username).Info("User authenticated successfully")
	
	return token, user, nil
}

// ValidateToken validates a JWT token and returns the user
func (s *AuthService) ValidateToken(tokenString string) (*models.User, error) {
	if s.storage == nil {
		return nil, errors.New("storage not set")
	}
	
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	
	if err != nil {
		return nil, err
	}
	
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, ok := claims["user_id"].(string)
		if !ok {
			return nil, ErrInvalidToken
		}
		
		user, err := s.storage.GetUserByID(userID)
		if err != nil {
			return nil, err
		}
		
		if user == nil || !user.Active {
			return nil, ErrUserNotFound
		}
		
		return user, nil
	}
	
	return nil, ErrInvalidToken
}

// RefreshToken refreshes a JWT token
func (s *AuthService) RefreshToken(tokenString string) (string, error) {
	user, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	
	return s.generateToken(user)
}

// GetUserByUsername returns a user by username
func (s *AuthService) GetUserByUsername(username string) (*models.User, error) {
	if s.storage == nil {
		return nil, errors.New("storage not set")
	}
	
	user, err := s.storage.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	
	if user == nil {
		return nil, ErrUserNotFound
	}
	
	return user, nil
}

// UpdateUser updates user information
func (s *AuthService) UpdateUser(username string, updates map[string]interface{}) error {
	if s.storage == nil {
		return errors.New("storage not set")
	}
	
	user, err := s.storage.GetUserByUsername(username)
	if err != nil {
		return err
	}
	
	if user == nil {
		return ErrUserNotFound
	}
	
	// Update user object
	for key, value := range updates {
		switch key {
		case "password":
			if newPassword, ok := value.(string); ok {
				hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
				if err != nil {
					return err
				}
				user.Password = string(hashedPassword)
			}
		case "role":
			if role, ok := value.(string); ok {
				user.Role = role
			}
		case "active":
			if active, ok := value.(bool); ok {
				user.Active = active
			}
		}
	}
	
	user.UpdatedAt = time.Now()
	
	s.logger.WithField("username", username).Info("User updated successfully")
	
	return nil
}

// generateToken generates a JWT token for a user
func (s *AuthService) generateToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	
	return token.SignedString(s.jwtSecret)
}

// generateUserID generates a unique user ID
func generateUserID() string {
	return "user_" + time.Now().Format("20060102150405")
}

// SessionManager manages user sessions
type SessionManager struct {
	sessions map[string]*models.Session
	logger   *logrus.Logger
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*models.Session),
		logger:   logrus.New(),
	}
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(userID, token string, expiresAt time.Time) (*models.Session, error) {
	session := &models.Session{
		ID:        generateSessionID(),
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		Active:    true,
	}
	
	sm.sessions[session.ID] = session
	
	sm.logger.WithField("user_id", userID).Info("Session created")
	
	return session, nil
}

// GetSession returns a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*models.Session, error) {
	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}
	
	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		session.Active = false
		return nil, ErrTokenExpired
	}
	
	return session, nil
}

// InvalidateSession invalidates a session
func (sm *SessionManager) InvalidateSession(sessionID string) error {
	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}
	
	session.Active = false
	
	sm.logger.WithField("session_id", sessionID).Info("Session invalidated")
	
	return nil
}

// InvalidateAllUserSessions invalidates all sessions for a user
func (sm *SessionManager) InvalidateAllUserSessions(userID string) error {
	count := 0
	for _, session := range sm.sessions {
		if session.UserID == userID && session.Active {
			session.Active = false
			count++
		}
	}
	
	sm.logger.WithFields(logrus.Fields{
		"user_id": userID,
		"count":   count,
	}).Info("All user sessions invalidated")
	
	return nil
}

// CleanupExpiredSessions removes expired sessions
func (sm *SessionManager) CleanupExpiredSessions() {
	count := 0
	now := time.Now()
	
	for id, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			delete(sm.sessions, id)
			count++
		}
	}
	
	if count > 0 {
		sm.logger.WithField("count", count).Info("Cleaned up expired sessions")
	}
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return "sess_" + time.Now().Format("20060102150405") + "_" + generateRandomString(8)
}

// generateRandomString generates a random string
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().Nanosecond()%len(charset)]
	}
	return string(b)
}
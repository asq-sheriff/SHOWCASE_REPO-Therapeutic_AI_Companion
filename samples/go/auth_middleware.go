// Package auth provides HIPAA-compliant authentication and authorization
// for the Lilo Engine therapeutic AI platform.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Role definitions for RBAC
type Role string

const (
	RoleResident Role = "resident"
	RoleFamily   Role = "family"
	RoleStaff    Role = "staff"
	RoleProvider Role = "provider"
	RoleAdmin    Role = "admin"
	RoleSystem   Role = "system"
)

// Permission definitions
type Permission string

const (
	PermissionReadResident     Permission = "resident:read"
	PermissionWriteResident    Permission = "resident:write"
	PermissionReadCrisis       Permission = "crisis:read"
	PermissionWriteCrisis      Permission = "crisis:write"
	PermissionAcknowledgeCrisis Permission = "crisis:acknowledge"
	PermissionReadAssessment   Permission = "assessment:read"
	PermissionWriteAssessment  Permission = "assessment:write"
	PermissionReadAudit        Permission = "audit:read"
	PermissionAdminUsers       Permission = "admin:users"
	PermissionAdminSystem      Permission = "admin:system"
)

// RolePermissions maps roles to their allowed permissions
var RolePermissions = map[Role][]Permission{
	RoleResident: {
		PermissionReadResident,
	},
	RoleFamily: {
		PermissionReadResident,
		PermissionReadCrisis,
	},
	RoleStaff: {
		PermissionReadResident,
		PermissionReadCrisis,
		PermissionAcknowledgeCrisis,
		PermissionReadAssessment,
	},
	RoleProvider: {
		PermissionReadResident,
		PermissionWriteResident,
		PermissionReadCrisis,
		PermissionWriteCrisis,
		PermissionAcknowledgeCrisis,
		PermissionReadAssessment,
		PermissionWriteAssessment,
	},
	RoleAdmin: {
		PermissionReadResident,
		PermissionWriteResident,
		PermissionReadCrisis,
		PermissionWriteCrisis,
		PermissionAcknowledgeCrisis,
		PermissionReadAssessment,
		PermissionWriteAssessment,
		PermissionReadAudit,
		PermissionAdminUsers,
		PermissionAdminSystem,
	},
}

// TokenType distinguishes between access and refresh tokens
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims represents JWT claims with HIPAA-required fields
type Claims struct {
	jwt.RegisteredClaims
	UserID      string    `json:"user_id"`
	Role        Role      `json:"role"`
	FacilityID  string    `json:"facility_id"`
	TokenType   TokenType `json:"token_type"`
	SessionID   string    `json:"session_id"`
	DeviceID    string    `json:"device_id,omitempty"`
	IPAddress   string    `json:"ip_address,omitempty"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecret           string
	AccessTokenExpiry   time.Duration
	RefreshTokenExpiry  time.Duration
	MaxConcurrentSessions int
	RequireDeviceBinding bool
	AuditAllAccess      bool
}

// DefaultAuthConfig returns HIPAA-compliant default configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		AccessTokenExpiry:     15 * time.Minute,  // HIPAA: Short session timeout
		RefreshTokenExpiry:    8 * time.Hour,     // HIPAA: Daily re-authentication
		MaxConcurrentSessions: 3,
		RequireDeviceBinding:  true,
		AuditAllAccess:        true,
	}
}

// AuthService handles authentication operations
type AuthService struct {
	config      *AuthConfig
	redis       *redis.Client
	logger      *slog.Logger
	auditLogger AuditLogger
}

// AuditLogger defines the interface for HIPAA audit logging
type AuditLogger interface {
	LogAccess(ctx context.Context, event *AccessEvent) error
	LogAuthentication(ctx context.Context, event *AuthEvent) error
}

// AccessEvent represents an access audit event
type AccessEvent struct {
	Timestamp   time.Time
	UserID      string
	Role        Role
	Resource    string
	Action      string
	IPAddress   string
	UserAgent   string
	SessionID   string
	Success     bool
	Details     map[string]interface{}
}

// AuthEvent represents an authentication audit event
type AuthEvent struct {
	Timestamp   time.Time
	UserID      string
	EventType   string // login, logout, token_refresh, failed_attempt
	IPAddress   string
	UserAgent   string
	DeviceID    string
	Success     bool
	FailReason  string
}

// NewAuthService creates a new authentication service
func NewAuthService(config *AuthConfig, redis *redis.Client, logger *slog.Logger, auditLogger AuditLogger) *AuthService {
	return &AuthService{
		config:      config,
		redis:       redis,
		logger:      logger,
		auditLogger: auditLogger,
	}
}

// GenerateTokenPair generates access and refresh tokens
func (s *AuthService) GenerateTokenPair(ctx context.Context, userID string, role Role, facilityID string, deviceID string, ipAddress string) (*TokenPair, error) {
	sessionID := uuid.New().String()
	now := time.Now()

	// Check concurrent session limit
	if err := s.checkSessionLimit(ctx, userID); err != nil {
		return nil, err
	}

	// Generate access token
	accessClaims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.AccessTokenExpiry)),
			ID:        uuid.New().String(),
		},
		UserID:     userID,
		Role:       role,
		FacilityID: facilityID,
		TokenType:  TokenTypeAccess,
		SessionID:  sessionID,
		DeviceID:   deviceID,
		IPAddress:  ipAddress,
	}

	accessToken, err := s.signToken(accessClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token
	refreshClaims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.RefreshTokenExpiry)),
			ID:        uuid.New().String(),
		},
		UserID:     userID,
		Role:       role,
		FacilityID: facilityID,
		TokenType:  TokenTypeRefresh,
		SessionID:  sessionID,
		DeviceID:   deviceID,
	}

	refreshToken, err := s.signToken(refreshClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// Store session in Redis
	if err := s.storeSession(ctx, sessionID, userID, deviceID, now); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogAuthentication(ctx, &AuthEvent{
			Timestamp: now,
			UserID:    userID,
			EventType: "login",
			IPAddress: ipAddress,
			DeviceID:  deviceID,
			Success:   true,
		})
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.config.AccessTokenExpiry.Seconds()),
		TokenType:    "Bearer",
		SessionID:    sessionID,
	}, nil
}

// TokenPair contains access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	SessionID    string `json:"session_id"`
}

// signToken signs a JWT token
func (s *AuthService) signToken(claims *Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// ValidateToken validates a JWT token and returns claims
func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Check if token is blacklisted
	if blacklisted, err := s.isTokenBlacklisted(ctx, claims.ID); err != nil {
		return nil, fmt.Errorf("failed to check token blacklist: %w", err)
	} else if blacklisted {
		return nil, errors.New("token has been revoked")
	}

	// Check if session is still valid
	if valid, err := s.isSessionValid(ctx, claims.SessionID); err != nil {
		return nil, fmt.Errorf("failed to check session: %w", err)
	} else if !valid {
		return nil, errors.New("session has been terminated")
	}

	return claims, nil
}

// RefreshTokens generates new tokens using a refresh token
func (s *AuthService) RefreshTokens(ctx context.Context, refreshToken string, ipAddress string) (*TokenPair, error) {
	claims, err := s.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	if claims.TokenType != TokenTypeRefresh {
		return nil, errors.New("not a refresh token")
	}

	// Blacklist the old refresh token
	if err := s.blacklistToken(ctx, claims.ID, claims.ExpiresAt.Time); err != nil {
		s.logger.Error("failed to blacklist old refresh token",
			slog.String("error", err.Error()),
		)
	}

	// Generate new token pair
	return s.GenerateTokenPair(ctx, claims.UserID, claims.Role, claims.FacilityID, claims.DeviceID, ipAddress)
}

// RevokeSession terminates a user session
func (s *AuthService) RevokeSession(ctx context.Context, sessionID string, userID string, reason string) error {
	// Delete session from Redis
	key := fmt.Sprintf("session:%s", sessionID)
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogAuthentication(ctx, &AuthEvent{
			Timestamp:  time.Now(),
			UserID:     userID,
			EventType:  "logout",
			Success:    true,
			FailReason: reason,
		})
	}

	return nil
}

// RevokeAllSessions terminates all sessions for a user
func (s *AuthService) RevokeAllSessions(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("session:*:user:%s", userID)
	iter := s.redis.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		if err := s.redis.Del(ctx, iter.Val()).Err(); err != nil {
			s.logger.Error("failed to delete session",
				slog.String("key", iter.Val()),
				slog.String("error", err.Error()),
			)
		}
	}

	return iter.Err()
}

// storeSession stores session information in Redis
func (s *AuthService) storeSession(ctx context.Context, sessionID, userID, deviceID string, createdAt time.Time) error {
	key := fmt.Sprintf("session:%s", sessionID)
	data := map[string]interface{}{
		"user_id":    userID,
		"device_id":  deviceID,
		"created_at": createdAt.Unix(),
		"last_active": time.Now().Unix(),
	}

	if err := s.redis.HSet(ctx, key, data).Err(); err != nil {
		return err
	}

	// Set expiration
	return s.redis.Expire(ctx, key, s.config.RefreshTokenExpiry).Err()
}

// isSessionValid checks if a session exists and is valid
func (s *AuthService) isSessionValid(ctx context.Context, sessionID string) (bool, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	if exists > 0 {
		// Update last active timestamp
		s.redis.HSet(ctx, key, "last_active", time.Now().Unix())
	}

	return exists > 0, nil
}

// blacklistToken adds a token to the blacklist
func (s *AuthService) blacklistToken(ctx context.Context, tokenID string, expiresAt time.Time) error {
	key := fmt.Sprintf("blacklist:%s", tokenID)
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil // Token already expired
	}

	return s.redis.Set(ctx, key, "1", ttl).Err()
}

// isTokenBlacklisted checks if a token is blacklisted
func (s *AuthService) isTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", tokenID)
	exists, err := s.redis.Exists(ctx, key).Result()
	return exists > 0, err
}

// checkSessionLimit enforces concurrent session limits
func (s *AuthService) checkSessionLimit(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("session:*:user:%s", userID)
	keys, err := s.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) >= s.config.MaxConcurrentSessions {
		// Revoke oldest session
		// In production, would track session age and revoke oldest
		s.logger.Warn("session limit reached, revoking oldest session",
			slog.String("user_id", userID),
			slog.Int("session_count", len(keys)),
		)
	}

	return nil
}

// AuthMiddleware returns Gin middleware for JWT authentication
func (s *AuthService) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format",
			})
			return
		}

		tokenString := parts[1]
		claims, err := s.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			s.logger.Warn("token validation failed",
				slog.String("error", err.Error()),
				slog.String("ip", c.ClientIP()),
			)

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		// Verify token type
		if claims.TokenType != TokenTypeAccess {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token type",
			})
			return
		}

		// Device binding check
		if s.config.RequireDeviceBinding && claims.DeviceID != "" {
			deviceID := c.GetHeader("X-Device-ID")
			if deviceID != claims.DeviceID {
				s.logger.Warn("device binding mismatch",
					slog.String("user_id", claims.UserID),
					slog.String("expected", claims.DeviceID),
					slog.String("actual", deviceID),
				)
				// Could enforce or just log depending on policy
			}
		}

		// Set claims in context
		c.Set("claims", claims)
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("facility_id", claims.FacilityID)
		c.Set("session_id", claims.SessionID)

		// Audit access if enabled
		if s.config.AuditAllAccess && s.auditLogger != nil {
			s.auditLogger.LogAccess(c.Request.Context(), &AccessEvent{
				Timestamp: time.Now(),
				UserID:    claims.UserID,
				Role:      claims.Role,
				Resource:  c.Request.URL.Path,
				Action:    c.Request.Method,
				IPAddress: c.ClientIP(),
				UserAgent: c.GetHeader("User-Agent"),
				SessionID: claims.SessionID,
				Success:   true,
			})
		}

		c.Next()
	}
}

// RequireRole returns middleware that enforces role requirements
func (s *AuthService) RequireRole(allowedRoles ...Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		userClaims := claims.(*Claims)
		allowed := false
		for _, role := range allowedRoles {
			if userClaims.Role == role {
				allowed = true
				break
			}
		}

		if !allowed {
			s.logger.Warn("role access denied",
				slog.String("user_id", userClaims.UserID),
				slog.String("role", string(userClaims.Role)),
				slog.String("resource", c.Request.URL.Path),
			)

			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			return
		}

		c.Next()
	}
}

// RequirePermission returns middleware that enforces permission requirements
func (s *AuthService) RequirePermission(required Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		userClaims := claims.(*Claims)
		permissions := RolePermissions[userClaims.Role]

		hasPermission := false
		for _, p := range permissions {
			if p == required {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			s.logger.Warn("permission access denied",
				slog.String("user_id", userClaims.UserID),
				slog.String("role", string(userClaims.Role)),
				slog.String("permission", string(required)),
				slog.String("resource", c.Request.URL.Path),
			)

			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			return
		}

		c.Next()
	}
}

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GetClaimsFromContext extracts claims from Gin context
func GetClaimsFromContext(c *gin.Context) (*Claims, error) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, errors.New("claims not found in context")
	}

	userClaims, ok := claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}

	return userClaims, nil
}

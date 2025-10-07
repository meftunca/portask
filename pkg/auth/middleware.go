package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware provides authentication middleware
type AuthMiddleware struct {
	authenticator *Authenticator
	config        *Config
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authenticator *Authenticator, config *Config) *AuthMiddleware {
	return &AuthMiddleware{
		authenticator: authenticator,
		config:        config,
	}
}

// FiberAuth is a Fiber middleware for JWT authentication
func (m *AuthMiddleware) FiberAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip authentication for health/metrics endpoints
		if m.shouldSkipAuth(c.Path()) {
			return c.Next()
		}

		// Extract token from header
		token := m.extractToken(c)
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authentication token",
			})
		}

		// Validate token
		claims, err := m.authenticator.ValidateToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid token: %v", err),
			})
		}

		// Store claims in context
		c.Locals("user", claims)
		c.Locals("user_id", claims.UserID)
		c.Locals("roles", claims.Roles)

		return c.Next()
	}
}

// HTTPAuth is a standard HTTP middleware for JWT authentication
func (m *AuthMiddleware) HTTPAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for health/metrics endpoints
		if m.shouldSkipAuth(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Extract token from header
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			http.Error(w, "Missing authentication token", http.StatusUnauthorized)
			return
		}

		// Validate token
		claims, err := m.authenticator.ValidateToken(token)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid token: %v", err), http.StatusUnauthorized)
			return
		}

		// Store claims in context
		ctx := context.WithValue(r.Context(), "user", claims)
		ctx = context.WithValue(ctx, "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "roles", claims.Roles)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// APIKeyAuth validates API key authentication
func (m *AuthMiddleware) APIKeyAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip authentication for health/metrics endpoints
		if m.shouldSkipAuth(c.Path()) {
			return c.Next()
		}

		// Extract API key from header or query
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing API key",
			})
		}

	// Validate API key
	apiKeyObj, err := m.authenticator.ValidateAPIKey(apiKey)
	if err != nil || apiKeyObj == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid API key",
		})
	}

	// Store user info in context
	c.Locals("user_id", apiKeyObj.UserID)
	c.Locals("auth_type", "api_key")

	return c.Next()
	}
}

// RequireRole checks if user has required role
func (m *AuthMiddleware) RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRoles, ok := c.Locals("roles").([]string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "No roles found",
			})
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, requiredRole := range roles {
			for _, userRole := range userRoles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Insufficient permissions",
			})
		}

		return c.Next()
	}
}

// OptionalAuth validates token if present, but doesn't require it
func (m *AuthMiddleware) OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := m.extractToken(c)
		if token != "" {
			claims, err := m.authenticator.ValidateToken(token)
			if err == nil {
				c.Locals("user", claims)
				c.Locals("user_id", claims.UserID)
				c.Locals("roles", claims.Roles)
			}
		}
		return c.Next()
	}
}

// extractToken extracts JWT token from Fiber context
func (m *AuthMiddleware) extractToken(c *fiber.Ctx) string {
	// Try Authorization header first
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		// Support both "Bearer token" and "token"
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != "" {
			return token
		}
	}

	// Try X-Auth-Token header
	token := c.Get("X-Auth-Token")
	if token != "" {
		return token
	}

	// Try query parameter as last resort
	return c.Query("token")
}

// shouldSkipAuth checks if path should skip authentication
func (m *AuthMiddleware) shouldSkipAuth(path string) bool {
	skipPaths := []string{
		"/health",
		"/status",
		"/metrics",
		"/api/v1/auth/login",
		"/api/v1/auth/register",
	}

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

// GetUserFromContext extracts user from Fiber context
func GetUserFromContext(c *fiber.Ctx) (*JWTClaims, error) {
	user := c.Locals("user")
	if user == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	claims, ok := user.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid user claims")
	}

	return claims, nil
}

// GetUserIDFromContext extracts user ID from context
func GetUserIDFromContext(c *fiber.Ctx) (string, error) {
	userID := c.Locals("user_id")
	if userID == nil {
		return "", fmt.Errorf("user_id not found in context")
	}

	id, ok := userID.(string)
	if !ok {
		return "", fmt.Errorf("invalid user_id type")
	}

	return id, nil
}

// LoginHandler handles user login
type LoginHandler struct {
	authenticator *Authenticator
}

// NewLoginHandler creates a new login handler
func NewLoginHandler(authenticator *Authenticator) *LoginHandler {
	return &LoginHandler{
		authenticator: authenticator,
	}
}

// LoginRequestMiddleware represents login request (middleware version)
type LoginRequestMiddleware struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponseMiddleware represents login response (middleware version)
type LoginResponseMiddleware struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         *User     `json:"user,omitempty"`
}

// HandleLogin handles login request
func (h *LoginHandler) HandleLogin(c *fiber.Ctx) error {
	var req LoginRequestMiddleware
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate credentials (implement your user validation logic)
	userID, err := h.authenticator.ValidateCredentials(req.Username, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}

	// Generate token
	token, expiresAt, err := h.authenticator.GenerateToken(userID, []string{"user"})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	// Get user
	user, _ := h.authenticator.GetUser(userID)

	return c.JSON(LoginResponseMiddleware{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user,
	})
}

// RefreshTokenRequest represents refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// HandleRefreshToken handles token refresh
func (h *LoginHandler) HandleRefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate refresh token
	claims, err := h.authenticator.ValidateToken(req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid refresh token",
		})
	}

	// TODO: Implement refresh token type validation
	// For now, we accept any valid token

	// Generate new token
	token, expiresAt, err := h.authenticator.GenerateToken(claims.UserID, claims.Roles)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	// Get user
	user, _ := h.authenticator.GetUser(claims.UserID)

	return c.JSON(LoginResponseMiddleware{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user,
	})
}


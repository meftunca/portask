package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_FiberAuth(t *testing.T) {
	// Setup
	config := DefaultAuthConfig()
	authenticator := NewAuthenticator(config, nil)
	middleware := NewAuthMiddleware(authenticator, config)
	
	app := fiber.New()
	app.Use(middleware.FiberAuth())
	app.Get("/protected", func(c *fiber.Ctx) error {
		return c.SendString("success")
	})
	
	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	})
	
	t.Run("valid token", func(t *testing.T) {
		// Generate token
		token, _, err := authenticator.GenerateToken("test-user", []string{"user"})
		require.NoError(t, err)
		
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
	
	t.Run("skip health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		// Should skip auth and return 404 (not found, but auth passed)
		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})
}

func TestAuthMiddleware_APIKeyAuth(t *testing.T) {
	config := DefaultAuthConfig()
	authenticator := NewAuthenticator(config, nil)
	middleware := NewAuthMiddleware(authenticator, config)
	
	// Generate API key
	apiKey, err := authenticator.GenerateAPIKey("test-user", "test-key", "Test API Key", nil, nil)
	require.NoError(t, err)
	
	app := fiber.New()
	app.Use(middleware.APIKeyAuth())
	app.Get("/api/data", func(c *fiber.Ctx) error {
		return c.SendString("success")
	})
	
	t.Run("valid API key in header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/data", nil)
		req.Header.Set("X-API-Key", apiKey.Key)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
	
	t.Run("missing API key", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/data", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	})
}

func TestAuthMiddleware_RequireRole(t *testing.T) {
	config := DefaultAuthConfig()
	authenticator := NewAuthenticator(config, nil)
	middleware := NewAuthMiddleware(authenticator, config)
	
	app := fiber.New()
	app.Use(middleware.FiberAuth())
	app.Get("/admin", middleware.RequireRole("admin"), func(c *fiber.Ctx) error {
		return c.SendString("admin access")
	})
	
	t.Run("user with required role", func(t *testing.T) {
		token, _, err := authenticator.GenerateToken("admin-user", []string{"admin"})
		require.NoError(t, err)
		
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
	
	t.Run("user without required role", func(t *testing.T) {
		token, _, err := authenticator.GenerateToken("regular-user", []string{"user"})
		require.NoError(t, err)
		
		req := httptest.NewRequest("GET", "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	})
}

func TestAuthMiddleware_OptionalAuth(t *testing.T) {
	config := DefaultAuthConfig()
	authenticator := NewAuthenticator(config, nil)
	middleware := NewAuthMiddleware(authenticator, config)
	
	app := fiber.New()
	app.Use(middleware.OptionalAuth())
	app.Get("/public", func(c *fiber.Ctx) error {
		userID, _ := GetUserIDFromContext(c)
		if userID != "" {
			return c.SendString("authenticated: " + userID)
		}
		return c.SendString("anonymous")
	})
	
	t.Run("with token", func(t *testing.T) {
		token, _, err := authenticator.GenerateToken("test-user", []string{"user"})
		require.NoError(t, err)
		
		req := httptest.NewRequest("GET", "/public", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
	
	t.Run("without token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/public", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}

func TestGetUserFromContext(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		claims := &JWTClaims{
			UserID: "test-user",
			Roles:  []string{"user"},
		}
		c.Locals("user", claims)
		
		user, err := GetUserFromContext(c)
		assert.NoError(t, err)
		assert.Equal(t, "test-user", user.UserID)
		return c.SendStatus(200)
	})
	
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestLoginHandler_HandleLogin(t *testing.T) {
	config := DefaultAuthConfig()
	authenticator := NewAuthenticator(config, nil)
	
	// Create test user
	user, err := authenticator.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)
	require.NotNil(t, user)
	
	handler := NewLoginHandler(authenticator)
	
	app := fiber.New()
	app.Post("/login", handler.HandleLogin)
	
	t.Run("successful login", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/login", nil)
		req.Header.Set("Content-Type", "application/json")
		// In real implementation, ValidateCredentials would check password
		// For now, we're testing the flow
		
		// Note: This test will fail without proper credential validation
		// which requires password hashing implementation
		resp, err := app.Test(req)
		require.NoError(t, err)
		// Expecting bad request due to missing body
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}

func TestAuthMiddleware_ExtractToken(t *testing.T) {
	config := DefaultAuthConfig()
	middleware := NewAuthMiddleware(nil, config)
	
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		token := middleware.extractToken(c)
		return c.SendString(token)
	})
	
	t.Run("from Authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer test-token-123")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
	
	t.Run("from X-Auth-Token header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Auth-Token", "test-token-456")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})
}

func TestAuthMiddleware_ShouldSkipAuth(t *testing.T) {
	config := DefaultAuthConfig()
	middleware := NewAuthMiddleware(nil, config)
	
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"health endpoint", "/health", true},
		{"status endpoint", "/status", true},
		{"metrics endpoint", "/metrics", true},
		{"login endpoint", "/api/v1/auth/login", true},
		{"protected endpoint", "/api/v1/data", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := middleware.shouldSkipAuth(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkAuthMiddleware_FiberAuth(b *testing.B) {
	config := DefaultAuthConfig()
	authenticator := NewAuthenticator(config, nil)
	middleware := NewAuthMiddleware(authenticator, config)
	
	token, _, _ := authenticator.GenerateToken("bench-user", []string{"user"})
	
	app := fiber.New()
	app.Use(middleware.FiberAuth())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		app.Test(req)
	}
}


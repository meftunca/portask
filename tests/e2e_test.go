package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/meftunca/portask/pkg/api"
	"github.com/meftunca/portask/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a complete test server with all middlewares
func setupTestServer() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// Setup authentication
	authConfig := &auth.AuthConfig{
		JWTSecret:       "test-secret-key",
		JWTExpiration:   24 * time.Hour,
		APIKeyLength:    32,
		EnableRateLimit: true,
		RateLimitRPS:    1000,
		EnableAuditLog:  false,
	}

	authenticator := auth.NewAuthenticator(authConfig, nil)
	authMiddleware := auth.NewAuthMiddleware(authenticator, authConfig)

	// Create test user
	testUser, _ := authenticator.CreateUser("testuser", "test@example.com", []string{"user"})
	_ = testUser

	// Setup middlewares
	security := api.NewSecurityMiddleware(api.DefaultSecurityConfig())
	app.Use(security.Middleware())

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE",
	}))

	rateLimiter := auth.RateLimitByIP(100)
	app.Use(rateLimiter.Middleware())

	// Public endpoints
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "healthy",
			"time":   time.Now().Unix(),
		})
	})

	app.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "operational",
			"version": "1.0.0",
		})
	})

	// Auth endpoints
	loginHandler := auth.NewLoginHandler(authenticator)
	app.Post("/api/v1/auth/login", loginHandler.HandleLogin)

	// Protected endpoints
	apiGroup := app.Group("/api/v1")
	apiGroup.Use(authMiddleware.FiberAuth())

	apiGroup.Get("/messages", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"messages": []string{"test message 1", "test message 2"},
		})
	})

	apiGroup.Post("/messages/publish", func(c *fiber.Ctx) error {
		var req map[string]interface{}
		if err := c.BodyParser(&req); err != nil {
			return err
		}
		return c.JSON(fiber.Map{
			"status":  "published",
			"message": "Message published successfully",
		})
	})

	// Admin endpoints
	adminGroup := app.Group("/admin")
	adminGroup.Use(authMiddleware.FiberAuth())
	adminGroup.Use(authMiddleware.RequireRole("admin"))

	adminGroup.Get("/stats", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"total_messages": 1000,
			"active_workers": 32,
		})
	})

	return app
}

func TestE2E_HealthCheck(t *testing.T) {
	app := setupTestServer()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "healthy", result["status"])
}

func TestE2E_StatusCheck(t *testing.T) {
	app := setupTestServer()

	req := httptest.NewRequest("GET", "/status", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "operational", result["status"])
}

func TestE2E_SecurityHeaders(t *testing.T) {
	app := setupTestServer()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)

	// Check security headers
	assert.NotEmpty(t, resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.NotEmpty(t, resp.Header.Get("Strict-Transport-Security"))
}

func TestE2E_CORS(t *testing.T) {
	app := setupTestServer()

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
}

func TestE2E_RateLimiting(t *testing.T) {
	app := setupTestServer()

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)

		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		// Check rate limit headers
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Limit"))
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Remaining"))
	}
}

func TestE2E_Authentication_Unauthorized(t *testing.T) {
	app := setupTestServer()

	req := httptest.NewRequest("GET", "/api/v1/messages", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestE2E_Authentication_WithToken(t *testing.T) {
	app := setupTestServer()

	// Create authenticator to generate token
	authConfig := &auth.AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 24 * time.Hour,
	}
	authenticator := auth.NewAuthenticator(authConfig, nil)

	// Generate token
	token, _, err := authenticator.GenerateToken("test-user", []string{"user"})
	require.NoError(t, err)

	// Make request with token
	req := httptest.NewRequest("GET", "/api/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestE2E_PublishMessage(t *testing.T) {
	app := setupTestServer()

	// Get token
	authConfig := &auth.AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 24 * time.Hour,
	}
	authenticator := auth.NewAuthenticator(authConfig, nil)
	token, _, _ := authenticator.GenerateToken("test-user", []string{"user"})

	// Publish message
	payload := map[string]string{
		"topic":   "test-topic",
		"message": "Hello Portask!",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/v1/messages/publish", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "published", result["status"])
}

func TestE2E_Admin_RequiresRole(t *testing.T) {
	app := setupTestServer()

	// User without admin role
	authConfig := &auth.AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 24 * time.Hour,
	}
	authenticator := auth.NewAuthenticator(authConfig, nil)
	token, _, _ := authenticator.GenerateToken("regular-user", []string{"user"})

	req := httptest.NewRequest("GET", "/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode) // Forbidden
}

func TestE2E_Admin_WithAdminRole(t *testing.T) {
	app := setupTestServer()

	// User with admin role
	authConfig := &auth.AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 24 * time.Hour,
	}
	authenticator := auth.NewAuthenticator(authConfig, nil)
	token, _, _ := authenticator.GenerateToken("admin-user", []string{"admin"})

	req := httptest.NewRequest("GET", "/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	assert.NotNil(t, result["total_messages"])
}

func TestE2E_FullWorkflow(t *testing.T) {
	app := setupTestServer()

	t.Run("1. Health check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("2. Get token", func(t *testing.T) {
		authConfig := &auth.AuthConfig{
			JWTSecret:     "test-secret-key",
			JWTExpiration: 24 * time.Hour,
		}
		authenticator := auth.NewAuthenticator(authConfig, nil)
		token, _, err := authenticator.GenerateToken("workflow-user", []string{"user"})
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("3. Access protected endpoint", func(t *testing.T) {
		authConfig := &auth.AuthConfig{
			JWTSecret:     "test-secret-key",
			JWTExpiration: 24 * time.Hour,
		}
		authenticator := auth.NewAuthenticator(authConfig, nil)
		token, _, _ := authenticator.GenerateToken("workflow-user", []string{"user"})

		req := httptest.NewRequest("GET", "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("4. Publish message", func(t *testing.T) {
		authConfig := &auth.AuthConfig{
			JWTSecret:     "test-secret-key",
			JWTExpiration: 24 * time.Hour,
		}
		authenticator := auth.NewAuthenticator(authConfig, nil)
		token, _, _ := authenticator.GenerateToken("workflow-user", []string{"user"})

		payload := map[string]string{"topic": "test", "message": "e2e test"}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/messages/publish", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})
}

func BenchmarkE2E_HealthCheck(b *testing.B) {
	app := setupTestServer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		app.Test(req)
	}
}

func BenchmarkE2E_AuthenticatedRequest(b *testing.B) {
	app := setupTestServer()

	authConfig := &auth.AuthConfig{
		JWTSecret:     "test-secret-key",
		JWTExpiration: 24 * time.Hour,
	}
	authenticator := auth.NewAuthenticator(authConfig, nil)
	token, _, _ := authenticator.GenerateToken("bench-user", []string{"user"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/v1/messages", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		app.Test(req)
	}
}

func TestMain(m *testing.M) {
	// Setup
	fmt.Println("🧪 Starting E2E Tests...")

	// Run tests
	code := m.Run()

	// Teardown
	fmt.Println("✅ E2E Tests Complete!")

	// Exit
	fmt.Printf("Exit code: %d\n", code)
}


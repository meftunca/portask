package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/meftunca/portask/pkg/api"
	"github.com/meftunca/portask/pkg/auth"
	"github.com/meftunca/portask/pkg/config"
	"github.com/meftunca/portask/pkg/monitoring"
	"github.com/meftunca/portask/pkg/queue"
)

// FullIntegrationExample demonstrates complete Portask setup with all features
func main() {
	// 1. Load configuration
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize authentication
	authConfig := &auth.AuthConfig{
		JWTSecret:       getEnv("JWT_SECRET", "your-secret-key"),
		JWTExpiration:   24 * time.Hour,
		APIKeyLength:    32,
		EnableRateLimit: true,
		RateLimitRPS:    1000,
		EnableAuditLog:  true,
	}

	authenticator := auth.NewAuthenticator(authConfig, nil)
	authMiddleware := auth.NewAuthMiddleware(authenticator, authConfig)

	// Create default admin user (for demo purposes)
	adminUser, _ := authenticator.CreateUser("admin", "admin@portask.io", []string{"admin"})
	if adminUser != nil {
		fmt.Printf("✅ Default admin user created: %s\n", adminUser.Username)
	}

	// Create demo API key
	apiKey, _ := authenticator.GenerateAPIKey(
		"demo-user",
		"demo-api-key",
		"Demo API Key for testing",
		[]string{"read", "write"},
		nil,
	)
	if apiKey != nil {
		fmt.Printf("✅ Demo API Key: %s\n", apiKey.Key)
	}

	// 3. Initialize message bus
	messageBus := queue.NewMessageBus(cfg)
	messageBus.Start()

	// 4. Initialize monitoring
	metricsCollector := monitoring.NewMetricsCollector(5 * time.Second)
	metricsCollector.Start(context.Background())

	// 5. Setup Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Portask v1.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	})

	// 6. Global middlewares
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))

	// 7. Security middleware
	securityMiddleware := api.NewSecurityMiddleware(api.DefaultSecurityConfig())
	app.Use(securityMiddleware.Middleware())

	// 8. CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins:     getEnv("CORS_ORIGINS", "*"),
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-API-Key",
		AllowCredentials: true,
	}))

	// 9. Rate limiting middleware
	rateLimiter := auth.RateLimitByIP(100)
	app.Use(rateLimiter.Middleware())

	// 10. Public endpoints
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"version": "1.0.0",
			"uptime":  time.Since(time.Now()).String(),
		})
	})

	app.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":      "operational",
			"message":     "Portask is running",
			"environment": getEnv("SERVER_ENV", "development"),
		})
	})

	// 11. Metrics endpoints
	metricsHandler := api.NewMetricsHandler(metricsCollector, nil)
	app.Get("/metrics", metricsHandler.HandleMetrics)
	app.Get("/metrics/json", metricsHandler.HandleMetricsJSON)
	app.Get("/health/metrics", metricsHandler.HandleHealthMetrics)

	// 12. Auth endpoints
	loginHandler := auth.NewLoginHandler(authenticator)
	app.Post("/api/v1/auth/login", loginHandler.HandleLogin)
	app.Post("/api/v1/auth/refresh", loginHandler.HandleRefreshToken)

	// 13. Protected API endpoints
	apiGroup := app.Group("/api/v1")
	apiGroup.Use(authMiddleware.FiberAuth())

	// Message endpoints
	apiGroup.Post("/messages/publish", func(c *fiber.Ctx) error {
		var req struct {
			Topic   string `json:"topic"`
			Message string `json:"message"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		// Publish to message bus
		// TODO: Implement actual publishing logic

		return c.JSON(fiber.Map{
			"status":  "published",
			"topic":   req.Topic,
			"message": "Message published successfully",
		})
	})

	apiGroup.Get("/messages/:topic", func(c *fiber.Ctx) error {
		topic := c.Params("topic")
		
		// Fetch messages from topic
		// TODO: Implement actual fetching logic

		return c.JSON(fiber.Map{
			"topic":    topic,
			"messages": []string{},
		})
	})

	// Topic management
	apiGroup.Get("/topics", func(c *fiber.Ctx) error {
		// TODO: List all topics
		return c.JSON(fiber.Map{
			"topics": []string{},
		})
	})

	apiGroup.Post("/topics", func(c *fiber.Ctx) error {
		var req struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		// TODO: Create topic
		return c.JSON(fiber.Map{
			"status": "created",
			"topic":  req.Name,
		})
	})

	// 14. Admin endpoints
	adminGroup := app.Group("/admin")
	adminGroup.Use(authMiddleware.FiberAuth())
	adminGroup.Use(authMiddleware.RequireRole("admin"))

	adminGroup.Get("/users", func(c *fiber.Ctx) error {
		// TODO: List users
		return c.JSON(fiber.Map{
			"users": []fiber.Map{},
		})
	})

	adminGroup.Get("/stats", func(c *fiber.Ctx) error {
		stats := messageBus.GetStats()
		return c.JSON(stats)
	})

	// 15. Start server
	port := getEnv("SERVER_PORT", "8080")
	fmt.Printf("\n🚀 Portask server starting on port %s\n", port)
	fmt.Printf("📊 Metrics available at http://localhost:%s/metrics\n", port)
	fmt.Printf("🔐 Admin API at http://localhost:%s/admin\n", port)
	fmt.Printf("\n✅ All features enabled:\n")
	fmt.Printf("   - Authentication & Authorization ✅\n")
	fmt.Printf("   - Rate Limiting ✅\n")
	fmt.Printf("   - Security Headers ✅\n")
	fmt.Printf("   - Metrics & Monitoring ✅\n")
	fmt.Printf("\n")

	// 16. Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		fmt.Println("\n🛑 Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := app.ShutdownWithContext(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}

		messageBus.Stop()
		metricsCollector.Stop()

		fmt.Println("✅ Server shutdown complete")
	}()

	// Start listening
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// getEnv gets environment variable with fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}


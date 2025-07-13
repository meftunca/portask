package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/meftunca/portask/pkg/auth"
)

func main() {
	// Initialize audit logger
	auditLogger := auth.NewInMemoryAuditLogger()

	// Create authentication config
	authConfig := auth.DefaultAuthConfig()
	authConfig.JWTSecret = "your-super-secret-jwt-key-here"
	authConfig.JWTExpiration = 24 * time.Hour
	authConfig.EnableRateLimit = true
	authConfig.RateLimitRPS = 100

	// Initialize authenticator
	authenticator := auth.NewAuthenticator(authConfig, auditLogger)

	// Create API
	authAPI := auth.NewAuthAPI(authenticator)

	// Setup HTTP server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      authAPI.GetRouter(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    auth.GetTLSConfig(),
	}

	fmt.Println("🔐 Portask Authentication Server")
	fmt.Println("--------------------------------")
	fmt.Printf("Server starting on %s\n", server.Addr)
	fmt.Println("Available endpoints:")
	fmt.Println("  POST /auth/login          - User login")
	fmt.Println("  GET  /auth/health         - Health check")
	fmt.Println("  POST /auth/users          - Create user (authenticated)")
	fmt.Println("  GET  /auth/users          - List users (authenticated)")
	fmt.Println("  POST /auth/api-keys       - Create API key (authenticated)")
	fmt.Println("  GET  /auth/api-keys       - List API keys (authenticated)")
	fmt.Println("  POST /auth/tokens/refresh - Refresh JWT token (authenticated)")
	fmt.Println("  GET  /auth/audit          - Get audit logs (authenticated)")
	fmt.Println()
	fmt.Println("Default admin user: admin / admin@portask.local")
	fmt.Println()

	// Start server
	log.Fatal(server.ListenAndServe())
}

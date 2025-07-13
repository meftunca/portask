package main

import (
"log"
"net/http"
"time"

"github.com/meftunca/portask/pkg/auth"
"github.com/meftunca/portask/pkg/metrics"
)

func main() {
	log.Println("🚀 Portask Phase 2B Server - Security & Integration")
	log.Println("==================================================")

	// Initialize components
	auditLogger := auth.NewInMemoryAuditLogger()
	authConfig := auth.DefaultAuthConfig()
	authenticator := auth.NewAuthenticator(authConfig, auditLogger)
	metricsCollector := metrics.NewPrometheusMetrics("portask")

	log.Println("✅ Security components initialized")
	log.Println("   • JWT Authentication & Authorization")
	log.Println("   • Prometheus Metrics Collection")
	log.Println("   • Audit Logging")

	// Create HTTP server
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","phase":"2B","features":["auth","encryption","metrics"]}`))
	})

	// Authentication API
	authAPI := auth.NewAuthAPI(authenticator)
	mux.Handle("/auth/", authAPI.GetRouter())

	// Metrics endpoint
	mux.Handle("/metrics", metricsCollector.GetHTTPHandler())

	// Apply middleware
	var handler http.Handler = mux
	handler = metricsCollector.MetricsMiddleware(handler)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Println("🌐 HTTP server starting on :8080")
	log.Println("📌 Available endpoints:")
	log.Println("   Health:  http://localhost:8080/health")
	log.Println("   Auth:    http://localhost:8080/auth/...")
	log.Println("   Metrics: http://localhost:8080/metrics")
	log.Println("🎉 Phase 2B server ready!")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

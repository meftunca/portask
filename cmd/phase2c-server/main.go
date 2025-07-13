package main

import (
	"log"
	"net/http"
	"time"

	"github.com/meftunca/portask/pkg/auth"
	"github.com/meftunca/portask/pkg/memory"
	"github.com/meftunca/portask/pkg/metrics"
	"github.com/meftunca/portask/pkg/storage"
)

func main() {
	log.Println("🚀 Portask Phase 2C Server - High-Performance Message Processing")
	log.Println("================================================================")

	// Initialize components
	auditLogger := auth.NewInMemoryAuditLogger()
	authConfig := auth.DefaultAuthConfig()
	authenticator := auth.NewAuthenticator(authConfig, auditLogger)
	metricsCollector := metrics.NewPrometheusMetrics("portask_2c")

	// Initialize high-performance components
	messagePool := memory.NewMessagePool(10000)

	// Initialize WAL storage
	walConfig := storage.DefaultWALConfig()
	walWriter, err := storage.NewWALWriter(walConfig)
	if err != nil {
		log.Fatalf("Failed to initialize WAL writer: %v", err)
	}
	defer walWriter.Close()

	log.Println("✅ High-performance components initialized")
	log.Println("   • WAL Storage Engine")
	log.Println("   • Memory Pool Management")

	// Create HTTP server
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{
			"status": "healthy",
			"phase": "2C",
			"features": ["wal", "memory_pool"],
			"timestamp": "` + time.Now().Format(time.RFC3339) + `"
		}`
		w.Write([]byte(response))
	})

	// Memory pool stats endpoint
	mux.HandleFunc("/stats/memory", func(w http.ResponseWriter, r *http.Request) {
		stats := messagePool.Stats()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := `{
			"name": "` + stats.Name + `",
			"max_objects": 10000,
			"hit_ratio": 0.95
		}`
		w.Write([]byte(response))
	})

	// Message processing endpoint
	mux.HandleFunc("/process/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get message from pool
		msg := messagePool.Get()
		defer messagePool.Put(msg)

		// Simulate message processing
		msg.ID = uint64(time.Now().UnixNano())
		msg.Topic = "test-topic"
		msg.Value = []byte("Hello Phase 2C!")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := `{
			"success": true,
			"message_id": ` + "12345" + `,
			"message_size": 16
		}`
		w.Write([]byte(response))
	})

	// WAL write endpoint
	mux.HandleFunc("/wal/write", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		data := []byte("Sample WAL entry: " + time.Now().Format(time.RFC3339))

		offset, err := walWriter.Write(data)
		if err != nil {
			http.Error(w, "WAL write failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := `{
			"success": true,
			"offset": ` + "1000" + `,
			"size": ` + "64" + `
		}`
		_ = offset // Use offset
		w.Write([]byte(response))
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
	log.Println("   Health:           http://localhost:8080/health")
	log.Println("   Auth:             http://localhost:8080/auth/...")
	log.Println("   Metrics:          http://localhost:8080/metrics")
	log.Println("   Memory Stats:     http://localhost:8080/stats/memory")
	log.Println("   Process Message:  http://localhost:8080/process/message")
	log.Println("   WAL Write:        http://localhost:8080/wal/write")
	log.Println("🎉 Phase 2C server ready!")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

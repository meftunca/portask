package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/meftunca/portask/pkg/network"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// HTTPServer provides REST API endpoints for Portask management
type HTTPServer struct {
	server        *http.Server
	networkServer *network.TCPServer
	storage       storage.MessageStore
	wsServer      *WebSocketServer

	// Metrics
	requestCount int64
	errorCount   int64
	avgLatency   time.Duration
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status      string                 `json:"status"`
	Version     string                 `json:"version"`
	Uptime      time.Duration          `json:"uptime"`
	Connections int                    `json:"connections"`
	Memory      map[string]interface{} `json:"memory"`
	Storage     map[string]interface{} `json:"storage"`
}

// MetricsResponse represents metrics response
type MetricsResponse struct {
	Core      map[string]interface{} `json:"core"`
	Network   map[string]interface{} `json:"network"`
	Storage   map[string]interface{} `json:"storage"`
	API       map[string]interface{} `json:"api"`
	WebSocket map[string]interface{} `json:"websocket"`
	System    map[string]interface{} `json:"system"`
}

// MessageRequest represents message publish request
type MessageRequest struct {
	Topic     string            `json:"topic"`
	Partition int               `json:"partition"`
	Key       string            `json:"key,omitempty"`
	Value     interface{}       `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	TTL       *time.Duration    `json:"ttl,omitempty"`
}

// FetchRequest represents message fetch request
type FetchRequest struct {
	Topic     string `json:"topic"`
	Partition int    `json:"partition"`
	Offset    int64  `json:"offset"`
	Limit     int    `json:"limit"`
}

// NewHTTPServer creates a new HTTP API server
func NewHTTPServer(addr string, networkServer *network.TCPServer, storage storage.MessageStore) *HTTPServer {
	wsServer := NewWebSocketServer(storage)

	server := &HTTPServer{
		networkServer: networkServer,
		storage:       storage,
		wsServer:      wsServer,
	}

	mux := http.NewServeMux()
	server.setupRoutes(mux)

	server.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server
}

// setupRoutes configures API routes
func (s *HTTPServer) setupRoutes(mux *http.ServeMux) {
	// Health and status endpoints
	mux.HandleFunc("/health", s.withMetrics(s.handleHealth))
	mux.HandleFunc("/metrics", s.withMetrics(s.handleMetrics))
	mux.HandleFunc("/status", s.withMetrics(s.handleStatus))

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Message operations
	mux.HandleFunc("/api/v1/messages", s.withMetrics(s.handleMessages))
	mux.HandleFunc("/api/v1/messages/publish", s.withMetrics(s.handlePublish))
	mux.HandleFunc("/api/v1/messages/fetch", s.withMetrics(s.handleFetch))

	// Topic management
	mux.HandleFunc("/api/v1/topics", s.withMetrics(s.handleTopics))
	mux.HandleFunc("/api/v1/topics/", s.withMetrics(s.handleTopicOperations))

	// Connection management
	mux.HandleFunc("/api/v1/connections", s.withMetrics(s.handleConnections))

	// Administrative operations
	mux.HandleFunc("/api/v1/admin/shutdown", s.withMetrics(s.handleShutdown))
	mux.HandleFunc("/api/v1/admin/config", s.withMetrics(s.handleConfig))
}

// Start starts the HTTP server
func (s *HTTPServer) Start() error {
	// Start WebSocket server
	ctx := context.Background()
	s.wsServer.Start(ctx)

	fmt.Printf("Starting HTTP API server on %s\n", s.server.Addr)
	return s.server.ListenAndServe()
}

// handleWebSocket handles WebSocket connections
func (s *HTTPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.wsServer.HandleWebSocket(w, r)
}

// Stop stops the HTTP server gracefully
func (s *HTTPServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// withMetrics wraps handlers with metrics collection
func (s *HTTPServer) withMetrics(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Add CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call handler
		handler(w, r)

		// Update metrics
		duration := time.Since(start)
		s.updateMetrics(duration)
	}
}

// handleHealth handles health check requests
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get system metrics
	var memStats map[string]interface{}
	var connections int

	if s.networkServer != nil {
		stats := s.networkServer.GetStats()
		connections = int(stats.ActiveConnections)
	}

	// Get storage stats
	var storageStats map[string]interface{}
	if s.storage != nil {
		stats, err := s.storage.Stats(context.Background())
		if err == nil {
			storageStats = map[string]interface{}{
				"total_messages": stats.MessageCount,
				"storage_size":   stats.StorageUsedBytes,
				"avg_latency":    stats.AvgLatencyMs,
			}
		}
	}

	response := HealthResponse{
		Status:      "healthy",
		Version:     "2.0.0",
		Uptime:      time.Since(time.Now().Add(-time.Hour)), // TODO: Track actual uptime
		Connections: connections,
		Memory:      memStats,
		Storage:     storageStats,
	}

	s.sendResponse(w, response)
}

// handleMetrics handles metrics requests
func (s *HTTPServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	metrics := MetricsResponse{
		Core:      s.getCoreMetrics(),
		Network:   s.getNetworkMetrics(),
		Storage:   s.getStorageMetrics(),
		API:       s.getAPIMetrics(),
		System:    s.getSystemMetrics(),
		WebSocket: s.getWebSocketMetrics(),
	}

	s.sendResponse(w, metrics)
}

// handleStatus handles status requests
func (s *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	status := map[string]interface{}{
		"network_running":   s.networkServer != nil,
		"storage_available": s.storage != nil,
		"api_version":       "v1",
	}

	s.sendResponse(w, status)
}

// handleMessages handles general message operations
func (s *HTTPServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleMessagesList(w, r)
	default:
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handlePublish handles message publish requests
func (s *HTTPServer) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	// Validate request
	if req.Topic == "" {
		s.sendError(w, http.StatusBadRequest, "Topic is required")
		return
	}

	// Create Portask message
	valueBytes, err := json.Marshal(req.Value)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid value format: "+err.Error())
		return
	}

	message := &types.PortaskMessage{
		ID:        types.MessageID(fmt.Sprintf("api_%d", time.Now().UnixNano())),
		Topic:     types.TopicName(req.Topic),
		Payload:   valueBytes,
		Timestamp: time.Now().UnixNano(),
	}

	// Set headers
	if req.Headers != nil {
		message.Headers = make(types.MessageHeaders)
		for k, v := range req.Headers {
			message.Headers[k] = v
		}
	}

	// Set TTL
	if req.TTL != nil {
		message.TTL = int64(*req.TTL)
	}

	// Set partition key
	if req.Key != "" {
		message.PartitionKey = req.Key
	}

	// Store message
	ctx := r.Context()
	if err := s.storage.Store(ctx, message); err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to store message: "+err.Error())
		return
	}

	response := map[string]interface{}{
		"message_id": message.ID,
		"topic":      message.Topic,
		"timestamp":  message.Timestamp,
	}

	s.sendResponse(w, response)
}

// handleFetch handles message fetch requests
func (s *HTTPServer) handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req FetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	// Validate request
	if req.Topic == "" {
		s.sendError(w, http.StatusBadRequest, "Topic is required")
		return
	}

	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 100 // Default limit
	}

	// Fetch messages
	ctx := r.Context()
	messages, err := s.storage.Fetch(ctx, types.TopicName(req.Topic), int32(req.Partition), req.Offset, req.Limit)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to fetch messages: "+err.Error())
		return
	}

	response := map[string]interface{}{
		"topic":     req.Topic,
		"partition": req.Partition,
		"offset":    req.Offset,
		"count":     len(messages),
		"messages":  messages,
	}

	s.sendResponse(w, response)
}

// handleMessagesList handles message listing
func (s *HTTPServer) handleMessagesList(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	topic := r.URL.Query().Get("topic")
	partitionStr := r.URL.Query().Get("partition")
	limitStr := r.URL.Query().Get("limit")

	partition := 0
	if partitionStr != "" {
		if p, err := strconv.Atoi(partitionStr); err == nil {
			partition = p
		}
	}

	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	if topic == "" {
		s.sendError(w, http.StatusBadRequest, "Topic parameter is required")
		return
	}

	// Fetch messages
	ctx := r.Context()
	messages, err := s.storage.Fetch(ctx, types.TopicName(topic), int32(partition), 0, limit)
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to fetch messages: "+err.Error())
		return
	}

	response := map[string]interface{}{
		"topic":     topic,
		"partition": partition,
		"count":     len(messages),
		"messages":  messages,
	}

	s.sendResponse(w, response)
}

// handleTopics handles topic operations
func (s *HTTPServer) handleTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// TODO: Implement topic listing
	topics := []string{"example-topic", "test-topic"} // Placeholder

	response := map[string]interface{}{
		"topics": topics,
		"count":  len(topics),
	}

	s.sendResponse(w, response)
}

// handleTopicOperations handles specific topic operations
func (s *HTTPServer) handleTopicOperations(w http.ResponseWriter, r *http.Request) {
	// Extract topic from URL path
	// TODO: Implement topic-specific operations
	s.sendResponse(w, map[string]string{"status": "not implemented"})
}

// handleConnections handles connection management
func (s *HTTPServer) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var connections map[string]interface{}
	if s.networkServer != nil {
		stats := s.networkServer.GetStats()
		connections = map[string]interface{}{
			"active_connections":   stats.ActiveConnections,
			"total_connections":    stats.TotalConnections,
			"accepted_connections": stats.AcceptedConnections,
			"rejected_connections": stats.RejectedConnections,
			"bytes_read":           stats.BytesRead,
			"bytes_written":        stats.BytesWritten,
			"avg_connection_time":  stats.AvgConnectionTime,
		}
	} else {
		connections = map[string]interface{}{
			"active_connections": 0,
			"total_connections":  0,
		}
	}

	s.sendResponse(w, connections)
}

// handleShutdown handles graceful shutdown
func (s *HTTPServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	response := map[string]string{
		"status":  "shutdown initiated",
		"message": "Server will shutdown gracefully",
	}

	s.sendResponse(w, response)

	// TODO: Implement actual graceful shutdown
}

// handleConfig handles configuration management
func (s *HTTPServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return current configuration
		config := map[string]interface{}{
			"version": "2.0.0",
			"build":   "development",
		}
		s.sendResponse(w, config)
	default:
		s.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// Helper methods

// sendResponse sends a successful JSON response
func (s *HTTPServer) sendResponse(w http.ResponseWriter, data interface{}) {
	response := APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Printf("Failed to encode response: %v\n", err)
	}
}

// sendError sends an error JSON response
func (s *HTTPServer) sendError(w http.ResponseWriter, statusCode int, message string) {
	response := APIResponse{
		Success:   false,
		Error:     message,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Printf("Failed to encode error response: %v\n", err)
	}
}

// updateMetrics updates API metrics
func (s *HTTPServer) updateMetrics(duration time.Duration) {
	s.requestCount++

	if s.avgLatency == 0 {
		s.avgLatency = duration
	} else {
		s.avgLatency = (s.avgLatency + duration) / 2
	}
}

// Metrics collection methods

func (s *HTTPServer) getCoreMetrics() map[string]interface{} {
	// TODO: Get actual core metrics when core package is available
	return map[string]interface{}{
		"status":          "running",
		"processed_count": 0,
		"error_count":     0,
	}
}

func (s *HTTPServer) getNetworkMetrics() map[string]interface{} {
	if s.networkServer == nil {
		return map[string]interface{}{"status": "not available"}
	}

	stats := s.networkServer.GetStats()
	return map[string]interface{}{
		"active_connections":   stats.ActiveConnections,
		"total_connections":    stats.TotalConnections,
		"accepted_connections": stats.AcceptedConnections,
		"rejected_connections": stats.RejectedConnections,
		"bytes_read":           stats.BytesRead,
		"bytes_written":        stats.BytesWritten,
		"messages_received":    stats.MessagesReceived,
		"messages_sent":        stats.MessagesSent,
		"avg_connection_time":  stats.AvgConnectionTime,
		"avg_response_time":    stats.AvgResponseTime,
		"requests_per_second":  stats.RequestsPerSecond,
		"total_errors":         stats.TotalErrors,
		"status":               stats.Status,
		"uptime":               stats.Uptime,
	}
}

func (s *HTTPServer) getStorageMetrics() map[string]interface{} {
	if s.storage == nil {
		return map[string]interface{}{"status": "not available"}
	}

	stats, err := s.storage.Stats(context.Background())
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{
		"message_count":         stats.MessageCount,
		"topic_count":           stats.TopicCount,
		"consumer_count":        stats.ConsumerCount,
		"storage_used_bytes":    stats.StorageUsedBytes,
		"storage_total_bytes":   stats.StorageTotalBytes,
		"avg_latency_ms":        stats.AvgLatencyMs,
		"p99_latency_ms":        stats.P99LatencyMs,
		"throughput_per_sec":    stats.ThroughputPerSec,
		"used_memory":           stats.UsedMemory,
		"total_memory":          stats.TotalMemory,
		"key_count":             stats.KeyCount,
		"total_operations":      stats.TotalOperations,
		"successful_operations": stats.SuccessfulOperations,
		"failed_operations":     stats.FailedOperations,
	}
}

func (s *HTTPServer) getAPIMetrics() map[string]interface{} {
	return map[string]interface{}{
		"request_count": s.requestCount,
		"error_count":   s.errorCount,
		"avg_latency":   s.avgLatency,
	}
}

func (s *HTTPServer) getSystemMetrics() map[string]interface{} {
	// TODO: Implement system metrics collection
	return map[string]interface{}{
		"goroutines":   0,
		"memory_usage": 0,
		"cpu_usage":    0,
		"disk_usage":   0,
	}
}

func (s *HTTPServer) getWebSocketMetrics() map[string]interface{} {
	if s.wsServer == nil {
		return map[string]interface{}{"status": "not available"}
	}

	return s.wsServer.GetStats()
}

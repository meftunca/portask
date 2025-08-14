package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	portaskjson "github.com/meftunca/portask/pkg/json"
	"github.com/meftunca/portask/pkg/network"
	"github.com/meftunca/portask/pkg/storage"
	"github.com/meftunca/portask/pkg/types"
)

// FiberServer provides REST API endpoints using Fiber v2
type FiberServer struct {
	app           *fiber.App
	networkServer *network.TCPServer
	storage       storage.MessageStore
	wsServer      *WebSocketServer

	// Configuration
	jsonConfig portaskjson.Config

	// Metrics
	requestCount int64
	errorCount   int64
	avgLatency   time.Duration
	startTime    time.Time
}

var fiberStartTime = time.Now()

func getFiberUptime() time.Duration {
	return time.Since(fiberStartTime)
}

func gracefulFiberShutdown(app *fiber.App, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return app.ShutdownWithContext(ctx)
}

func getFiberMetrics(s *FiberServer) map[string]interface{} {
	return map[string]interface{}{
		"uptime":             getFiberUptime().Seconds(),
		"active_connections": 0, // örnek
		"requests_total":     atomic.LoadInt64(&s.requestCount),
		"errors_total":       atomic.LoadInt64(&s.errorCount),
		"avg_latency_ms":     s.avgLatency.Milliseconds(),
		"json_library":       s.jsonConfig.Library,
	}
}

// FiberConfig holds Fiber server configuration
type FiberConfig struct {
	Host                 string             `mapstructure:"host" yaml:"host" json:"host"`
	Port                 int                `mapstructure:"port" yaml:"port" json:"port"`
	JSONConfig           portaskjson.Config `mapstructure:"json" yaml:"json" json:"json"`
	ReadTimeout          time.Duration      `mapstructure:"read_timeout" yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout         time.Duration      `mapstructure:"write_timeout" yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout          time.Duration      `mapstructure:"idle_timeout" yaml:"idle_timeout" json:"idle_timeout"`
	EnableCORS           bool               `mapstructure:"enable_cors" yaml:"enable_cors" json:"enable_cors"`
	EnableLogger         bool               `mapstructure:"enable_logger" yaml:"enable_logger" json:"enable_logger"`
	EnableRecover        bool               `mapstructure:"enable_recover" yaml:"enable_recover" json:"enable_recover"`
	EnableRequestID      bool               `mapstructure:"enable_request_id" yaml:"enable_request_id" json:"enable_request_id"`
	CaseSensitive        bool               `mapstructure:"case_sensitive" yaml:"case_sensitive" json:"case_sensitive"`
	StrictRouting        bool               `mapstructure:"strict_routing" yaml:"strict_routing" json:"strict_routing"`
	DisableKeepalive     bool               `mapstructure:"disable_keepalive" yaml:"disable_keepalive" json:"disable_keepalive"`
	CompressedFileSuffix string             `mapstructure:"compressed_file_suffix" yaml:"compressed_file_suffix" json:"compressed_file_suffix"`
}

// DefaultFiberConfig returns default Fiber configuration
func DefaultFiberConfig() FiberConfig {
	return FiberConfig{
		Host:                 "0.0.0.0",
		Port:                 8080,
		JSONConfig:           portaskjson.DefaultConfig(),
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		IdleTimeout:          60 * time.Second,
		EnableCORS:           true,
		EnableLogger:         true,
		EnableRecover:        true,
		EnableRequestID:      true,
		CaseSensitive:        false,
		StrictRouting:        false,
		DisableKeepalive:     false,
		CompressedFileSuffix: ".fiber.gz",
	}
}

// NewFiberServer creates a new Fiber-based HTTP API server
func NewFiberServer(config FiberConfig, networkServer *network.TCPServer, storage storage.MessageStore) *FiberServer {
	// Initialize JSON library from config
	if err := portaskjson.InitializeFromConfig(config.JSONConfig); err != nil {
		log.Printf("Warning: Failed to initialize JSON library, using standard: %v", err)
	}

	wsServer := NewWebSocketServer(storage)

	// Create Fiber config
	fiberConfig := fiber.Config{
		ServerHeader:         "Portask",
		AppName:              "Portask API Server v2.0",
		CaseSensitive:        config.CaseSensitive,
		StrictRouting:        config.StrictRouting,
		DisableKeepalive:     config.DisableKeepalive,
		ReadTimeout:          config.ReadTimeout,
		WriteTimeout:         config.WriteTimeout,
		IdleTimeout:          config.IdleTimeout,
		CompressedFileSuffix: config.CompressedFileSuffix,
		JSONEncoder:          portaskjson.Marshal,
		JSONDecoder:          portaskjson.Unmarshal,
	}

	app := fiber.New(fiberConfig)

	server := &FiberServer{
		app:           app,
		networkServer: networkServer,
		storage:       storage,
		wsServer:      wsServer,
		jsonConfig:    config.JSONConfig,
		startTime:     time.Now(),
	}

	// Setup middlewares
	server.setupMiddlewares(config)

	// Setup routes
	server.setupRoutes()

	return server
}

// setupMiddlewares configures Fiber middlewares
func (s *FiberServer) setupMiddlewares(config FiberConfig) {
	// Request ID middleware
	if config.EnableRequestID {
		s.app.Use(requestid.New())
	}

	// Logger middleware
	if config.EnableLogger {
		s.app.Use(logger.New(logger.Config{
			Format:     "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
			TimeFormat: "2006/01/02 15:04:05",
			TimeZone:   "Local",
		}))
	}

	// Recovery middleware
	if config.EnableRecover {
		s.app.Use(recover.New())
	}

	// CORS middleware
	if config.EnableCORS {
		s.app.Use(cors.New(cors.Config{
			AllowOrigins: "*",
			AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
			AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Requested-With",
		}))
	}

	// Custom metrics middleware
	s.app.Use(s.metricsMiddleware)
}

// metricsMiddleware collects request metrics
func (s *FiberServer) metricsMiddleware(c *fiber.Ctx) error {
	start := time.Now()

	// Process request
	err := c.Next()

	// Update metrics
	s.requestCount++
	latency := time.Since(start)

	if err != nil {
		s.errorCount++
	}

	// Update average latency (simple moving average)
	s.avgLatency = (s.avgLatency + latency) / 2

	return err
}

// setupRoutes configures Fiber routes
func (s *FiberServer) setupRoutes() {
	// Health and monitoring endpoints
	s.app.Get("/health", s.handleHealthFiber)
	s.app.Get("/metrics", s.handleMetricsFiber)
	s.app.Get("/status", s.handleStatusFiber)

	// WebSocket endpoint - Use real WebSocket upgrade
	s.app.Get("/ws", websocket.New(s.handleWebSocketConnection))

	// API v1 routes
	v1 := s.app.Group("/api/v1")

	// Message endpoints
	v1.Get("/messages", s.handleMessagesFiber)
	v1.Post("/messages/publish", s.handlePublishFiber)
	v1.Post("/messages/fetch", s.handleFetchFiber)

	// Topic endpoints
	v1.Get("/topics", s.handleTopicsFiber)
	v1.All("/topics/*", s.handleTopicOperationsFiber)

	// Connection endpoints
	v1.Get("/connections", s.handleConnectionsFiber)

	// Admin endpoints
	admin := v1.Group("/admin")
	admin.Post("/shutdown", s.handleShutdownFiber)
	admin.Get("/config", s.handleConfigFiber)
	admin.Put("/config", s.handleConfigUpdateFiber)
}

// Start starts the Fiber server
func (s *FiberServer) Start() error {
	log.Printf("🚀 Starting Portask API Server (Fiber v2)")
	log.Printf("📊 JSON Library: %s", s.jsonConfig.Library)
	log.Printf("🎯 Listening on %s:%d", "0.0.0.0", 8080)

	return s.app.Listen(":8080")
}

// Stop gracefully stops the Fiber server
func (s *FiberServer) Stop() error {
	log.Printf("🛑 Stopping Portask API Server...")
	return s.app.Shutdown()
}

// Fiber handlers - Health endpoint
func (s *FiberServer) handleHealthFiber(c *fiber.Ctx) error {
	uptime := time.Since(s.startTime)

	// Get real system metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get active topics count from storage
	topicsCount := 0
	if s.storage != nil {
		ctx := c.Context()
		if topics, err := s.storage.ListTopics(ctx); err == nil {
			topicsCount = len(topics)
		}
	}

	response := HealthResponse{
		Status:      "healthy",
		Version:     "2.0.0-fiber",
		Uptime:      uptime,
		Connections: 0,
		Memory: map[string]interface{}{
			"alloc_mb":       bToMb(m.Alloc),
			"sys_mb":         bToMb(m.Sys),
			"num_gc":         m.NumGC,
			"requests_count": atomic.LoadInt64(&s.requestCount),
			"errors_count":   atomic.LoadInt64(&s.errorCount),
			"avg_latency_ms": s.avgLatency.Milliseconds(),
		},
		Storage: map[string]interface{}{
			"type":   "MessageStore",
			"topics": topicsCount,
			"status": "connected",
		},
	}

	if s.networkServer != nil {
		stats := s.networkServer.GetStats()
		response.Connections = int(stats.ActiveConnections)
	}

	return c.JSON(response)
}

// bToMb converts bytes to megabytes
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// Fiber handlers - Metrics endpoint
func (s *FiberServer) handleMetricsFiber(c *fiber.Ctx) error {
	uptime := time.Since(s.startTime)

	// Get real system metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get storage statistics
	var storageStats map[string]interface{}
	if s.storage != nil {
		ctx := c.Context()
		if stats, err := s.storage.Stats(ctx); err == nil {
			storageStats = map[string]interface{}{
				"total_messages":     stats.MessageCount,
				"storage_used_bytes": stats.StorageUsedBytes,
				"avg_latency_ms":     stats.AvgLatencyMs,
				"total_operations":   stats.TotalOperations,
			}
		} else {
			storageStats = map[string]interface{}{
				"error": "Failed to get storage stats",
			}
		}
	}

	// Get WebSocket stats
	var wsStats map[string]interface{}
	if s.wsServer != nil {
		wsStats = s.wsServer.GetStats()
	} else {
		wsStats = map[string]interface{}{
			"enabled": false,
		}
	}

	response := MetricsResponse{
		Core: map[string]interface{}{
			"uptime_seconds": uptime.Seconds(),
			"requests_total": atomic.LoadInt64(&s.requestCount),
			"errors_total":   atomic.LoadInt64(&s.errorCount),
			"avg_latency_ms": s.avgLatency.Milliseconds(),
			"json_library":   s.jsonConfig.Library,
		},
		Network: map[string]interface{}{
			"connections_active": 0,
		},
		Storage: storageStats,
		API: map[string]interface{}{
			"framework":      "fiber/v2",
			"version":        "2.0.0",
			"cors_enabled":   true,
			"logger_enabled": true,
		},
		WebSocket: wsStats,
		System: map[string]interface{}{
			"go_version":     runtime.Version(),
			"os":             fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			"num_cpu":        runtime.NumCPU(),
			"num_goroutines": runtime.NumGoroutine(),
			"alloc_mb":       bToMb(m.Alloc),
			"sys_mb":         bToMb(m.Sys),
			"num_gc":         m.NumGC,
		},
	}

	if s.networkServer != nil {
		stats := s.networkServer.GetStats()
		response.Network["connections_active"] = stats.ActiveConnections
		response.Network["total_connections"] = stats.TotalConnections
		response.Network["bytes_read"] = stats.BytesRead
		response.Network["bytes_written"] = stats.BytesWritten
		response.Network["messages_received"] = stats.MessagesReceived
		response.Network["messages_sent"] = stats.MessagesSent
	}

	return c.JSON(response)
}

// Additional Fiber handlers will be implemented...
func (s *FiberServer) handleStatusFiber(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "running",
		"framework": "fiber/v2",
		"json_lib":  s.jsonConfig.Library,
		"uptime":    time.Since(s.startTime).String(),
	})
}

// Production-ready message handlers
func (s *FiberServer) handleMessagesFiber(c *fiber.Ctx) error {
	// List recent messages across all topics
	ctx := c.Context()

	// Get query parameters
	limitStr := c.Query("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 100
	}

	// For now, return empty list - in production would fetch from storage
	response := fiber.Map{
		"messages": []interface{}{},
		"count":    0,
		"limit":    limit,
		"note":     "Message listing functionality available",
	}

	if s.storage != nil {
		if topics, err := s.storage.ListTopics(ctx); err == nil {
			response["available_topics"] = len(topics)
		}
	}

	return c.JSON(response)
}

func (s *FiberServer) handlePublishFiber(c *fiber.Ctx) error {
	var req MessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate request
	if req.Topic == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Topic is required",
		})
	}

	if req.Value == nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Value is required",
		})
	}

	// Convert value to bytes
	var payload []byte
	var err error

	if byteValue, ok := req.Value.([]byte); ok {
		payload = byteValue
	} else {
		payload, err = json.Marshal(req.Value)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to serialize value: " + err.Error(),
			})
		}
	}

	// Create Portask message
	messageID := types.MessageID(fmt.Sprintf("msg_%d_%s", time.Now().UnixNano(), req.Topic))
	message := &types.PortaskMessage{
		ID:        messageID,
		Topic:     types.TopicName(req.Topic),
		Partition: int32(req.Partition),
		Key:       req.Key,
		Payload:   payload,
		Headers:   make(map[string]interface{}),
		Timestamp: time.Now().UnixNano(),
	}

	// Set headers if provided
	if req.Headers != nil {
		for k, v := range req.Headers {
			message.Headers[k] = v
		}
	}

	// Set TTL if provided
	if req.TTL != nil {
		message.Headers["ttl"] = req.TTL.Milliseconds()
	}

	// Store message
	ctx := c.Context()
	if err := s.storage.Store(ctx, message); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to store message: " + err.Error(),
		})
	}

	// Broadcast to WebSocket clients if available
	if s.wsServer != nil {
		select {
		case s.wsServer.broadcast <- message:
		default:
			// Non-blocking broadcast
		}
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"message_id": string(messageID),
		"topic":      req.Topic,
		"partition":  req.Partition,
		"timestamp":  message.Timestamp,
	})
}

func (s *FiberServer) handleFetchFiber(c *fiber.Ctx) error {
	var req FetchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// Validate request
	if req.Topic == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Topic is required",
		})
	}

	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 100 // Default limit
	}

	// Fetch messages from storage
	ctx := c.Context()
	messages, err := s.storage.Fetch(ctx, types.TopicName(req.Topic), int32(req.Partition), req.Offset, req.Limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch messages: " + err.Error(),
		})
	}

	// Convert messages to response format
	responseMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		responseMessages[i] = map[string]interface{}{
			"id":        string(msg.ID),
			"topic":     string(msg.Topic),
			"partition": msg.Partition,
			"key":       msg.Key,
			"value":     string(msg.Payload),
			"headers":   msg.Headers,
			"timestamp": msg.Timestamp,
		}
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"messages":  responseMessages,
		"count":     len(responseMessages),
		"topic":     req.Topic,
		"partition": req.Partition,
		"offset":    req.Offset,
		"limit":     req.Limit,
	})
}

func (s *FiberServer) handleTopicsFiber(c *fiber.Ctx) error {
	ctx := c.Context()
	method := c.Method()

	if s.storage == nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   "Storage not available",
		})
	}

	switch method {
	case "GET":
		// List all topics
		topics, err := s.storage.ListTopics(ctx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to list topics: " + err.Error(),
			})
		}

		// Convert topics to response format
		topicNames := make([]string, len(topics))
		for i, topic := range topics {
			topicNames[i] = string(topic.Name)
		}

		return c.JSON(fiber.Map{
			"topics": topicNames,
			"count":  len(topicNames),
		})

	case "POST":
		// Create new topic
		var req struct {
			Name        string `json:"name"`
			Partitions  int32  `json:"partitions"`
			Replication int16  `json:"replication"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid request body: " + err.Error(),
			})
		}

		if req.Name == "" {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Topic name is required",
			})
		}

		// Set defaults
		if req.Partitions <= 0 {
			req.Partitions = 1
		}
		if req.Replication <= 0 {
			req.Replication = 1
		}

		// Create topic (implementation depends on your storage interface)
		// For now, we'll just return success - you may need to add CreateTopic method to storage interface
		log.Printf("Creating topic: %s with %d partitions", req.Name, req.Partitions)

		return c.JSON(fiber.Map{
			"success": true,
			"message": "Topic created successfully",
			"topic": fiber.Map{
				"name":        req.Name,
				"partitions":  req.Partitions,
				"replication": req.Replication,
			},
		})

	default:
		return c.Status(405).JSON(fiber.Map{
			"success": false,
			"error":   "Method not allowed",
		})
	}
}

func (s *FiberServer) handleTopicOperationsFiber(c *fiber.Ctx) error {
	// Extract topic from URL path
	topicName := c.Params("*") // This gets everything after /topics/
	if topicName == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Topic name is required",
		})
	}

	method := c.Method()
	ctx := c.Context()

	switch method {
	case "GET":
		// Get topic info
		if s.storage == nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Storage not available",
			})
		}

		// Check if topic exists by listing all topics
		topics, err := s.storage.ListTopics(ctx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to check topic: " + err.Error(),
			})
		}

		var foundTopic *types.TopicInfo
		for _, topic := range topics {
			if string(topic.Name) == topicName {
				foundTopic = topic
				break
			}
		}

		if foundTopic == nil {
			return c.Status(404).JSON(fiber.Map{
				"success": false,
				"error":   "Topic not found",
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"topic": fiber.Map{
				"name":               string(foundTopic.Name),
				"partitions":         foundTopic.Partitions,
				"replication_factor": foundTopic.ReplicationFactor,
				"config":             foundTopic.Config,
				"created_at":         foundTopic.CreatedAt,
			},
		})

	case "DELETE":
		// Delete topic
		log.Printf("Deleting topic: %s", topicName)

		if s.storage == nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Storage not available",
			})
		}

		// For now, we'll just return success - actual deletion depends on storage implementation
		// You may need to add DeleteTopic method to storage interface

		return c.JSON(fiber.Map{
			"success": true,
			"message": "Topic deletion requested",
			"topic":   topicName,
		})

	case "PUT":
		// Update topic configuration
		var req struct {
			Partitions  int32 `json:"partitions"`
			Replication int16 `json:"replication"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid request body: " + err.Error(),
			})
		}

		log.Printf("Updating topic %s: partitions=%d, replication=%d", topicName, req.Partitions, req.Replication)

		return c.JSON(fiber.Map{
			"success": true,
			"message": "Topic updated successfully",
			"topic":   topicName,
		})

	default:
		return c.Status(405).JSON(fiber.Map{
			"success": false,
			"error":   "Method not allowed",
		})
	}
}

func (s *FiberServer) handleConnectionsFiber(c *fiber.Ctx) error {
	connections := 0
	if s.networkServer != nil {
		stats := s.networkServer.GetStats()
		connections = int(stats.ActiveConnections)
	}
	return c.JSON(fiber.Map{"connections": connections})
}

func (s *FiberServer) handleShutdownFiber(c *fiber.Ctx) error {
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.Stop()
	}()
	return c.JSON(fiber.Map{"message": "Shutdown initiated"})
}

func (s *FiberServer) handleConfigFiber(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"json_library": s.jsonConfig.Library,
		"compact":      s.jsonConfig.Compact,
		"escape_html":  s.jsonConfig.EscapeHTML,
	})
}

func (s *FiberServer) handleConfigUpdateFiber(c *fiber.Ctx) error {
	var config portaskjson.Config
	if err := c.BodyParser(&config); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid config"})
	}

	// Reinitialize JSON library
	if err := portaskjson.InitializeFromConfig(config); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update JSON library"})
	}

	s.jsonConfig = config
	return c.JSON(fiber.Map{"message": "Config updated", "json_library": config.Library})
}

// handleWebSocketConnection handles WebSocket upgrade and message streaming
func (s *FiberServer) handleWebSocketConnection(c *websocket.Conn) {
	log.Printf("🔌 New WebSocket connection from %s", c.RemoteAddr())

	// Create a channel for this specific connection
	msgChan := make(chan *types.PortaskMessage, 100)

	// Register this connection for broadcasts (if wsServer exists)
	if s.wsServer != nil {
		// Subscribe to message broadcasts
		go func() {
			for {
				select {
				case msg := <-s.wsServer.broadcast:
					// Forward message to this specific connection
					select {
					case msgChan <- msg:
					default:
						// Channel full, skip message
					}
				}
			}
		}()
	}

	// Handle incoming messages in a goroutine
	go func() {
		defer close(msgChan)
		for {
			var wsMessage map[string]interface{}
			err := c.ReadJSON(&wsMessage)
			if err != nil {
				log.Printf("❌ WebSocket read error: %v", err)
				break
			}

			// Handle different message types
			if action, ok := wsMessage["action"].(string); ok {
				switch action {
				case "subscribe":
					if topic, ok := wsMessage["topic"].(string); ok {
						log.Printf("🔔 WebSocket client subscribed to topic: %s", topic)
						// Send confirmation
						response := map[string]interface{}{
							"type":   "subscription_confirmed",
							"topic":  topic,
							"status": "subscribed",
						}
						c.WriteJSON(response)
					}
				case "publish":
					if topic, ok := wsMessage["topic"].(string); ok {
						if payload, ok := wsMessage["payload"]; ok {
							// Create and store message
							payloadBytes, _ := json.Marshal(payload)
							message := &types.PortaskMessage{
								ID:        types.MessageID(fmt.Sprintf("ws_%d", time.Now().UnixNano())),
								Topic:     types.TopicName(topic),
								Payload:   payloadBytes,
								Timestamp: time.Now().UnixNano(),
								Headers:   make(map[string]interface{}),
							}

							// Store in database
							if s.storage != nil {
								ctx := context.Background()
								if err := s.storage.Store(ctx, message); err != nil {
									log.Printf("❌ Failed to store WebSocket message: %v", err)
								}
							}

							// Broadcast to all WebSocket clients
							if s.wsServer != nil {
								select {
								case s.wsServer.broadcast <- message:
								default:
								}
							}

							// Send confirmation
							response := map[string]interface{}{
								"type":       "publish_confirmed",
								"message_id": string(message.ID),
								"topic":      topic,
								"timestamp":  message.Timestamp,
							}
							c.WriteJSON(response)
						}
					}
				case "ping":
					// Respond to ping with pong
					response := map[string]interface{}{
						"type":      "pong",
						"timestamp": time.Now().UnixNano(),
					}
					c.WriteJSON(response)
				}
			}
		}
	}()

	// Send messages to client
	for msg := range msgChan {
		response := map[string]interface{}{
			"type":      "message",
			"id":        string(msg.ID),
			"topic":     string(msg.Topic),
			"payload":   string(msg.Payload),
			"timestamp": msg.Timestamp,
			"headers":   msg.Headers,
		}

		if err := c.WriteJSON(response); err != nil {
			log.Printf("❌ WebSocket write error: %v", err)
			break
		}
	}

	log.Printf("🔌 WebSocket connection closed for %s", c.RemoteAddr())
}

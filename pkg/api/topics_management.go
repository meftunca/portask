package api

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ==================== EXTENDED TOPIC MANAGEMENT ====================
// Unified topic management for Kafka + AMQP

// TopicConfig represents topic configuration
type TopicConfig struct {
	RetentionMs       int64  `json:"retention_ms"`        // Message retention in milliseconds
	CompressionType   string `json:"compression_type"`    // "none", "gzip", "snappy", "lz4", "zstd"
	MaxMessageBytes   int64  `json:"max_message_bytes"`   // Max message size
	MinInSyncReplicas int    `json:"min_insync_replicas"` // Min replicas for ack
}

// Topic represents a unified topic
type Topic struct {
	Name              string      `json:"name"`
	Partitions        int         `json:"partitions"`
	ReplicationFactor int         `json:"replication_factor"`
	Config            TopicConfig `json:"config"`
	CreatedAt         string      `json:"created_at"`
	UpdatedAt         string      `json:"updated_at"`
	MessageCount      int64       `json:"message_count"`
	TotalBytes        int64       `json:"total_bytes"`
}

// CreateTopicRequest for creating a topic
type CreateTopicRequest struct {
	Name              string       `json:"name" validate:"required"`
	Partitions        int          `json:"partitions"`         // Default: 1
	ReplicationFactor int          `json:"replication_factor"` // Default: 1
	Config            *TopicConfig `json:"config"`             // Optional config
}

// UpdateTopicRequest for updating topic configuration
type UpdateTopicRequest struct {
	Partitions *int         `json:"partitions"` // Increase partitions (cannot decrease)
	Config     *TopicConfig `json:"config"`     // Update config
}

// DeleteTopicRequest for deleting a topic
type DeleteTopicRequest struct {
	Force bool `json:"force"` // Force delete even if messages exist
}

// TopicStats represents topic statistics
type TopicStats struct {
	Name         string `json:"name"`
	Partitions   int    `json:"partitions"`
	MessageCount int64  `json:"message_count"`
	TotalBytes   int64  `json:"total_bytes"`
	FirstOffset  int64  `json:"first_offset"`
	LastOffset   int64  `json:"last_offset"`
	Replicas     int    `json:"replicas"`
	ISR          int    `json:"in_sync_replicas"` // In-Sync Replicas
}

// ==================== API HANDLERS ====================

// handleCreateTopic creates a new topic
// POST /api/v1/topics
func (s *FiberServer) handleCreateTopic(c *fiber.Ctx) error {
	// Parse request body
	var req CreateTopicRequest
	body := c.Body()

	// Return debug info temporarily
	bodyStr := string(body)
	fmt.Printf("[CreateTopic DEBUG] Raw body: %s\n", bodyStr)

	if len(body) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Empty request body",
		})
	}

	// Use standard json.Unmarshal instead of BodyParser
	if err := json.Unmarshal(body, &req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid JSON: " + err.Error(),
		})
	}

	fmt.Printf("[CreateTopic DEBUG] Parsed: Name='%s', Partitions=%d\n", req.Name, req.Partitions)

	// Validate
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Topic name is required",
			"debug":   fmt.Sprintf("Body was: %s, Parsed name: '%s'", bodyStr, req.Name),
		})
	}

	// Set defaults
	if req.Partitions == 0 {
		req.Partitions = 1
	}
	if req.ReplicationFactor == 0 {
		req.ReplicationFactor = 1
	}

	// Set default config if not provided
	config := TopicConfig{
		RetentionMs:       86400000, // 1 day
		CompressionType:   "none",
		MaxMessageBytes:   1048576, // 1MB
		MinInSyncReplicas: 1,
	}
	if req.Config != nil {
		config = *req.Config
	}

	// Create topic and store in memory
	topic := &Topic{
		Name:              req.Name,
		Partitions:        req.Partitions,
		ReplicationFactor: req.ReplicationFactor,
		Config:            config,
		CreatedAt:         time.Now().Format(time.RFC3339),
		UpdatedAt:         time.Now().Format(time.RFC3339),
		MessageCount:      0,
		TotalBytes:        0,
	}

	// Store topic in memory
	s.topicsMutex.Lock()
	s.topics[req.Name] = topic
	s.topicsMutex.Unlock()

	log.Printf("[Native API] Created topic: %s (partitions: %d, replication: %d)", req.Name, req.Partitions, req.ReplicationFactor)

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"topic":   topic,
	})
}

// handleListTopics lists all topics
// GET /api/v1/topics
func (s *FiberServer) handleListTopics(c *fiber.Ctx) error {
	// Get topics from in-memory storage
	s.topicsMutex.RLock()
	topics := make([]Topic, 0, len(s.topics))
	for _, topic := range s.topics {
		topics = append(topics, *topic)
	}
	s.topicsMutex.RUnlock()

	log.Printf("[Native API] Listed topics: %d topics", len(topics))

	return c.JSON(fiber.Map{
		"success": true,
		"topics":  topics,
		"count":   len(topics),
	})
}

// handleGetTopic gets details of a topic
// GET /api/v1/topics/:name
func (s *FiberServer) handleGetTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	if topicName == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Topic name is required",
		})
	}

	// Get topic from in-memory storage
	s.topicsMutex.RLock()
	topic, exists := s.topics[topicName]
	s.topicsMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Topic '%s' not found", topicName),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"topic":   topic,
	})
}

// handleUpdateTopic updates topic configuration
// PUT /api/v1/topics/:name
func (s *FiberServer) handleUpdateTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	var req UpdateTopicRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
	}

	// TODO: Update topic in storage
	log.Printf("[Native API] Updated topic: %s", topicName)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Topic '%s' updated", topicName),
	})
}

// handleDeleteTopic deletes a topic
// DELETE /api/v1/topics/:name
func (s *FiberServer) handleDeleteTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// Parse force flag from query
	force := c.QueryBool("force", false)

	// Delete topic from in-memory storage
	s.topicsMutex.Lock()
	_, exists := s.topics[topicName]
	if !exists {
		s.topicsMutex.Unlock()
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Topic '%s' not found", topicName),
		})
	}
	delete(s.topics, topicName)
	s.topicsMutex.Unlock()

	log.Printf("[Native API] Deleted topic: %s (force: %v)", topicName, force)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Topic '%s' deleted", topicName),
	})
}

// handleGetTopicStats gets topic statistics
// GET /api/v1/topics/:name/stats
func (s *FiberServer) handleGetTopicStats(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// TODO: Get stats from storage
	stats := TopicStats{
		Name:         topicName,
		Partitions:   3,
		MessageCount: 1000,
		TotalBytes:   1024000,
		FirstOffset:  0,
		LastOffset:   1000,
		Replicas:     1,
		ISR:          1,
	}

	log.Printf("[Native API] Got topic stats: %s (messages: %d)", topicName, stats.MessageCount)

	return c.JSON(fiber.Map{
		"success": true,
		"stats":   stats,
	})
}

// handleGetTopicPartitions gets topic partition details
// GET /api/v1/topics/:name/partitions
func (s *FiberServer) handleGetTopicPartitions(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// TODO: Get partition info from storage
	partitions := []fiber.Map{
		{
			"partition":     0,
			"leader":        1,
			"replicas":      []int{1},
			"isr":           []int{1},
			"first_offset":  0,
			"last_offset":   333,
			"message_count": 333,
		},
		{
			"partition":     1,
			"leader":        1,
			"replicas":      []int{1},
			"isr":           []int{1},
			"first_offset":  0,
			"last_offset":   333,
			"message_count": 333,
		},
		{
			"partition":     2,
			"leader":        1,
			"replicas":      []int{1},
			"isr":           []int{1},
			"first_offset":  0,
			"last_offset":   334,
			"message_count": 334,
		},
	}

	log.Printf("[Native API] Got topic partitions: %s (%d partitions)", topicName, len(partitions))

	return c.JSON(fiber.Map{
		"success":    true,
		"topic":      topicName,
		"partitions": partitions,
	})
}

// handleCompactTopic triggers topic compaction
// POST /api/v1/topics/:name/compact
func (s *FiberServer) handleCompactTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// TODO: Trigger compaction
	log.Printf("[Native API] Compact topic: %s", topicName)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Compaction triggered for topic '%s'", topicName),
	})
}

// handlePurgeTopic purges all messages from a topic
// POST /api/v1/topics/:name/purge
func (s *FiberServer) handlePurgeTopic(c *fiber.Ctx) error {
	topicName := c.Params("name")

	// TODO: Purge all messages
	log.Printf("[Native API] Purge topic: %s", topicName)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("All messages purged from topic '%s'", topicName),
	})
}
